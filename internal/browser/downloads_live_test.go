package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chromium"
)

// downloadPage holds one way each a page starts a download. /file/<name>
// answers as an attachment named <name>.
const downloadPage = `<title>Downloads</title><style>button,a{display:block;font-size:30px}</style>
<a id="attr" href="/plain/attr.csv" download="attr.csv">attr</a>
<a id="attachment" href="/file/attachment.csv">attachment</a>
<a id="blank" href="/file/blank.csv" target="_blank">blank</a>
<button id="location" onclick="location.href='/file/location.csv'">location</button>
<button id="open" onclick="window.open('/file/open.csv')">open</button>
<button id="blob" onclick="const a = document.createElement('a');
 a.href = window.URL.createObjectURL(new Blob(['blob.csv,1'], {type: 'text/csv'})); a.download = 'blob.csv'; a.click()">blob</button>`

func TestChromeDownloadsLandInDownloadsAndThePageStays(t *testing.T) {
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	dir := testDownloads(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if name, ok := strings.CutPrefix(r.URL.Path, "/file/"); ok {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
			w.Header().Set("Content-Type", "text/csv")
			fmt.Fprint(w, name+",1")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/plain/") {
			w.Header().Set("Content-Type", "text/csv")
			fmt.Fprint(w, "attr.csv,1")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, downloadPage)
	}))
	t.Cleanup(srv.Close)
	s := newService(chromeLaunch, func(string, string) (string, error) { return t.TempDir(), nil }, time.Now, idleAfter)
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	if _, err := s.Open("s", srv.URL+"/"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"attr", "attachment", "blank", "location", "open", "blob"} {
		t.Run(id, func(t *testing.T) {
			page := srv.URL + "/" + id
			if _, err := s.Go(t.Context(), api.BrowserOpenOptions{ID: "s", URL: page}); err != nil {
				t.Fatal(err)
			}
			info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#" + id}})
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(dir, id+".csv")
			got := info.Downloads
			for deadline := time.Now().Add(10 * time.Second); (len(got) == 0 || got[0].State == api.DownloadInProgress) && time.Now().Before(deadline); {
				// A new tab hands its URL to the page after the click returns,
				// and a read names a download still running, then again once
				// it ends.
				time.Sleep(100 * time.Millisecond)
				read, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
				if err != nil {
					t.Fatal(err)
				}
				if len(read.Downloads) > 0 {
					got = read.Downloads
				}
			}
			if len(got) != 1 || got[0].Path != want || got[0].State != api.DownloadCompleted {
				t.Fatalf("downloads %+v, want %s completed", got, want)
			}
			if b, err := os.ReadFile(want); err != nil || string(b) != id+".csv,1" {
				t.Fatalf("%s holds %q (%v)", want, b, err)
			}
			read, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
			if err != nil {
				t.Fatal(err)
			}
			if read.URL != page {
				t.Fatalf("the page moved to %s", read.URL)
			}
		})
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 6 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("Downloads holds %v, want the six files and nothing else", names)
	}
}

func TestChromeOpenAndNavigateToAFileDownloadIt(t *testing.T) {
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	dir := testDownloads(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
		fmt.Fprint(w, name)
	}))
	t.Cleanup(srv.Close)
	s := newService(chromeLaunch, func(string, string) (string, error) { return t.TempDir(), nil }, time.Now, idleAfter)
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	// The first open launches the browser on the file, the second loads one
	// in it, and the navigate a third.
	calls := []func(context.Context, api.BrowserOpenOptions) (api.BrowserHandle, error){s.OpenURL, s.OpenURL, s.Go}
	for i, call := range calls {
		name := fmt.Sprintf("direct%d.csv", i)
		h, err := call(t.Context(), api.BrowserOpenOptions{ID: "s", URL: srv.URL + "/" + name})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := filepath.Join(dir, name)
		if len(h.Downloads) != 1 || h.Downloads[0].Path != want || h.Downloads[0].State != api.DownloadCompleted {
			t.Fatalf("%s: downloads %+v, want %s completed", name, h.Downloads, want)
		}
		if b, err := os.ReadFile(want); err != nil || string(b) != name {
			t.Fatalf("%s holds %q (%v)", want, b, err)
		}
		if h.URL != "about:blank" {
			t.Fatalf("%s: the page moved to %s", name, h.URL)
		}
	}
}

// testDownloads is the folder Chrome saves into for this test, never the
// machine user's Downloads.
func testDownloads(t *testing.T) string {
	dir := filepath.Join(t.TempDir(), "Downloads")
	downloadFolder = func() (string, error) { return dir, nil }
	t.Cleanup(func() { downloadFolder = downloadDir })
	return dir
}
