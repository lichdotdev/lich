package browser

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chromium"
)

// lateTextPage swaps Loading for Saved a moment after it loads, below the
// fold so the wait is seen to read the whole page and not the viewport.
const lateTextPage = `<title>Late</title><div style="height:3000px"></div>
<p id="status">Loading</p>
<script>setTimeout(() => { document.getElementById('status').textContent = 'Saved' }, 300)</script>`

func TestChromeReadWaitsForTextThePageWritesLater(t *testing.T) {
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, lateTextPage)
	}))
	t.Cleanup(srv.Close)
	s := newService(chromeLaunch, func(string, string) (string, error) { return t.TempDir(), nil }, time.Now, idleAfter)
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	// One browser per case: each reads the page's text change from its load.
	for _, opts := range []api.BrowserInfoOptions{
		{ID: "for", WaitFor: "Saved"},
		{ID: "gone", WaitGone: "Loading"},
	} {
		if _, err := s.Open(opts.ID, srv.URL); err != nil {
			t.Fatal(err)
		}
		info, err := s.Read(t.Context(), opts)
		if err != nil {
			t.Fatal(err)
		}
		if info.WaitTimedOut {
			t.Fatalf("%+v timed out, want the page's late text seen", opts)
		}
	}
	s.textWait = 3 * textPoll
	info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "for", WaitFor: "saved"})
	if err != nil {
		t.Fatal(err)
	}
	if !info.WaitTimedOut || info.Title != "Late" {
		t.Fatalf("got timed out %v on %q, want the page answered with the wait timed out", info.WaitTimedOut, info.Title)
	}
}
