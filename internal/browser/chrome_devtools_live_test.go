package browser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chromium"
)

// devtoolsService is a service on a real Chrome showing, in a pane, a page
// whose /pause runs a debugger statement every 200 ms and counts in its title
// the times it ran through.
func devtoolsService(t *testing.T, path string) (*Service, *emitted, string) {
	t.Helper()
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<title>ran 0</title><p>Probe</p>`)
		if r.URL.Path == "/pause" {
			fmt.Fprint(w, `<script>let n = 0; setInterval(() => { debugger; document.title = "ran " + (++n) }, 200)</script>`)
		}
	}))
	t.Cleanup(srv.Close)
	// Made before the cleanup is registered, so the browser closes first.
	dir := t.TempDir()
	s := newService(chromeLaunch, func(string, string) (string, error) { return dir, nil }, time.Now, 5*time.Minute)
	t.Cleanup(func() { _ = s.Cleanup() })
	out := &emitted{}
	s.emit = out.emit
	if _, err := s.Open("s", srv.URL+path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach("s", Viewport{W: 900, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	return s, out, srv.URL
}

// waitPane waits for the window to be told tabs ok accepts.
func waitPane(t *testing.T, out *emitted, ok func([]api.BrowserTab) bool) []api.BrowserTab {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if states := out.states(); len(states) > 0 && ok(states[len(states)-1].Tabs) {
			return states[len(states)-1].Tabs
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the window never got the tabs expected; last %+v", out.states()[len(out.states())-1].Tabs)
	return nil
}

func devtoolsTab(tabs []api.BrowserTab) (api.BrowserTab, bool) {
	i := slices.IndexFunc(tabs, func(t api.BrowserTab) bool { return t.DevTools })
	if i < 0 {
		return api.BrowserTab{}, false
	}
	return tabs[i], true
}

// framesOf counts the frames sent for tab since the from-th, and the PNGs
// among them.
func framesOf(out *emitted, from int, tab string) (frames, stills int) {
	for _, f := range out.named(api.BrowserFrameEvents.Of("s").Name)[from:] {
		if f := f.(api.BrowserFrame); f.Tab == tab {
			frames++
			if strings.HasPrefix(f.Data, pngPrefix) {
				stills++
			}
		}
	}
	return frames, stills
}

func TestChromeDevToolsDrawsInThePaneWhileTheAgentDrivesThePage(t *testing.T) {
	s, out, url := devtoolsService(t, "/")
	frameEvents := api.BrowserFrameEvents.Of("s").Name
	if err := s.DevTools("s"); err != nil {
		t.Fatal(err)
	}
	pane := waitPane(t, out, func(tabs []api.BrowserTab) bool { dt, ok := devtoolsTab(tabs); return ok && dt.Front })
	dt, _ := devtoolsTab(pane)
	if len(pane) != 2 || pane[0].DevTools || dt.URL != "" || dt.Title != "" {
		t.Fatalf("window's tabs = %+v, want the page then its DevTools, which carries no URL", pane)
	}
	// DevTools builds its UI in 2 to 5 s; once still, its pane gets a PNG of it.
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, stills := framesOf(out, 0, dt.ID); stills > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no PNG of DevTools once still")
		}
		time.Sleep(100 * time.Millisecond)
	}
	agent, err := s.Tabs("s")
	if err != nil {
		t.Fatal(err)
	}
	if len(agent) != 1 || agent[0].DevTools || !agent[0].Front || agent[0].ID != pane[0].ID {
		t.Fatalf("agent's tabs = %+v, want the page alone, in front", agent)
	}
	info, err := s.Info("s", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(info.URL, url) || !strings.Contains(info.Text, "Probe") {
		t.Fatalf("the agent read %q at %s, want the page", info.Text, info.URL)
	}
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: api.TabSelect, Tab: pane[0].ID}); err != nil {
		t.Fatal(err)
	}
	from := len(out.named(frameEvents))
	time.Sleep(time.Second)
	if page, _ := framesOf(out, from, pane[0].ID); page == 0 {
		t.Fatal("no frame of the page once its tab was selected again")
	}
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: api.TabClose, Tab: dt.ID}); err != nil {
		t.Fatal(err)
	}
	waitPane(t, out, func(tabs []api.BrowserTab) bool { return len(tabs) == 1 && tabs[0].Front })
}

func TestChromeAPauseInDevToolsHoldsTheAgentUntilDevToolsCloses(t *testing.T) {
	s, out, _ := devtoolsService(t, "/pause")
	if err := s.DevTools("s"); err != nil {
		t.Fatal(err)
	}
	pane := waitPane(t, out, func(tabs []api.BrowserTab) bool { dt, ok := devtoolsTab(tabs); return ok && dt.Paused })
	start := time.Now()
	if _, err := s.Info("s", ""); !errors.Is(err, errPaused) {
		t.Fatalf("Info on a paused page = %v, want the pause named", err)
	}
	if err := s.Click("s", Target{Selector: "p"}); !errors.Is(err, errPaused) {
		t.Fatalf("Click on a paused page = %v, want the pause named", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("the agent waited %v on a paused page", took)
	}
	if _, err := s.MouseInput(context.Background(), api.BrowserMouseOptions{ID: "s", Type: "move", X: 10, Y: 10}); err != nil {
		t.Fatalf("pane input = %v, want none", err)
	}
	dt, _ := devtoolsTab(pane)
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: api.TabClose, Tab: dt.ID}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		info, err := s.Info("s", "")
		if err == nil && info.Title != "ran 0" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the page never went on once DevTools closed: %q, %v", info.Title, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestChromeClosingAPageClosesItsDevTools(t *testing.T) {
	s, out, url := devtoolsService(t, "/")
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: api.TabNew}); err != nil {
		t.Fatal(err)
	}
	if err := s.Nav("s", "go", url); err != nil {
		t.Fatal(err)
	}
	if err := s.DevTools("s"); err != nil {
		t.Fatal(err)
	}
	pane := waitPane(t, out, func(tabs []api.BrowserTab) bool { return len(tabs) == 3 })
	dt, _ := devtoolsTab(pane)
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: api.TabClose, Tab: pane[1].ID}); err != nil {
		t.Fatal(err)
	}
	waitPane(t, out, func(tabs []api.BrowserTab) bool { return len(tabs) == 1 && tabs[0].Front })
	p := s.current("s").page.(*chromePage)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if !targetOpen(t, p, target.ID(dt.ID)) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the closed page's DevTools target is still open")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestChromeAWindowDevToolsOpensComesInBehind(t *testing.T) {
	s, out, url := devtoolsService(t, "/")
	if err := s.DevTools("s"); err != nil {
		t.Fatal(err)
	}
	waitPane(t, out, func(tabs []api.BrowserTab) bool { dt, ok := devtoolsTab(tabs); return ok && dt.Front })
	d := s.current("s").page.(*chromePage).tabs.shown()
	if err := chromedp.Run(d.ctx, chromedp.Evaluate(fmt.Sprintf(`void window.open(%q, "_blank")`, url+"/learn"), nil)); err != nil {
		t.Fatal(err)
	}
	pane := waitPane(t, out, func(tabs []api.BrowserTab) bool { return len(tabs) == 3 })
	if !pane[1].DevTools || !pane[1].Front || !strings.HasSuffix(pane[2].URL, "/learn") || pane[2].Front {
		t.Fatalf("window's tabs = %+v, want the window as a tab behind, past the DevTools still shown", pane)
	}
	agent, err := s.Tabs("s")
	if err != nil {
		t.Fatal(err)
	}
	if len(agent) != 2 || !agent[0].Front || agent[1].Front {
		t.Fatalf("agent's tabs = %+v, want the page still in front, the new tab behind", agent)
	}
}

func targetOpen(t *testing.T, p *chromePage, id target.ID) bool {
	t.Helper()
	c := chromedp.FromContext(p.ctx)
	var infos []*target.Info
	err := chromedp.Run(p.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		infos, err = target.GetTargets().Do(cdp.WithExecutor(ctx, c.Browser))
		return err
	}))
	if err != nil {
		t.Fatal(err)
	}
	return slices.ContainsFunc(infos, func(i *target.Info) bool { return i.TargetID == id })
}
