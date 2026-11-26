//go:build !darwin

package browser

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

// Mouse passes on the wheel alone where the window embeds the page: the
// window sends it while Select is on, when the page lets the pointer through
// to the window and X gives the page no wheel. Everything else reaches the
// page from the X server. A page drawn by frames takes it all over the wire.
func (p *webkitPage) Mouse(ctx context.Context, m api.BrowserMouseOptions) error {
	if p.conn.frames {
		return p.sendMouse(ctx, m)
	}
	if m.Type != "wheel" {
		return nil
	}
	return p.do(ctx, browserwire.Mouse, browserwire.MouseParams{Page: p.id, Kind: browserwire.MouseWheel, X: m.X, Y: m.Y, DX: m.DX, DY: m.DY}, nil)
}

func (p *webkitPage) Key(ctx context.Context, k api.BrowserKeyOptions) error {
	if p.conn.frames {
		return p.sendKey(ctx, k)
	}
	return nil
}

func (p *webkitPage) InsertText(ctx context.Context, text string) error {
	if p.conn.frames {
		return p.sendText(ctx, text)
	}
	return nil
}
