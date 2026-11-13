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

func TestChromeRelaunchDrawsThePaneAtTheWindowsNewScale(t *testing.T) {
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	testDownloads(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "kept"})
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<title>%s</title><p style="font-size:40px">Sharp text</p>`, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	s := newService(chromeLaunch, func(string, string) (string, error) { return t.TempDir(), nil }, time.Now, idleAfter)
	s.rescaleWait = 200 * time.Millisecond
	frames := make(chan string, 64)
	s.emit = func(name string, data any) {
		if name != api.BrowserFrameEvents.Of("s").Name {
			return
		}
		// The PNG a still page gets is no frame of the screencast's size.
		if f := data.(api.BrowserFrame); !strings.HasPrefix(f.Data, "iVBOR") {
			select {
			case frames <- f.Data:
			default:
			}
		}
	}
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1}, 1)
	h, err := s.Open("s", srv.URL+"/one")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TabAct(api.BrowserTabOptions{ID: "s", Action: api.TabNew, URL: srv.URL + "/two"}); err != nil {
		t.Fatal(err)
	}
	// A pane 600x450 device pixels wide, as the window sends it: CSS pixels
	// and its scale, the window's times the page zoom.
	if _, err := s.Attach("s", Viewport{W: 600, H: 450, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	waitFrameSize(t, frames, 600, 450)
	before, err := s.Tabs(h.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Moved to a 1.5x monitor: the browser launched at 1 draws the pane's
	// 400x300 CSS pixels at 1, which the window would enlarge, until it
	// relaunches at 1.5.
	if err := s.Resize("s", Viewport{W: 400, H: 300, Scale: 1.5}); err != nil {
		t.Fatal(err)
	}
	waitFrameSize(t, frames, 400, 300)
	s.SetDefaultView(Viewport{W: 800, H: 533, Scale: 1.5}, 1.5)
	waitFrameSize(t, frames, 600, 450)

	tabs, err := s.Tabs(h.ID)
	if err != nil {
		t.Fatalf("the agent's handle after the relaunch: %v", err)
	}
	if len(tabs) != 2 || !tabs[1].Front || tabs[0].URL != srv.URL+"/one" || tabs[1].URL != srv.URL+"/two" {
		t.Fatalf("tabs after the relaunch = %+v, want /one and /two in front", tabs)
	}
	// The window keeps frames by tab id and frees those of tabs the state
	// stops listing: the relaunched tabs are new targets, and say so.
	if st := s.Running(); len(st) != 1 || len(st[0].Tabs) != 2 || st[0].Tabs[0].ID == before[0].ID || st[0].Tabs[1].ID == before[1].ID {
		t.Fatalf("state after the relaunch = %+v, want two tabs with new ids past %+v", st, before)
	}
	out, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: h.ID, Script: "document.cookie"})
	if err != nil || out != "sid=kept" {
		t.Fatalf("cookie after the relaunch = %q, %v", out, err)
	}
}

func TestChromeRelaunchOpensDevToolsAgainInFront(t *testing.T) {
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	testDownloads(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<title>Inspected</title><p>page</p>`)
	}))
	t.Cleanup(srv.Close)
	s := newService(chromeLaunch, func(string, string) (string, error) { return t.TempDir(), nil }, time.Now, idleAfter)
	s.rescaleWait = 200 * time.Millisecond
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 1}, 1)
	if _, err := s.Open("s", srv.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach("s", Viewport{W: 600, H: 450, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.DevTools("s"); err != nil {
		t.Fatal(err)
	}
	before := s.Running()[0].Tabs
	s.SetDefaultView(Viewport{W: 800, H: 533, Scale: 1.5}, 1.5)
	deadline := time.Now().Add(15 * time.Second)
	for {
		st := s.Running()
		if len(st) == 1 && len(st[0].Tabs) == 2 && st[0].Tabs[1].DevTools && st[0].Tabs[1].Front && st[0].Tabs[0].ID != before[0].ID {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("pane tabs %+v after the relaunch, want the page's DevTools back in front (before %+v)", st, before)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
