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

// consolePage logs one error and one warning as it loads, asks for an image
// that is not there, and throws when its button is clicked.
const consolePage = `<title>Console</title>
<img src="/missing.png">
<button id="throw" style="font-size:30px" onclick="null.f()">throw</button>
<script>console.error("load failed:", 42); console.warn("only a warning")</script>`

func TestChromeNamesTheConsoleErrorsOnceInTheNextPageAnswer(t *testing.T) {
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Chrome asks for a favicon on its own, at a moment of its choosing:
		// its 404 would land in whichever answer comes next.
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, consolePage)
	}))
	t.Cleanup(srv.Close)
	// The profile's folder is made before the service so that its removal,
	// a cleanup, runs after Cleanup closed Chrome: Windows refuses to delete
	// a file Chrome still holds.
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
	loaded := readConsole(t, s, 2)
	if !hasError(loaded, "load failed: 42") || !hasError(loaded, "/missing.png") || hasError(loaded, "warning") {
		t.Fatalf("the page answer named %q, want the error and the missing image, no warning", loaded)
	}
	clicked, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#throw"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(clicked.ConsoleErrors) != 1 || !strings.Contains(clicked.ConsoleErrors[0], "Uncaught TypeError") {
		t.Fatalf("the click answered %q, want the one uncaught exception", clicked.ConsoleErrors)
	}
	again, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.ConsoleErrors) != 0 {
		t.Fatalf("a read after the click repeated %q", again.ConsoleErrors)
	}
}

// readConsole reads the page until its answers have named want errors: the
// image's failure arrives when Chrome gets the 404, not with the document.
func readConsole(t *testing.T, s *Service, want int) []string {
	t.Helper()
	var got []string
	deadline := time.Now().Add(pageWait)
	for len(got) < want && time.Now().Before(deadline) {
		info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, info.ConsoleErrors...)
		time.Sleep(downloadPoll)
	}
	return got
}

func hasError(errs []string, part string) bool {
	for _, e := range errs {
		if strings.Contains(e, part) {
			return true
		}
	}
	return false
}
