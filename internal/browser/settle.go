package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
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

// fetches are a tab's fetch and XHR requests in flight, by when lich heard
// each begin.
type fetches struct {
	mu sync.Mutex
	at map[network.RequestID]time.Time
}

// track keeps ev when it is a fetch or XHR beginning or a request ending, and
// says whether it was a request's event.
func (f *fetches) track(ev any, at time.Time) bool {
	switch e := ev.(type) {
	case *network.EventRequestWillBeSent:
		if e.Type == network.ResourceTypeFetch || e.Type == network.ResourceTypeXHR {
			f.begin(e.RequestID, at)
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

// begin keeps the first time a request is heard: a redirect sends it again.
func (f *fetches) begin(id network.RequestID, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.at == nil {
		f.at = map[network.RequestID]time.Time{}
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
func (f *fetches) since(t time.Time) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, at := range f.at {
		if !at.Before(t) {
			return true
		}
	}
	return false
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

// settleQuiet waits, up to quietMax, until the fetches t began since have
// landed and its document then goes quietFor without a change. A fetch that
// change starts is waited for too.
func settleQuiet(t *tab, since time.Time) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		deadline := time.Now().Add(quietMax)
		for time.Now().Before(deadline) {
			if t.fetches.since(since) {
				if err := sleepCtx(ctx, settlePoll); err != nil {
					return err
				}
				continue
			}
			quiet, err := domQuiet(ctx, time.Until(deadline))
			switch {
			case ctx.Err() != nil:
				return ctx.Err()
			case err != nil:
				// A navigation replaced the document under the observer.
				if err := sleepCtx(ctx, settlePoll); err != nil {
					return err
				}
			case quiet && !t.fetches.since(since):
				return nil
			}
		}
		return nil
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
