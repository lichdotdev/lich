package main

import (
	"image"
	"math"

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
	n.cuts = append(n.cuts, roundedCut(r, top, bottom)...)
}

// roundedCut is r with its top corners rounded by top and its bottom ones by
// bottom, as the rows of pixels a rounded corner leaves whole.
func roundedCut(r image.Rectangle, top, bottom int) []image.Rectangle {
	top, bottom = min(top, r.Dy()/2, r.Dx()/2), min(bottom, r.Dy()/2, r.Dx()/2)
	rows := []image.Rectangle{image.Rect(r.Min.X, r.Min.Y+top, r.Max.X, r.Max.Y-bottom)}
	for i := range top {
		in := cornerInset(top, i)
		rows = append(rows, image.Rect(r.Min.X+in, r.Min.Y+i, r.Max.X-in, r.Min.Y+i+1))
	}
	for i := range bottom {
		in := cornerInset(bottom, i)
		rows = append(rows, image.Rect(r.Min.X+in, r.Max.Y-i-1, r.Max.X-in, r.Max.Y-i))
	}
	return rows
}

// cornerInset is how far in from the side a corner of radius rounds row i,
// counted from the edge it rounds.
func cornerInset(radius, i int) int {
	dy := float64(radius) - float64(i) - 0.5
	return int(math.Round(float64(radius) - math.Sqrt(float64(radius*radius)-dy*dy)))
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
