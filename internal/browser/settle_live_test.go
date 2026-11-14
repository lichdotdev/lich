package browser

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chromium"
)

// settleService is a live service with session "s" open on the page body,
// its other paths answered by rest.
func settleService(t *testing.T, body string, rest http.HandlerFunc) *Service {
	t.Helper()
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, body)
		default:
			rest(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	profile := t.TempDir()
	s := newService(chromeLaunch, func(string, string) (string, error) { return profile, nil }, time.Now, idleAfter)
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	if _, err := s.Open("s", srv.URL+"/"); err != nil {
		t.Fatal(err)
	}
	return s
}

func clickGo(t *testing.T, s *Service) (api.BrowserPageInfo, time.Duration) {
	t.Helper()
	start := time.Now()
	info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#go"}})
	if err != nil {
		t.Fatal(err)
	}
	return info, time.Since(start)
}

func TestChromeClickAnswersOnceTheFetchItStartedHasRendered(t *testing.T) {
	s := settleService(t, `<button id="go" style="font-size:30px"
onclick="fetch('/slow').then(r => r.text()).then(t => { out.textContent = 'loaded ' + t })">go</button>
<div id="out">nothing</div>`, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		fmt.Fprint(w, "done")
	})
	info, _ := clickGo(t, s)
	if !strings.Contains(info.Text, "loaded done") {
		t.Fatalf("the click answered %q, want the page the fetch it started rendered", info.Text)
	}
}

func TestChromeClickIsNotHeldByAFetchThatBeganBeforeIt(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	s := settleService(t, `<script>fetch('/hang')</script>
<button id="go" style="font-size:30px" onclick="out.textContent = 'clicked'">go</button>
<div id="out">nothing</div>`, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	info, took := clickGo(t, s)
	if !strings.Contains(info.Text, "clicked") || took >= quietMax {
		t.Fatalf("the click answered %q after %v, want it before quietMax (%v)", info.Text, took, quietMax)
	}
}

func TestChromeClickOnAPageThatNeverStopsChangingAnswersByQuietMax(t *testing.T) {
	s := settleService(t, `<button id="go" style="font-size:30px"
onclick="setInterval(() => { out.textContent = Date.now() }, 20)">go</button>
<div id="out">nothing</div>`, http.NotFound)
	_, took := clickGo(t, s)
	if took < quietMax || took > quietMax+domSettle {
		t.Fatalf("the click answered after %v, want quietMax (%v)", took, quietMax)
	}
}
