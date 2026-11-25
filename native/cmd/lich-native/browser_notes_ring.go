package main

import (
	"image"

	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// A pick's outline is a dashed ring a few dp outside the element, so it never
// merges with the element's own border, its dashes marching while the pointer
// is on the element or its note is in focus.

var (
	noteRingGap    = unit.Dp(5)
	noteRingRadius = unit.Dp(8)
	noteRingWidth  = unit.Dp(1.5)
)

// noteRing is the ring around the element at r.
func noteRing(gtx C, r image.Rectangle) image.Rectangle {
	return r.Inset(-gtx.Dp(noteRingGap))
}

// cut marks r, local to the page, as drawn over it, its top corners rounded
// by top and its bottom ones by bottom, so the page still shows past a
// rounded corner.
func (n *browserNotes) cut(r image.Rectangle, top, bottom int) {
	n.cuts = append(n.cuts, ui.RoundedRows(r, top, bottom)...)
}

// pageRing is a ring a page the window draws itself draws over its own
// content, in its CSS pixels: drawn by the window and cut out of the page, a
// ring lost its faint fill and showed the pane's background behind its
// dashes, a dark frame round the element.
type pageRing struct {
	X, Y, W, H, Radius, Width, Dash float32
	// Fill is the fill's alpha, as notePick's over the element.
	Fill uint8
	// March sets the dashes marching, as ui.MarchingRing's do.
	March bool
}

// ringOnPage hands the page ring r, in the pane's pixels at scale, to draw.
func (n *browserNotes) ringOnPage(gtx C, r image.Rectangle, scale float32, fill uint8, march bool) {
	css := func(px int) float32 { return float32(px) / scale }
	n.rings = append(n.rings, pageRing{
		X: css(r.Min.X), Y: css(r.Min.Y), W: css(r.Dx()), H: css(r.Dy()),
		Radius: css(gtx.Dp(noteRingRadius)), Width: css(gtx.Dp(noteRingWidth)), Dash: css(gtx.Dp(ui.RingDash)),
		Fill: fill, March: march,
	})
}
