package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// ringPixel draws w at 1x and reads the ring's leftmost column, halfway down.
func ringPixel(t *testing.T, w layout.Widget) color.RGBA {
	t.Helper()
	side := int(2 * sessionTreeRingR)
	win, err := headless.NewWindow(side, side)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	ops := new(op.Ops)
	w(layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(side, side))})
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img.RGBAAt(0, side/2)
}

// A session's ring on a pink tray drew over the card's own fill: pinker
// than the checkout's ring beside it, and different again under the pointer
// or in the rail, which drew it over the sidebar.
func TestATreeRingMixesOverItsRowsGround(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Surface: ui.Dark.Sidebar}
	tray := nodeGround(th, blockView{kind: blockFolder, color: "pink"})
	var s sidebar
	ring := func(card *ui.Theme) color.RGBA {
		sv := sessionView{place: treePlace{tree: true, ground: tray}}
		return ringPixel(t, func(gtx C) D { return s.statusGlyph(gtx, card, &sv) })
	}
	checkout := ringPixel(t, func(gtx C) D { return checkoutNode(gtx, onGround(th, tray), icBranch, treeEntry{}) })
	cases := map[string]*ui.Theme{
		"rail":         th,
		"card at rest": th.On(cardFill(th, "pink", false, false)),
		"hovered card": th.On(cardFill(th, "pink", false, true)),
		"active card":  th.On(cardFill(th, "pink", true, false)),
	}
	for name, card := range cases {
		if got := ring(card); got != checkout {
			t.Errorf("%s: ring %v, want the checkout's %v on the same tray", name, got, checkout)
		}
	}
	if bare := ringPixel(t, func(gtx C) D { return checkoutNode(gtx, th, icBranch, treeEntry{}) }); bare == checkout {
		t.Errorf("ring on the tray %v, want it tinted apart from one on the sidebar %v", checkout, bare)
	}
}

func TestAHeadsGlyphIsTheOpenHeadersInTheRail(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Surface: ui.Dark.Sidebar}
	if got, want := headGlyphColor(th, ""), th.Over(ui.Alpha(th.MutedForeground, headerTitleOpacity)); got != want {
		t.Errorf("uncolored head glyph %v, want the title's %v", got, want)
	}
	if got := headGlyphColor(th, "pink"); got != cardColors["pink"] {
		t.Errorf("pink head glyph %v, want the folder's color", got)
	}
}
