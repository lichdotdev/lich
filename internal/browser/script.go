package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/pagescript"
)

// evalFunc runs a script expression in a page and decodes its value into
// out, nil to drop it. Every engine's page reads the page through the same
// scripts, each with its own evalFunc.
type evalFunc func(ctx context.Context, expr string, out any) error

// indexCounter is the element index pageInfoScript hands out next.
type indexCounter struct {
	mu   sync.Mutex
	next int
}

func (c *indexCounter) IndexNext() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.next
}

func (c *indexCounter) SetIndexNext(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next = n
}

func readInfo(ctx context.Context, eval evalFunc, idx *indexCounter, within string) (PageInfo, error) {
	arg, err := json.Marshal(within)
	if err != nil {
		return PageInfo{}, err
	}
	base := idx.IndexNext()
	var info struct {
		PageInfo
		Next int `json:"next"`
	}
	if err := eval(ctx, pageInfoScript+"("+strconv.Itoa(base)+", "+string(arg)+")", &info); err != nil {
		return PageInfo{}, err
	}
	if info.Error != "" {
		return PageInfo{}, errors.New(info.Error)
	}
	idx.mu.Lock()
	idx.next = max(idx.next, info.Next)
	idx.mu.Unlock()
	return info.PageInfo, nil
}

func hasText(ctx context.Context, eval evalFunc, text string) (bool, error) {
	literal, err := json.Marshal(text)
	if err != nil {
		return false, err
	}
	var has bool
	return has, eval(ctx, fmt.Sprintf(hasTextScript, literal), &has)
}

// clearFieldJS empties the focused field, inside a same-origin frame too.
// Assigning el.value fires no input event, so a framework that owns the field
// keeps its old value; a delete through the editing commands does.
const clearFieldJS = `(() => {
	` + pagescript.Frames + `
	const el = focused();
	if (!el) return;
	if (typeof el.select === 'function') el.select();
	else el.ownerDocument.execCommand('selectAll');
	el.ownerDocument.execCommand('delete');
})()`

// viewMiddleJS is the viewport's middle, where a scroll with no point turns.
const viewMiddleJS = `({x: innerWidth / 2, y: innerHeight / 2})`

// resolveTarget is the point a click on t lands on, by resolveJS.
func resolveTarget(ctx context.Context, eval evalFunc, t Target) (point, error) {
	args, err := json.Marshal([]any{t.Index, t.Selector, t.X, t.Y, t.shotW, t.shotH, shotScale(t.shotW, t.shotH)})
	if err != nil {
		return point{}, err
	}
	var r *resolved
	if err := eval(ctx, resolveJS+"("+string(args[1:len(args)-1])+")", &r); err != nil {
		return point{}, err
	}
	if r == nil {
		return point{}, fmt.Errorf("no element matches that target: the page may have changed since it was read; read it again for fresh indexes")
	}
	if r.Disabled {
		return point{}, fmt.Errorf("the element is disabled: a click on it does nothing; read the page again to see what enables it")
	}
	if r.Covered != "" {
		return point{}, coveredError(r.Covered)
	}
	if r.Resized {
		return point{}, fmt.Errorf("the viewport is %gx%g, and was %dx%d at your last screenshot: the page was resized since; take a new screenshot before clicking at x and y",
			r.W, r.H, t.shotW, t.shotH)
	}
	if r.Outside {
		return point{}, fmt.Errorf("%g,%g is outside the %gx%g viewport: scroll first, or click by index or selector, which scroll the element into view",
			r.X, r.Y, r.W, r.H)
	}
	return r.point, nil
}

// annotation is annotateJS drawing the elements info lists.
func annotation(info PageInfo) (string, error) {
	els, err := json.Marshal(info.Interactive)
	if err != nil {
		return "", err
	}
	return annotateJS + "(" + string(els) + fmt.Sprintf(", %d, %d, %d)", shotMax, labelPx, annotateLinger.Milliseconds()), nil
}

func elementAt(ctx context.Context, eval evalFunc, x, y float64) (api.BrowserElement, error) {
	var el *api.BrowserElement
	if err := eval(ctx, pagescript.ElementAt(x, y), &el); err != nil {
		return api.BrowserElement{}, err
	}
	if el == nil {
		return api.BrowserElement{}, nil
	}
	return *el, nil
}

func locate(ctx context.Context, eval evalFunc, selectors []string, reveal string) ([]api.BrowserRect, error) {
	script, err := pagescript.Locate(selectors, reveal)
	if err != nil {
		return nil, err
	}
	boxes := []api.BrowserRect{}
	return boxes, eval(ctx, script, &boxes)
}

// awaitReady returns once the document is interactive or domSettle has
// passed, whichever comes first.
func awaitReady(ctx context.Context, eval evalFunc) error {
	deadline := time.Now().Add(domSettle)
	for {
		var state string
		if err := eval(ctx, `document.readyState`, &state); err == nil &&
			(state == "interactive" || state == "complete") {
			return nil
		}
		if time.Now().After(deadline) {
			return nil
		}
		if err := sleepCtx(ctx, settlePoll); err != nil {
			return err
		}
	}
}

// settleQuiet waits, up to quietMax, until the fetches begun since have
// landed (inFlight) and the document then goes quietFor without a change
// (quiet). A fetch that change starts is waited for too.
func settleQuiet(ctx context.Context, inFlight func(browserTime) bool, quiet func(context.Context, time.Duration) (bool, error), since browserTime) error {
	deadline := time.Now().Add(quietMax)
	for time.Now().Before(deadline) {
		if inFlight(since) {
			if err := sleepCtx(ctx, settlePoll); err != nil {
				return err
			}
			continue
		}
		still, err := quiet(ctx, time.Until(deadline))
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			// A navigation replaced the document under the observer.
			if err := sleepCtx(ctx, settlePoll); err != nil {
				return err
			}
		case still && !inFlight(since):
			return nil
		}
	}
	return nil
}
