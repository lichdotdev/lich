package browser

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// tab is one of the page's tabs. Only the front one is drawn and driven: a
// headless Chrome draws no frames for a tab behind another and takes no input
// there.
type tab struct {
	ctx    context.Context
	cancel context.CancelFunc
	id     target.ID
}

// tabStack is the first tab with the tabs opened over it, the front one
// last. A tab the front one opens goes in front of it, and the one under it
// comes back when it closes, still logged in and where it was: loading the
// new tab's URL in the first one instead lost a login a script held there.
type tabStack struct {
	mu   sync.Mutex
	open []*tab
	// view is the size the page was last laid out at, which a tab coming to
	// the front is laid out at too.
	view *Viewport
	// dialog is the tab holding a JavaScript dialog open, nil for none. A tab
	// behind the front one can: with its opener, a new tab shares the
	// renderer of the one under it, and a dialog there holds the front tab's
	// script too (Chromium 155, measured 2026-10-29).
	dialog *tab
}

func (s *tabStack) front() *tab {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.open[len(s.open)-1]
}

func (s *tabStack) push(t *tab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.open = append(s.open, t)
}

// remove takes the tab id out of the stack, and says whether it was the
// front one. The first tab is never removed: it is the browser's own.
func (s *tabStack) remove(id target.ID) (t *tab, front bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.open) - 1; i > 0; i-- {
		if s.open[i].id == id {
			t = s.open[i]
			s.open = append(s.open[:i], s.open[i+1:]...)
			return t, i == len(s.open)
		}
	}
	return nil, false
}

func (s *tabStack) setView(v Viewport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view = &v
}

func (s *tabStack) lastView() *Viewport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view
}

func (s *tabStack) setDialog(t *tab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialog = t
}

// dialogClosed says whether t was the tab holding the dialog, which it no
// longer does.
func (s *tabStack) dialogClosed(t *tab) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dialog != t {
		return false
	}
	s.dialog = nil
	return true
}

// dialogHolder is the tab holding the dialog, else the front one.
func (s *tabStack) dialogHolder() *tab {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dialog != nil {
		return s.dialog
	}
	return s.open[len(s.open)-1]
}

// followNewTabs brings in front what the front tab opens in a new tab or
// window, and the tab under it back once it closes.
func (p *chromePage) followNewTabs() {
	c := chromedp.FromContext(p.ctx)
	discover := chromedp.ActionFunc(func(ctx context.Context) error {
		return target.SetDiscoverTargets(true).Do(cdp.WithExecutor(ctx, c.Browser))
	})
	if err := p.runFree(nil, discover); err != nil {
		slog.Warn("browser: watch for new tabs", "err", err)
		return
	}
	chromedp.ListenBrowser(p.ctx, func(ev any) {
		var info *target.Info
		switch e := ev.(type) {
		case *target.EventTargetCreated:
			info = e.TargetInfo
		case *target.EventTargetInfoChanged:
			info = e.TargetInfo
		case *target.EventTargetDestroyed:
			go p.dropTab(e.TargetID)
			return
		}
		// A new tab is created at about:blank and gets its URL after.
		if info == nil || info.OpenerID != p.tabs.front().id || info.Type != "page" || info.URL == "" || info.URL == "about:blank" {
			return
		}
		// Both the created and the info-changed event can name the tab.
		if _, taken := p.listen.taken.LoadOrStore(info.TargetID, true); !taken {
			go p.bringIn(info.TargetID, info.URL)
		}
	})
}

// bringIn attaches to the tab id the front tab opened and puts it in front.
func (p *chromePage) bringIn(id target.ID, url string) {
	if _, err := allowURL(url); err != nil {
		slog.Warn("browser: a new tab's URL is not one the page may load", "url", url, "err", err)
		p.closeTab(id)
		return
	}
	ctx, cancel := chromedp.NewContext(p.ctx, chromedp.WithTargetID(id))
	t := &tab{ctx: ctx, cancel: cancel, id: id}
	p.listenTab(t)
	if err := chromedp.Run(ctx); err != nil {
		cancel()
		if p.Alive() {
			slog.Warn("browser: attach to a tab the page opened", "url", url, "err", err)
		}
		return
	}
	err := p.castPastDialog(func() error {
		casting := p.casting
		// Pushed even when the stop fails, or nothing would let the tab go.
		stopped := p.stopCast(nil)
		p.tabs.push(t)
		return errors.Join(stopped, p.showFront(casting))
	})
	if err != nil && p.Alive() {
		slog.Warn("browser: bring a new tab to the front", "url", url, "err", err)
	}
	p.reportInfo()
}

// dropTab forgets the tab id once Chrome closed it, and brings the one under
// it back when it was in front.
func (p *chromePage) dropTab(id target.ID) {
	t, front := p.tabs.remove(id)
	if t == nil {
		return
	}
	defer t.cancel()
	// A tab closed on its dialog sends no event that it closed.
	if p.tabs.dialogClosed(t) {
		p.gate.close()
		p.dialogShown(nil)
	}
	if !front {
		return
	}
	err := p.castPastDialog(func() error {
		// Chrome ended the closed tab's screencast with it.
		casting := p.casting
		p.casting = false
		return p.showFront(casting)
	})
	if err != nil && p.Alive() {
		slog.Warn("browser: bring a tab back to the front", "err", err)
	}
	p.reportInfo()
}

// showFront raises the front tab, lays it out at the page's size and starts
// its frames when casting.
func (p *chromePage) showFront(casting bool) error {
	if err := p.runFree(nil, page.BringToFront()); err != nil {
		return err
	}
	v := p.tabs.lastView()
	switch {
	case v == nil:
		return nil
	case casting:
		return p.startCast(nil, *v)
	default:
		return p.Resize(nil, *v)
	}
}

func (p *chromePage) closeTab(id target.ID) {
	c := chromedp.FromContext(p.ctx)
	closeTab := chromedp.ActionFunc(func(ctx context.Context) error {
		return target.CloseTarget(id).Do(cdp.WithExecutor(ctx, c.Browser))
	})
	if err := p.runFree(nil, closeTab); err != nil {
		slog.Warn("browser: close a tab the page opened", "err", err)
	}
}
