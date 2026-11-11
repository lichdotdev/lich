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
