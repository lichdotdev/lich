package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/performance"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// quietFor and quietMax are chrome-devtools-mcp's stableDomFor and
// stableDomTimeout (src/utils/WaitForHelper.ts): an action's page answer
// waits for the document to go quietFor without a change, and a page that
// never stops changing answers at quietMax.
const (
	quietFor = 100 * time.Millisecond
	quietMax = 3 * time.Second
)

// settlePoll is how often a settle looks at the page again.
const settlePoll = 40 * time.Millisecond

// Reading the clock must give up promptly if Chrome is busy or held.
const actionClockWait = 500 * time.Millisecond

type browserTime struct {
	wall  time.Time
	ticks time.Duration
}

// Using Chrome's ticks on both sides keeps system clock changes out of the comparison.
func (at browserTime) before(other browserTime) bool {
	if at.ticks != 0 && other.ticks != 0 {
		return at.ticks < other.ticks
	}
	return at.wall.Before(other.wall)
}

// fetches are a tab's fetch and XHR requests in flight, by when they began.
type fetches struct {
	mu sync.Mutex
	at map[network.RequestID]browserTime
}

// track keeps ev when it is a fetch or XHR beginning or a request ending, and
// says whether it was a request's event. heard is when lich got ev.
func (f *fetches) track(ev any, heard time.Time) bool {
	switch e := ev.(type) {
	case *network.EventRequestWillBeSent:
		if e.Type == network.ResourceTypeFetch || e.Type == network.ResourceTypeXHR {
			f.begin(e.RequestID, requestTime(e, heard))
		}
	case *network.EventLoadingFinished:
		f.end(e.RequestID)
	case *network.EventLoadingFailed:
		f.end(e.RequestID)
	default:
		return false
	}
	return true
}

func requestTime(e *network.EventRequestWillBeSent, heard time.Time) browserTime {
	at := browserTime{wall: heard}
	if e.WallTime != nil {
		at.wall = e.WallTime.Time()
	}
	if e.Timestamp != nil {
		at.ticks = e.Timestamp.Time().Sub(*cdp.MonotonicTimeEpoch)
	}
	return at
}

// begin keeps the first time of a request: a redirect sends it again.
func (f *fetches) begin(id network.RequestID, at browserTime) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.at == nil {
		f.at = map[network.RequestID]browserTime{}
	}
	if _, ok := f.at[id]; !ok {
		f.at[id] = at
	}
}

func (f *fetches) end(id network.RequestID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.at, id)
}

// since says whether one that began at or after t is still in flight. One
// from before, a long poll, never holds an action that did not start it.
func (f *fetches) since(start browserTime) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, at := range f.at {
		if !at.before(start) {
			return true
		}
	}
	return false
}

func (p *chromePage) ActionTime(ctx context.Context) (time.Duration, error) {
	var ticks time.Duration
	err := p.run(ctx, performance.Enable(), chromedp.ActionFunc(func(ctx context.Context) error {
		metrics, err := performance.GetMetrics().Do(ctx)
		if err != nil {
			return err
		}
		for _, metric := range metrics {
			if metric.Name == "Timestamp" {
				ticks = time.Duration(metric.Value * float64(time.Second))
				return nil
			}
		}
		return fmt.Errorf("browser clock is unavailable")
	}))
	if err != nil {
		return 0, err
	}
	return ticks, nil
}

// quietScript resolves true once the document has gone quiet ms without a
// mutation, or false at max ms.
const quietScript = `((quiet, max) => new Promise(done => {
  let t, m;
  const end = (ok) => { o.disconnect(); clearTimeout(t); clearTimeout(m); done(ok); };
  const o = new MutationObserver(() => { clearTimeout(t); t = setTimeout(end, quiet, true); });
  o.observe(document, {subtree: true, childList: true, attributes: true, characterData: true});
  t = setTimeout(end, quiet, true);
  m = setTimeout(end, max, false);
}))(%d, %d)`

// chromeSettleQuiet is settleQuiet on t's fetches and document.
func chromeSettleQuiet(t *tab, since browserTime) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		return settleQuiet(ctx, t.fetches.since, domQuiet, since)
	})
}

func domQuiet(ctx context.Context, max time.Duration) (bool, error) {
	var quiet bool
	err := chromedp.Evaluate(fmt.Sprintf(quietScript, quietFor.Milliseconds(), max.Milliseconds()), &quiet,
		func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }).Do(ctx)
	return quiet, err
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
