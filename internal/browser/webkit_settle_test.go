package browser

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

func at(ms int) browserTime {
	return browserTime{wall: time.Now(), ticks: time.Duration(ms) * time.Millisecond}
}

func TestInflightHoldsOnlyWhatBeganSince(t *testing.T) {
	start := func(ms int) browserwire.RequestEvent {
		return browserwire.RequestEvent{Started: true, At: int64(time.Duration(ms) * time.Millisecond)}
	}
	finish := browserwire.RequestEvent{Finished: true}
	for _, tc := range []struct {
		name   string
		events []browserwire.RequestEvent
		want   bool
	}{
		{"none", nil, false},
		{"a fetch the action began", []browserwire.RequestEvent{start(20)}, true},
		{"a long poll from before", []browserwire.RequestEvent{start(5)}, false},
		{"the action's fetch landed", []browserwire.RequestEvent{start(20), finish}, false},
		{"the poll stays, the fetch landed", []browserwire.RequestEvent{start(5), start(20), finish}, false},
		{"two fetches, one landed", []browserwire.RequestEvent{start(20), start(30), finish}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var f inflight
			for _, ev := range tc.events {
				f.note(ev, time.Now())
			}
			if got := f.since(at(10)); got != tc.want {
				t.Fatalf("in flight since 10ms = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadTrackAwait(t *testing.T) {
	for _, tc := range []struct {
		name       string
		steps      []string
		wantFailed bool
	}{
		{"no load starts", nil, false},
		{"commits", []string{browserwire.LoadStarted, browserwire.LoadCommitted}, false},
		{"fails", []string{browserwire.LoadStarted, browserwire.LoadFailed}, true},
		{"fails after it committed", []string{browserwire.LoadStarted, browserwire.LoadCommitted, browserwire.LoadFailed}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := loadTrack{wake: make(chan struct{})}
			from := l.mark()
			go func() {
				for _, step := range tc.steps {
					time.Sleep(time.Millisecond)
					l.note(step, at(1))
				}
			}()
			failed, err := l.await(t.Context(), from, 50*time.Millisecond)
			if err != nil || failed != tc.wantFailed {
				t.Fatalf("await = %v, %v: want failed %v", failed, err, tc.wantFailed)
			}
		})
	}
}

func TestLoadTrackAwaitSinceWaitsForTheActionsLoad(t *testing.T) {
	l := loadTrack{wake: make(chan struct{})}
	l.note(browserwire.LoadStarted, at(5))
	if err := l.awaitSince(t.Context(), at(10), time.Second); err != nil {
		t.Fatal("a load from before the action held it:", err)
	}
	l.note(browserwire.LoadStarted, at(20))
	go func() {
		time.Sleep(5 * time.Millisecond)
		l.note(browserwire.LoadCommitted, at(25))
	}()
	begun := time.Now()
	if err := l.awaitSince(t.Context(), at(10), time.Second); err != nil || time.Since(begun) < 5*time.Millisecond {
		t.Fatalf("awaitSince = %v after %v: want it to wait for the commit", err, time.Since(begun))
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	l.note(browserwire.LoadStarted, at(30))
	if err := l.awaitSince(ctx, at(10), time.Second); err == nil {
		t.Fatal("a cancelled wait did not say so")
	}
	if err := l.awaitSince(t.Context(), at(10), time.Millisecond); err != nil {
		t.Fatal("a load past the wait is no error:", err)
	}
}

func TestWebkitServiceCallsReachTheWire(t *testing.T) {
	f := newFakeLichBrowser(t)
	f.eval = func(script string) (string, string, bool) {
		switch {
		case strings.HasPrefix(script, "(document.body"):
			return "true", "", true
		case strings.Contains(script, "elementFromPoint"):
			return `{"selector":"#a"}`, "", true
		case strings.HasPrefix(script, annotateJS), script == unannotateJS:
			return "", "", true
		}
		return "", "", false
	}
	s, _ := wkService(t, f)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		call   func() error
		method string
	}{
		{"reload", func() error { return s.Reload("s") }, browserwire.Reload},
		{"back", func() error { return s.Back("s") }, browserwire.Back},
		{"forward", func() error { return s.Forward("s") }, browserwire.Forward},
		{"wait for text", func() error {
			_, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s", WaitFor: "hello"})
			return err
		}, browserwire.Eval},
		{"element at", func() error {
			_, err := s.ElementAtPoint(t.Context(), api.BrowserPointOptions{ID: "s", X: 1, Y: 2})
			return err
		}, browserwire.Eval},
		{"locate", func() error {
			_, err := s.LocateElements(t.Context(), api.BrowserLocateOptions{ID: "s", Selectors: []string{"#a"}})
			return err
		}, browserwire.Eval},
		{"annotated screenshot", func() error {
			_, err := s.AnnotatedScreenshot("s", filepath.Join(t.TempDir(), "a.png"))
			return err
		}, browserwire.Snapshot},
		{"detach", func() error { return s.Detach("s") }, browserwire.Hello},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := len(f.called(tc.method))
			if err := tc.call(); err != nil {
				t.Fatal(err)
			}
			if tc.method != browserwire.Hello && len(f.called(tc.method)) == before {
				t.Fatalf("no %s reached lich-browser", tc.method)
			}
		})
	}
}
