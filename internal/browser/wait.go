package browser

import (
	"context"
	"time"

	"github.com/lichdotdev/lich/api"
)

// textPoll is how often a wait reads the page's text again: short enough that
// the read follows the page by a frame or so, long enough not to keep the
// renderer busy answering.
const textPoll = 100 * time.Millisecond

// maxTextWait bounds a wait for text, after which the page is read as it
// stands. 20 seconds leaves that read its own pageWait under the client's
// 60-second openCall.
const maxTextWait = 20 * time.Second

// hasTextScript takes the text as a JSON string literal.
const hasTextScript = `(document.body ? document.body.innerText : '').includes(%s)`

// waitText holds a read until opts.WaitFor is on the page and opts.WaitGone
// is off it, and says whether it gave up first.
func (s *Service) waitText(opts api.BrowserInfoOptions) (timedOut bool, err error) {
	if opts.WaitFor == "" && opts.WaitGone == "" {
		return false, nil
	}
	sess, err := s.live(opts.ID)
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.textWait)
	defer cancel()
	tick := time.NewTicker(textPoll)
	defer tick.Stop()
	// A failed read is the page between two documents, which a click that
	// navigates leaves for a moment: the wait reads again. A Chrome that died
	// stops it, and the page read after reports that.
	for sess.page.Alive() {
		if ok, err := textSettled(ctx, sess.page, opts); err == nil && ok {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return true, nil
		case <-tick.C:
		}
	}
	return false, nil
}

func textSettled(ctx context.Context, p Page, opts api.BrowserInfoOptions) (bool, error) {
	if opts.WaitFor != "" {
		if has, err := p.HasText(ctx, opts.WaitFor); err != nil || !has {
			return false, err
		}
	}
	if opts.WaitGone == "" {
		return true, nil
	}
	has, err := p.HasText(ctx, opts.WaitGone)
	return !has, err
}
