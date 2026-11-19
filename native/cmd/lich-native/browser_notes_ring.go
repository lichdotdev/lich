package main

import (
	"image"

	"gioui.org/unit"
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

// noteRingBand is half the band a ring and its halo cover across its edge.
const noteRingBand = unit.Dp(2.5)

// cut marks r, local to the page, as drawn over it.
func (n *browserNotes) cut(r image.Rectangle) {
	n.cuts = append(n.cuts, r)
}

// cutRing marks the band ring r is drawn on, not what it rings, which a page
// the window draws itself keeps showing: the ring's faint fill is lost there.
func (n *browserNotes) cutRing(gtx C, r image.Rectangle) {
	t := gtx.Dp(noteRingBand)
	n.cut(image.Rect(r.Min.X-t, r.Min.Y-t, r.Max.X+t, r.Min.Y+t))
	n.cut(image.Rect(r.Min.X-t, r.Max.Y-t, r.Max.X+t, r.Max.Y+t))
	n.cut(image.Rect(r.Min.X-t, r.Min.Y+t, r.Min.X+t, r.Max.Y-t))
	n.cut(image.Rect(r.Max.X-t, r.Min.Y+t, r.Max.X+t, r.Max.Y-t))
}
