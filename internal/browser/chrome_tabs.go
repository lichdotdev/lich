package browser

import (
	"context"
	"errors"
	"log/slog"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// followTabs keeps the tab list in step with Chrome: what a tab opens in a
// new tab or window joins it, in front when its opener was, each tab's URL
// and title as they change, and a tab Chrome closed leaves it.
func (p *chromePage) followTabs() {
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
		if info == nil || info.Type != "page" {
			return
		}
		if t := p.tabs.byID(info.TargetID); t != nil {
			if p.tabs.setInfo(t, info.URL, info.Title) {
				p.tabsChanged()
			}
			return
		}
		opener := p.tabs.byID(info.OpenerID)
		// A new tab is created at about:blank and gets its URL after.
		if opener == nil || info.URL == "" || info.URL == "about:blank" {
			return
		}
		// Both the created and the info-changed event can name the tab.
		if _, taken := p.listen.taken.LoadOrStore(info.TargetID, true); !taken {
			go p.bringIn(info.TargetID, info.URL, opener)
		}
	})
}

// bringIn attaches to the tab id opener opened, and puts it in front when
// opener was.
func (p *chromePage) bringIn(id target.ID, url string, opener *tab) {
	if _, err := allowURL(url); err != nil {
		slog.Warn("browser: a new tab's URL is not one the page may load", "url", url, "err", err)
		p.closeTarget(id)
		return
	}
	ctx, cancel := chromedp.NewContext(p.ctx, chromedp.WithTargetID(id))
	t := &tab{ctx: ctx, cancel: cancel, id: id, opener: opener, url: url}
	p.listenTab(t)
	if err := chromedp.Run(ctx, interceptFiles); err != nil {
		cancel()
		if p.Alive() {
			slog.Warn("browser: attach to a tab the page opened", "url", url, "err", err)
		}
		return
	}
	p.tabs.add(t)
	if p.tabs.frontTab() != opener {
		p.tabsChanged()
		return
	}
	if err := p.toFront(t); err != nil && p.Alive() {
		slog.Warn("browser: bring a new tab to the front", "url", url, "err", err)
	}
}

// NewTab opens a blank tab in front.
func (p *chromePage) NewTab(context.Context) error {
	ctx, cancel := chromedp.NewContext(p.ctx)
	t := &tab{ctx: ctx, cancel: cancel, url: "about:blank"}
	p.listenTab(t)
	if err := chromedp.Run(ctx, interceptFiles); err != nil {
		cancel()
		return err
	}
	t.id = chromedp.FromContext(ctx).Target.TargetID
	p.listen.taken.Store(t.id, true)
	p.tabs.add(t)
	return p.toFront(t)
}

// SelectTab brings the tab id to the front.
func (p *chromePage) SelectTab(_ context.Context, id string) error {
	t := p.tabs.byID(target.ID(id))
	if t == nil {
		return errNoTab
	}
	if t == p.tabs.frontTab() {
		return nil
	}
	return p.toFront(t)
}

// CloseTab closes the tab id, never the last one.
func (p *chromePage) CloseTab(ctx context.Context, id string) error {
	if p.tabs.byID(target.ID(id)) == nil {
		return errNoTab
	}
	if p.tabs.count() == 1 {
		return errLastTab
	}
	c := chromedp.FromContext(p.ctx)
	err := p.runFree(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return target.CloseTarget(target.ID(id)).Do(cdp.WithExecutor(ctx, c.Browser))
	}))
	if err != nil {
		return err
	}
	// Chrome's word that it closed comes after; the caller reads the tab that
	// comes to the front now.
	p.dropTab(target.ID(id))
	return nil
}

// toFront stops the front tab's frames, puts t in front and starts them there.
func (p *chromePage) toFront(t *tab) error {
	err := p.castPastDialog(func() error {
		casting := p.casting
		// Set even when the stop fails, or the tab would never come to the front.
		stopped := p.stopCast(nil)
		p.tabs.setFront(t)
		return errors.Join(stopped, p.showFront(casting))
	})
	p.tabsChanged()
	p.reportInfo()
	return err
}

// dropTab forgets the tab id once it closed, and brings another to the front
// when it was there.
func (p *chromePage) dropTab(id target.ID) {
	t, front := p.tabs.remove(id)
	if t == nil {
		return
	}
	if t.cancel != nil {
		defer t.cancel()
	}
	// A tab closed on its dialog sends no event that it closed.
	if p.tabs.dialogClosed(t) {
		p.gate.close()
		p.dialogShown(nil)
	}
	p.tabsChanged()
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

func (p *chromePage) closeTarget(id target.ID) {
	c := chromedp.FromContext(p.ctx)
	closeTab := chromedp.ActionFunc(func(ctx context.Context) error {
		return target.CloseTarget(id).Do(cdp.WithExecutor(ctx, c.Browser))
	})
	if err := p.runFree(nil, closeTab); err != nil {
		slog.Warn("browser: close a tab the page opened", "err", err)
	}
}

func (p *chromePage) tabsChanged() {
	p.listen.mu.Lock()
	onTabs := p.listen.onTabs
	p.listen.mu.Unlock()
	if onTabs != nil {
		onTabs(p.tabs.list())
	}
}
