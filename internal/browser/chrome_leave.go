package browser

import (
	"context"
	"log/slog"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// A beforeunload accepted too soon after it opened has Chrome abort the
// navigation it held and reload the page instead, and how soon is too soon
// grows with the machine's load: about 450 ms on an idle one, 2 to 3 s with
// the browser held to 30% of a core (Chromium 155, measured 2026-10-30). No
// CDP event marks the moment, so beforeunloadSettle is a guess, and a leave
// watches for the reload and goes where the person said to go once it lands.

// leaveTries is how many aborted leaves are taken up again before the page
// is left where Chrome put it.
const leaveTries = 3

// leaveWait bounds how long Settle waits on a leave: a held navigation that
// turned into a download never lands.
const leaveWait = 15 * time.Second

// leave is a beforeunload lich accepted, until the navigation it held lands.
type leave struct {
	tab *tab
	url string
	// aborted: Chrome reloaded the page instead.
	aborted bool
	tries   int
	done    chan struct{}
}

// holdLeave keeps url, the navigation the dialog t just opened holds, for
// the accept that would leave for it; "" for a dialog that holds none.
func (p *chromePage) holdLeave(t *tab, url string) {
	p.leaveMu.Lock()
	defer p.leaveMu.Unlock()
	p.held = &leave{tab: t, url: url}
}

// leaving is the answer to the open dialog: the leave an accepted
// beforeunload starts, nil for any other answer.
func (p *chromePage) leaving(accept bool) *leave {
	p.leaveMu.Lock()
	defer p.leaveMu.Unlock()
	held := p.held
	p.held = nil
	if !accept || held == nil || held.url == "" {
		return nil
	}
	held.done = make(chan struct{})
	p.leave = held
	return held
}

// forgetLeave drops l, the leave of an answer that never reached Chrome.
func (p *chromePage) forgetLeave(l *leave) {
	p.leaveMu.Lock()
	defer p.leaveMu.Unlock()
	if l != nil && p.leave == l {
		close(l.done)
		p.leave = nil
	}
}

// leaveStarted marks the leave on t aborted when Chrome starts reloading the
// page instead.
func (p *chromePage) leaveStarted(t *tab, kind page.FrameStartedNavigatingNavigationType) {
	p.leaveMu.Lock()
	defer p.leaveMu.Unlock()
	if l := p.leave; l != nil && l.tab == t && kind == page.FrameStartedNavigatingNavigationTypeReload {
		l.aborted = true
	}
}

// leaveLanded ends the leave on t once a document lands, or, when it is the
// reload of an aborted one, goes for the held navigation again. The reloaded
// document has no user activation, so that navigation asks nothing.
func (p *chromePage) leaveLanded(t *tab) {
	p.leaveMu.Lock()
	defer p.leaveMu.Unlock()
	l := p.leave
	if l == nil || l.tab != t {
		return
	}
	if !l.aborted || l.tries >= leaveTries {
		close(l.done)
		p.leave = nil
		return
	}
	l.aborted = false
	l.tries++
	go p.leaveAgain(t, l.url)
}

func (p *chromePage) leaveAgain(t *tab, url string) {
	ctx, cancel := context.WithTimeout(t.ctx, pageWait)
	defer cancel()
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, _, _, err := page.Navigate(url).Do(ctx)
		return err
	})); err != nil && p.Alive() {
		slog.Warn("browser: take up a leave Chrome aborted", "url", url, "err", err)
	}
}

// reloadAsks accepts, without showing it, the beforeunload a reload of an
// aborted leave on t opens, since the person already chose to leave; false
// when there is no such leave. It waits longer each try for the same reason
// the first accept waited.
func (p *chromePage) reloadAsks(t *tab) bool {
	p.leaveMu.Lock()
	l := p.leave
	ok := l != nil && l.tab == t && l.aborted
	var wait time.Duration
	if ok {
		wait = time.Duration(l.tries+2) * beforeunloadSettle
	}
	p.leaveMu.Unlock()
	if !ok {
		return false
	}
	go func() {
		time.Sleep(wait)
		ctx, cancel := context.WithTimeout(t.ctx, pageWait)
		defer cancel()
		if err := chromedp.Run(ctx, page.HandleJavaScriptDialog(true)); err != nil && p.Alive() {
			slog.Warn("browser: accept the beforeunload of a reload", "err", err)
		}
	}()
	return true
}

// awaitLeave waits for a leave in flight to land, bounded by ctx and
// leaveWait.
func (p *chromePage) awaitLeave(ctx context.Context) {
	p.leaveMu.Lock()
	l := p.leave
	p.leaveMu.Unlock()
	if l == nil {
		return
	}
	t := time.NewTimer(leaveWait)
	defer t.Stop()
	select {
	case <-l.done:
	case <-ctx.Done():
	case <-t.C:
	}
}
