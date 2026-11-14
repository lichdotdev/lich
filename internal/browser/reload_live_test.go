package browser

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// A script served with only a Last-Modified is fresh by Chrome's heuristic
// for a tenth of its age, so a plain reload takes it from the cache.
func TestReloadFetchesAScriptTheCacheHolds(t *testing.T) {
	var version atomic.Int32
	version.Store(1)
	s := settleService(t, `<title>page</title><script src="/v.js"></script>`, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set("Last-Modified", time.Now().Add(-24*time.Hour).UTC().Format(http.TimeFormat))
		fmt.Fprintf(w, "document.title = 'v%d'", version.Load())
	})
	waitTitle(t, s, "v1")
	version.Store(2)
	if err := s.Reload("s"); err != nil {
		t.Fatal(err)
	}
	waitTitle(t, s, "v2")
}

func waitTitle(t *testing.T, s *Service, want string) {
	t.Helper()
	deadline := time.Now().Add(pageWait)
	var title string
	for time.Now().Before(deadline) {
		info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
		if err != nil {
			t.Fatal(err)
		}
		if title = info.Title; title == want {
			return
		}
		time.Sleep(downloadPoll)
	}
	t.Fatalf("the page's title is %q, want %q", title, want)
}
