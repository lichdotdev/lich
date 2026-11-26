//go:build !darwin

package browser

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

// Mouse passes on the wheel alone: the window sends it while Select is on,
// when the page lets the pointer through to the window and X gives the page
// no wheel. Everything else reaches the page from the X server.
func (p *webkitPage) Mouse(ctx context.Context, m api.BrowserMouseOptions) error {
	if m.Type != "wheel" {
		return nil
	}
	return p.do(ctx, browserwire.Mouse, browserwire.MouseParams{Page: p.id, Kind: browserwire.MouseWheel, X: m.X, Y: m.Y, DX: m.DX, DY: m.DY}, nil)
}

func (p *webkitPage) Key(context.Context, api.BrowserKeyOptions) error { return nil }

func (p *webkitPage) InsertText(context.Context, string) error { return nil }
