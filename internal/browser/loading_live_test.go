package browser

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chromium"
)

// heldPages serves /held, a page whose image waits until release is
// closed, and /nothing, which answers 204.
func heldPages(release chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/held":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<title>held</title><img src="/image">`))
		case "/image":
			select {
			case <-release:
			case <-r.Context().Done():
			}
			w.WriteHeader(http.StatusNotFound)
		case "/nothing":
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// loadingService is a browser on a page of body, beside heldPages.
func loadingService(t *testing.T, body string) (s *Service, base string, release chan struct{}) {
	t.Helper()
	release = make(chan struct{})
	s = settleService(t, body, heldPages(release))
	waitLoading(t, s, false)
	tabs, err := s.Tabs("s")
	if err != nil {
		t.Fatal(err)
	}
	return s, tabs[0].URL, release
}

func TestATabLoadsUntilItsPageAndItsResourcesLanded(t *testing.T) {
	t.Parallel()
	s, base, release := loadingService(t, `<title>page</title>`)
	go s.Navigate("s", base+"held")
	waitLoading(t, s, true)
	close(release)
	waitLoading(t, s, false)
}

func TestATabALinkOpenedLoadsUntilItsPageLanded(t *testing.T) {
	t.Parallel()
	s, _, release := loadingService(t, `<a id="go" target="_blank" href="/held">go</a>`)
	go s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", Selector: "#go"})
	waitLoading(t, s, true)
	close(release)
	waitLoading(t, s, false)
}

func TestABrowserOpenedOnAPageLoadsUntilItLanded(t *testing.T) {
	t.Parallel()
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	release := make(chan struct{})
	srv := httptest.NewServer(heldPages(release))
	t.Cleanup(srv.Close)
	profile := t.TempDir()
	s := newService(chromeLaunch, func(string, string) (string, error) { return profile, nil }, time.Now, idleAfter)
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	go s.Open("s", srv.URL+"/held")
	waitLoading(t, s, true)
	close(release)
	waitLoading(t, s, false)
}

// Each of these ends a navigation without the load a page has: none leaves
// the tab loading.
func TestATabStopsLoadingWhereANavigationEndsWithoutAPage(t *testing.T) {
	t.Parallel()
	s, base, release := loadingService(t, `<title>page</title>`)
	close(release)
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	refused := "http://" + closed.Addr().String() + "/"
	closed.Close()
	for _, to := range []string{base + "#section", base + "nothing", refused} {
		s.Navigate("s", to)
		waitLoading(t, s, false)
	}
}

func TestATabStaysLoadedWhenThePersonStaysOnThePage(t *testing.T) {
	t.Parallel()
	s, base, _ := loadingService(t, `<button id="b">b</button>
<script>addEventListener('beforeunload', e => e.preventDefault())</script>`)
	// A click gives the page the user activation Chrome asks of a
	// beforeunload dialog.
	if _, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", Selector: "#b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Go(t.Context(), api.BrowserOpenOptions{ID: "s", URL: base + "held"}); !errors.Is(err, errDialogOpen) {
		t.Fatalf("navigate: err = %v, want the beforeunload held open", err)
	}
	if _, err := s.AnswerDialog(t.Context(), api.BrowserDialogOptions{ID: "s"}); err != nil {
		t.Fatal(err)
	}
	waitLoading(t, s, false)
}

// waitLoading waits for the front tab of the browser of session s to be
// loading or not, as want says.
func waitLoading(t *testing.T, s *Service, want bool) {
	t.Helper()
	var tabs []api.BrowserTab
	for deadline := time.Now().Add(pageWait); ; time.Sleep(downloadPoll) {
		got, err := s.Tabs("s")
		if err == nil {
			tabs = got
			if frontOf(tabs).Loading == want {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the front tab's loading is not %v: %+v (%v)", want, tabs, err)
		}
	}
}
