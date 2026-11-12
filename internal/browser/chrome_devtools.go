package browser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// errPaused is a page call a tab stopped in the debugger holds.
var errPaused = errors.New("the page is paused in the debugger of the pane's DevTools; the person resumes it there, or closes DevTools")

// devtoolsSeed turns off two things of the front-end's: its screencast of the
// page, which drew the page a second time inside the pane, and the Paused in
// debugger bar it lays over the page, a second resume beside its own.
const devtoolsSeed = `localStorage.setItem('screencast-enabled', 'false');
localStorage.setItem('disable-paused-state-overlay', 'true')`

// DevTools shows the front tab's DevTools in the pane, opening it first when
// the tab has none.
func (p *chromePage) DevTools(ctx context.Context) error {
	front := p.tabs.frontTab()
	if p.tabs.inspectedBy(front) == nil {
		if err := p.openDevTools(ctx, front); err != nil {
			return err
		}
	}
	return p.present(func() { p.tabs.inspect(front) })
}

// openDevTools loads the browser's own DevTools front-end for t, served on
// its debugging port: devtools://devtools/bundled refused the front-end its
// WebSocket (Chromium 157, measured 2026-11-12). It lives in a window of its
// own, which followTabs leaves out: it has no opener among the tabs. Lich
// watches t's debugger before the front-end attaches, so it hears the first
// pause.
func (p *chromePage) openDevTools(ctx context.Context, t *tab) error {
	port, err := debuggingPort(p.dir)
	if err != nil {
		return err
	}
	d, err := p.newDevTools(ctx)
	if err != nil {
		return err
	}
	if !p.tabs.setDevTools(t, d) {
		p.closeTarget(d.id)
		d.cancel()
		return errNoTab
	}
	front := fmt.Sprintf("http://127.0.0.1:%[1]s/devtools/inspector.html?ws=127.0.0.1:%[1]s/devtools/page/%[2]s", port, t.id)
	err = runOn(t.ctx, ctx, watchPauses)
	if err == nil {
		err = runOn(d.ctx, ctx, loadDevTools(front))
	}
	if err != nil {
		p.closeDevTools(t)
	}
	return err
}

// devtoolsLoads is how many times the front-end is loaded before giving up. In
// a browser launched a moment before, its first load sat 4 s on about:blank and
// then aborted in 5 of 15 runs; the load after it came in under a second
// (Helium 157, measured 2026-11-12).
const devtoolsLoads = 3

// loadDevTools navigates to the front-end at url, again when a load aborts.
func loadDevTools(url string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		for range devtoolsLoads {
			var errorText string
			_, _, errorText, _, err = page.Navigate(url).Do(ctx)
			if err != nil || errorText == "" {
				return err
			}
			err = fmt.Errorf("DevTools load error %s", errorText)
			if errorText != abortedText {
				return err
			}
		}
		return err
	})
}

// newDevTools is a blank target in a window of its own, set to load the
// front-end without its screencast.
func (p *chromePage) newDevTools(ctx context.Context) (*devtools, error) {
	c := chromedp.FromContext(p.ctx)
	var id target.ID
	err := p.runFree(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		id, err = target.CreateTarget("about:blank").WithNewWindow(true).Do(cdp.WithExecutor(ctx, c.Browser))
		return err
	}))
	if err != nil {
		return nil, err
	}
	dctx, cancel := chromedp.NewContext(p.ctx, chromedp.WithTargetID(id))
	d := &devtools{ctx: dctx, cancel: cancel, id: id}
	p.listenDevTools(d)
	// The first run attaches, for as long as the context it runs on lives.
	err = chromedp.Run(dctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(devtoolsSeed).Do(ctx)
		return err
	}))
	if err != nil {
		p.closeTarget(id)
		cancel()
		return nil, err
	}
	return d, nil
}

// watchPauses has the page's debugger tell lich when it stops and goes on.
// Breakpoints off: on lich's session alone, a debugger statement then ran
// through (Chromium 157, measured 2026-11-12), so only DevTools pauses the
// page. Enabling it took under 4 ms and sent 6 to 95 scriptParsed events,
// which DevTools is sent too.
var watchPauses = chromedp.ActionFunc(func(ctx context.Context) error {
	if _, err := debugger.Enable().Do(ctx); err != nil {
		return err
	}
	return debugger.SetBreakpointsActive(false).Do(ctx)
})

// debuggingPort is the port Chrome wrote into its profile, for
// --remote-debugging-port=0.
func debuggingPort(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "DevToolsActivePort"))
	if err != nil {
		return "", err
	}
	port, _, _ := strings.Cut(string(raw), "\n")
	if port == "" {
		return "", errors.New("chrome wrote no debugging port")
	}
	return port, nil
}

// listenDevTools hands on d's frames while the pane shows it.
func (p *chromePage) listenDevTools(d *devtools) {
	chromedp.ListenTarget(d.ctx, func(ev any) {
		if e, ok := ev.(*page.EventScreencastFrame); ok && p.tabs.shown() == d {
			go p.deliver(d.ctx, d.id, e)
		}
	})
}

// closeDevTools closes t's DevTools.
func (p *chromePage) closeDevTools(t *tab) {
	if d := p.tabs.inspectedBy(t); d != nil {
		p.closeTarget(d.id)
	}
	p.devtoolsGone(t)
}

// devtoolsGone forgets t's DevTools once its target closed, lets the page go
// on when it held it in the debugger, and brings the page back to the pane
// when the pane showed DevTools.
func (p *chromePage) devtoolsGone(t *tab) {
	d, shown := p.tabs.dropDevTools(t)
	if d == nil {
		return
	}
	d.cancel()
	// Another debugger client keeps a paused page paused: the page goes on
	// once lich's lets go too.
	if err := runOn(t.ctx, nil, debugger.Disable()); err != nil && p.Alive() {
		slog.Warn("browser: stop watching the page's debugger", "err", err)
	}
	p.syncPause()
	p.tabsChanged()
	if !shown {
		return
	}
	err := p.castPastDialog(func() error {
		// Chrome ended DevTools' screencast with its target.
		casting := p.casting
		p.casting = false
		return p.showFront(casting)
	})
	if err != nil && p.Alive() {
		slog.Warn("browser: bring the page back from DevTools", "err", err)
	}
}

// debuggerPaused records that t stopped in its debugger or went on.
func (p *chromePage) debuggerPaused(t *tab, paused bool) {
	if p.tabs.setPaused(t, paused) {
		p.syncPause()
		p.tabsChanged()
	}
}

// syncPause holds the page's calls while any tab is stopped in its debugger:
// a tab shares the renderer of the one that opened it.
func (p *chromePage) syncPause() {
	if p.tabs.anyPaused() {
		p.pause.open()
	} else {
		p.pause.close()
	}
}

// present changes what the pane shows: it stops the frames of what it showed,
// makes the change and starts them on what it shows now.
func (p *chromePage) present(change func()) error {
	err := p.castPastDialog(func() error {
		casting := p.casting
		// Run even when the stop fails, or the change would never be shown.
		stopped := p.stopCast(nil)
		change()
		return errors.Join(stopped, p.showFront(casting))
	})
	p.tabsChanged()
	p.reportInfo()
	return err
}

// runShown is run on what the pane shows: the page, or its DevTools, which no
// dialog or pause of the page holds.
func (p *chromePage) runShown(ctx context.Context, actions ...chromedp.Action) error {
	if d := p.tabs.shown(); d != nil {
		return runOn(d.ctx, ctx, actions...)
	}
	return p.run(ctx, actions...)
}
