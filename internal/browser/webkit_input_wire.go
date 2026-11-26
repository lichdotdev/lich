package browser

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

// Where the window shows a page it does not hold (a layer of lich-browser's
// on macOS, frames on Wayland), the page takes no input of its own: the
// pane's pointer, keys and text go over the wire, as the screencast's went
// to Chrome (docs/lich-browser.md).

var wireMouseKinds = map[string]string{
	"press": browserwire.MouseDown, "release": browserwire.MouseUp, "move": browserwire.MouseMove, "wheel": browserwire.MouseWheel,
}

var wireKeyKinds = map[string]string{"down": browserwire.KeyDown, "up": browserwire.KeyUp}

// wireButtons are the DOM's buttons by the pane's names.
var wireButtons = map[string]int{"left": 0, "middle": 1, "right": 2}

// wireMods is CDP's modifier bit set (Alt 1, Ctrl 2, Meta 4, Shift 8), which
// the pane sends, as the wire's.
func wireMods(cdp int) browserwire.Mods {
	var m browserwire.Mods
	for _, b := range []struct {
		cdp int
		mod browserwire.Mods
	}{{1, browserwire.ModAlt}, {2, browserwire.ModCtrl}, {4, browserwire.ModMeta}, {8, browserwire.ModShift}} {
		if cdp&b.cdp != 0 {
			m |= b.mod
		}
	}
	return m
}

func (p *webkitPage) sendMouse(ctx context.Context, m api.BrowserMouseOptions) error {
	kind, ok := wireMouseKinds[m.Type]
	if !ok {
		return errUnknownInput
	}
	return p.do(ctx, browserwire.Mouse, browserwire.MouseParams{
		Page: p.id, Kind: kind, X: m.X, Y: m.Y, Button: wireButtons[m.Button], Clicks: m.Clicks,
		Mods: wireMods(m.Modifiers), DX: m.DX, DY: m.DY,
	}, nil)
}

func (p *webkitPage) sendKey(ctx context.Context, k api.BrowserKeyOptions) error {
	kind, ok := wireKeyKinds[k.Type]
	if !ok {
		return errUnknownInput
	}
	return p.do(ctx, browserwire.Key, browserwire.KeyParams{Page: p.id, Kind: kind, Key: k.Key, Code: k.Code, Mods: wireMods(k.Modifiers)}, nil)
}

func (p *webkitPage) sendText(ctx context.Context, text string) error {
	return p.do(ctx, browserwire.Text, browserwire.TextParams{Page: p.id, Text: text}, nil)
}
