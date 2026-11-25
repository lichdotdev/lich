package browser

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// The pane's notes: the person points at an element of the page and leaves a
// note on it for the agent. The page only reports what is where; the pane
// draws the outline and the pins, so nothing is injected into its DOM.

func (p *chromePage) ElementAt(ctx context.Context, x, y float64) (api.BrowserElement, error) {
	return elementAt(ctx, p.evaluate, x, y)
}

func (p *chromePage) Locate(ctx context.Context, selectors []string, reveal string) ([]api.BrowserRect, error) {
	return locate(ctx, p.evaluate, selectors, reveal)
}

// ElementAtPoint is the element under a point of this session's page.
func (s *Service) ElementAtPoint(_ context.Context, opts api.BrowserPointOptions) (api.BrowserElement, error) {
	var el api.BrowserElement
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error {
		var err error
		el, err = p.ElementAt(ctx, opts.X, opts.Y)
		return err
	})
	return el, err
}

// LocateElements is where the elements opts.Selectors find stand now, after
// scrolling opts.Reveal's element into view when it is set.
func (s *Service) LocateElements(_ context.Context, opts api.BrowserLocateOptions) ([]api.BrowserRect, error) {
	var boxes []api.BrowserRect
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error {
		var err error
		boxes, err = p.Locate(ctx, opts.Selectors, opts.Reveal)
		return err
	})
	return boxes, err
}
