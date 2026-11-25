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

func TestWebkitSettleHoldsOnlyTheFetchesTheActionBegan(t *testing.T) {
	begin := func(id, kind string, ms int) browserwire.RequestEvent {
		return browserwire.RequestEvent{ID: id, Type: kind, Started: true, At: int64(time.Duration(ms) * time.Millisecond)}
	}
	end := func(id string) browserwire.RequestEvent { return browserwire.RequestEvent{ID: id, Finished: true} }
	for _, tc := range []struct {
		name   string
		events []browserwire.RequestEvent
		want   bool
	}{
		{"none", nil, false},
		{"a fetch the action began", []browserwire.RequestEvent{begin("a", browserwire.RequestFetch, 20)}, true},
		{"an XHR the action began", []browserwire.RequestEvent{begin("a", browserwire.RequestXHR, 20)}, true},
		{"an image", []browserwire.RequestEvent{begin("a", browserwire.RequestImage, 20)}, false},
		{"a long poll from before", []browserwire.RequestEvent{begin("a", browserwire.RequestFetch, 5)}, false},
		{"the action's fetch landed", []browserwire.RequestEvent{begin("a", browserwire.RequestFetch, 20), end("a")}, false},
		{"the poll landed, the fetch did not", []browserwire.RequestEvent{begin("p", browserwire.RequestFetch, 5), begin("a", browserwire.RequestFetch, 20), end("p")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p webkitPage
			for _, ev := range tc.events {
				p.noteRequest(ev, time.Now())
			}
			if got := p.requests.since(at(10)); got != tc.want {
				t.Fatalf("in flight since 10ms = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadTrackAwait(t *testing.T) {
	started := browserwire.LoadEvent{Step: browserwire.LoadStarted}
	committed := browserwire.LoadEvent{Step: browserwire.LoadCommitted}
	failed := browserwire.LoadEvent{Step: browserwire.LoadFailed, Error: "Could not resolve host"}
	download := browserwire.LoadEvent{Step: browserwire.LoadFailed, Download: true}
	for _, tc := range []struct {
		name    string
		steps   []browserwire.LoadEvent
		wantErr string
	}{
		{"no load starts", nil, ""},
		{"commits", []browserwire.LoadEvent{started, committed}, ""},
		{"fails", []browserwire.LoadEvent{started, failed}, "page load error Could not resolve host"},
		{"becomes a download", []browserwire.LoadEvent{started, download}, errNavDownload.Error()},
		{"fails after it committed", []browserwire.LoadEvent{started, committed, failed}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := loadTrack{wake: make(chan struct{})}
			from := l.mark()
			go func() {
				for _, step := range tc.steps {
					time.Sleep(time.Millisecond)
					l.note(step, time.Now())
				}
			}()
			failed, err := l.await(t.Context(), from, 50*time.Millisecond)
			if err != nil || (failed == nil) != (tc.wantErr == "") || failed != nil && failed.Error() != tc.wantErr {
				t.Fatalf("await = %v, %v: want %q", failed, err, tc.wantErr)
			}
		})
	}
}

func TestLoadTrackAwaitSinceWaitsForTheActionsLoad(t *testing.T) {
	l := loadTrack{wake: make(chan struct{})}
	l.note(loadAt(browserwire.LoadStarted, 5), time.Now())
	if err := l.awaitSince(t.Context(), at(10), time.Second); err != nil {
		t.Fatal("a load from before the action held it:", err)
	}
	l.note(loadAt(browserwire.LoadStarted, 20), time.Now())
	go func() {
		time.Sleep(5 * time.Millisecond)
		l.note(loadAt(browserwire.LoadCommitted, 25), time.Now())
	}()
	begun := time.Now()
	if err := l.awaitSince(t.Context(), at(10), time.Second); err != nil || time.Since(begun) < 5*time.Millisecond {
		t.Fatalf("awaitSince = %v after %v: want it to wait for the commit", err, time.Since(begun))
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	l.note(loadAt(browserwire.LoadStarted, 30), time.Now())
	if err := l.awaitSince(ctx, at(10), time.Second); err == nil {
		t.Fatal("a cancelled wait did not say so")
	}
	if err := l.awaitSince(t.Context(), at(10), time.Millisecond); err != nil {
		t.Fatal("a load past the wait is no error:", err)
	}
}

func loadAt(step string, ms int) browserwire.LoadEvent {
	return browserwire.LoadEvent{Step: step, At: int64(time.Duration(ms) * time.Millisecond)}
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
