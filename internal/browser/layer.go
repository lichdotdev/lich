package browser

import (
	"context"
	"errors"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

// The window's layer over a page lich-browser shows in it (browser.Draw,
// browser.Snapshot): a page Chrome holds is a screencast the window draws
// over itself, and has none.

var errNoLayer = errors.New("the window draws over this page itself: it is not lich-browser's")

// Draw hands the page the window's layer, at once: lich-browser runs the
// newest drawing once the page ran the last.
func (s *Service) Draw(_ context.Context, opts api.BrowserDrawOptions) (api.None, error) {
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error {
		wp, ok := p.(*webkitPage)
		if !ok {
			return errNoLayer
		}
		return wp.do(ctx, browserwire.Draw, browserwire.DrawParams{Page: wp.id, Script: opts.Script}, nil)
	})
	return api.None{}, err
}

// Snapshot is the PNG of what the page shows, the window's layer left out.
func (s *Service) Snapshot(_ context.Context, opts api.SessionIDOptions) ([]byte, error) {
	var snap browserwire.SnapshotResult
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error {
		wp, ok := p.(*webkitPage)
		if !ok {
			return errNoLayer
		}
		return wp.do(ctx, browserwire.Snapshot, browserwire.SnapshotParams{Page: wp.id, Bare: true}, &snap)
	})
	return snap.PNG, err
}
