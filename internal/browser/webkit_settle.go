package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/lichdotdev/lich/browserwire"
)

// loadTrack counts the front tab's main-frame loads from lich-browser's load
// events: started, and ended once committed, finished or failed.
type loadTrack struct {
	mu             sync.Mutex
	started, ended int
	// failed is how the last load to end failed before it committed, nil
	// when it committed.
	failed    error
	startedAt browserTime
	// wake is closed and replaced at each event.
	wake chan struct{}
}

func (l *loadTrack) note(ev browserwire.LoadEvent, heard time.Time) {
	// The blank page open and newTab start on their own loads after they
	// answered, so it landed after the navigation that followed was sent,
	// and ending first stood for it: open answered about:blank, untitled.
	if ev.URL == blankURL {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	switch ev.Step {
	case browserwire.LoadStarted:
		l.started++
		l.startedAt = browserTime{wall: heard, ticks: time.Duration(ev.At)}
	case browserwire.LoadCommitted, browserwire.LoadFinished, browserwire.LoadFailed:
		if l.ended < l.started {
			l.ended, l.failed = l.started, loadFailure(ev)
		}
	default:
		return
	}
	close(l.wake)
	l.wake = make(chan struct{})
}

const blankURL = "about:blank"

// mark is the loads started so far, which await counts from.
func (l *loadTrack) mark() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.started
}

// loadFailure is how ev's load failed: nil for none, errNavDownload for a
// navigation that became a download, else WebKit's error.
func loadFailure(ev browserwire.LoadEvent) error {
	switch {
	case ev.Step != browserwire.LoadFailed:
		return nil
	case ev.Download:
		return errNavDownload
	}
	return fmt.Errorf("page load error %s", ev.Error)
}

// await waits for a load past from to start, giving up after startWait, then
// for it to end, and returns how it failed. A request that loads nothing, a
// fragment or a pushState, starts no load.
func (l *loadTrack) await(ctx context.Context, from int, startWait time.Duration) (failed, err error) {
	noStart := time.NewTimer(startWait)
	defer noStart.Stop()
	for {
		l.mu.Lock()
		started, ended, failed, wake := l.started, l.ended, l.failed, l.wake
		l.mu.Unlock()
		if started > from && ended >= started {
			return failed, nil
		}
		select {
		case <-wake:
		case <-noStart.C:
			if started == from {
				return nil, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// awaitSince waits, up to wait, for a load that started at or after since to
// end: the action that started it is the one being settled.
func (l *loadTrack) awaitSince(ctx context.Context, since browserTime, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		l.mu.Lock()
		pending := l.ended < l.started && !l.startedAt.before(since)
		wake := l.wake
		l.mu.Unlock()
		if !pending {
			return nil
		}
		select {
		case <-wake:
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// noteRequest keeps a fetch or XHR of the front tab beginning, and any
// request ending, for the settle, as Chrome's page keeps them.
func (p *webkitPage) noteRequest(ev browserwire.RequestEvent, heard time.Time) {
	switch {
	case ev.Started && (ev.Type == browserwire.RequestFetch || ev.Type == browserwire.RequestXHR):
		p.requests.begin(network.RequestID(ev.ID), browserTime{wall: heard, ticks: time.Duration(ev.At)})
	case ev.Finished:
		p.requests.end(network.RequestID(ev.ID))
	}
}

func (p *webkitPage) ActionTime(ctx context.Context) (time.Duration, error) {
	var c browserwire.ClockResult
	if err := p.do(ctx, browserwire.Clock, nil, &c); err != nil {
		return 0, err
	}
	return time.Duration(c.NS), nil
}

// Settle waits out navGrace, a load the action started up to domSettle, the
// new document's readiness, and then its fetches and changes as Chrome's
// page does.
func (p *webkitPage) Settle(ctx context.Context, since browserTime) error {
	if err := sleepCtx(ctx, navGrace); err != nil {
		return err
	}
	return p.gated(ctx, func(ctx context.Context) error {
		if err := p.loads.awaitSince(ctx, since, domSettle); err != nil {
			return err
		}
		if err := awaitReady(ctx, p.evalRaw); err != nil {
			return err
		}
		return settleQuiet(ctx, p.requests.since, p.quiet, since)
	})
}

func (p *webkitPage) quiet(ctx context.Context, max time.Duration) (bool, error) {
	var quiet bool
	return quiet, p.evalRaw(ctx, fmt.Sprintf(quietScript, quietFor.Milliseconds(), max.Milliseconds()), &quiet)
}

// load sends a request that loads the front tab, then waits for the load it
// starts to end and for the new document's readiness, as Chrome's
// settleAfter does. A load that failed is a failedLoad.
func (p *webkitPage) load(ctx context.Context, method string, params any) error {
	return p.gated(ctx, func(ctx context.Context) error {
		from := p.loads.mark()
		if err := p.do(ctx, method, params, nil); err != nil {
			return err
		}
		failed, err := p.loads.await(ctx, from, domSettle)
		if err != nil {
			return err
		}
		if failed != nil {
			return failedLoad{failed}
		}
		return awaitReady(ctx, p.evalRaw)
	})
}

// failedLoad is a load that ended before it committed: an error to the
// agent's navigation, none to a reload or a history step, which leave the
// page as WebKit shows it, as Chrome's do.
type failedLoad struct{ err error }

func (e failedLoad) Error() string { return e.err.Error() }

func (e failedLoad) Unwrap() error { return e.err }
