package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lichdotdev/lich/browserwire"
)

// loadTrack counts the front tab's main-frame loads from lich-browser's load
// events: started, and ended once committed, finished or failed.
type loadTrack struct {
	mu             sync.Mutex
	started, ended int
	// failed: the last load to end failed before it committed.
	failed    bool
	startedAt browserTime
	// wake is closed and replaced at each event.
	wake chan struct{}
}

func (l *loadTrack) note(step string, at browserTime) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch step {
	case browserwire.LoadStarted:
		l.started++
		l.startedAt = at
	case browserwire.LoadCommitted, browserwire.LoadFinished, browserwire.LoadFailed:
		if l.ended < l.started {
			l.ended, l.failed = l.started, step == browserwire.LoadFailed
		}
	default:
		return
	}
	close(l.wake)
	l.wake = make(chan struct{})
}

// mark is the loads started so far, which await counts from.
func (l *loadTrack) mark() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.started
}

// await waits for a load past from to start, giving up after startWait, then
// for it to end, and says whether it failed. A request that loads nothing,
// a fragment or a pushState, starts no load.
func (l *loadTrack) await(ctx context.Context, from int, startWait time.Duration) (failed bool, err error) {
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
				return false, nil
			}
		case <-ctx.Done():
			return false, ctx.Err()
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

// inflight is the front tab's subresource requests in flight, by when each
// began. The wire names no request, so a request that finishes takes the
// latest start off: a long poll begun before an action stays in flight and
// never holds it, as Chrome's own fetches do.
type inflight struct {
	mu sync.Mutex
	at []browserTime
}

func (f *inflight) note(ev browserwire.RequestEvent, heard time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case ev.Started:
		f.at = append(f.at, browserTime{wall: heard, ticks: time.Duration(ev.At)})
	case ev.Finished && len(f.at) > 0:
		f.at = f.at[:len(f.at)-1]
	}
}

func (f *inflight) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.at = nil
}

// since says whether one that began at or after start is still in flight.
func (f *inflight) since(start browserTime) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, at := range f.at {
		if !at.before(start) {
			return true
		}
	}
	return false
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
// settleAfter does.
func (p *webkitPage) load(ctx context.Context, method string, params any) (failed bool, err error) {
	err = p.gated(ctx, func(ctx context.Context) error {
		from := p.loads.mark()
		if err := p.do(ctx, method, params, nil); err != nil {
			return err
		}
		failed, err = p.loads.await(ctx, from, domSettle)
		if err != nil || failed {
			return err
		}
		return awaitReady(ctx, p.evalRaw)
	})
	return failed, err
}

// loadError is a navigation that failed before it committed. WebKit fails a
// navigation that became a download the same way, so it is errNavAborted,
// which the service tells from a page that did not load by the download.
type loadError struct{ url string }

func (e loadError) Error() string { return "page load error: " + e.url + " did not load" }

func (e loadError) Is(target error) bool { return target == errNavAborted }
