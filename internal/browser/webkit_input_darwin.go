package browser

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// On macOS the window shows a page as a layer of lich-browser's, which takes
// no input of its own.

func (p *webkitPage) Mouse(ctx context.Context, m api.BrowserMouseOptions) error {
	return p.sendMouse(ctx, m)
}

func (p *webkitPage) Key(ctx context.Context, k api.BrowserKeyOptions) error {
	return p.sendKey(ctx, k)
}

func (p *webkitPage) InsertText(ctx context.Context, text string) error {
	return p.sendText(ctx, text)
}
