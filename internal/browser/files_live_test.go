package browser

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chromium"
)

// filesPage titles the page with what each input holds once it changes.
const filesPage = `<title>Files</title><style>input{display:block;font-size:30px}</style>
<input type="file" id="one" onchange="document.title = 'one ' + [...this.files].map(f => f.name)">
<input type="file" id="many" multiple onchange="document.title = 'many ' + [...this.files].map(f => f.name)">`

func TestChromeAFileInputFillsWithWhatThePersonPicks(t *testing.T) {
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	testDownloads(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, filesPage)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png")
	for _, f := range []string{a, b} {
		if err := os.WriteFile(f, []byte("png"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := newService(chromeLaunch, func(string, string) (string, error) { return t.TempDir(), nil }, time.Now, idleAfter)
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	s.SetPicker(func(string) (string, error) { return a, nil }, func(string) ([]string, error) { return []string{a, b}, nil })
	if _, err := s.Open("s", srv.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach("s", Viewport{W: 640, H: 400, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ id, title string }{{"one", "one a.png"}, {"many", "many a.png,b.png"}} {
		if _, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#" + c.id}}); err != nil {
			t.Fatal(err)
		}
		var title string
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			info, err := s.Info("s")
			if err != nil {
				t.Fatal(err)
			}
			if title = info.Title; title == c.title {
				break
			}
		}
		if title != c.title {
			t.Fatalf("#%s: title %q, want %q", c.id, title, c.title)
		}
	}
}
