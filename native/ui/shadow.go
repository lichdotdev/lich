package ui

import (
	"image"
	"math"

	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// shadowStep is how much of a shadow's blur one ring stands in for: a wider
// step shows as bands.
const shadowStep unit.Dp = 2

// Elevation is a floating surface's shadow, its BoxShadows painted in order.
// The window has three, by how high the surface stands (DESIGN.md, Elevation):
// a tooltip low, what opens over a screen (menus, popovers, the composer's
// panels, toasts) in the middle, and a dialog high.
type Elevation []BoxShadow

var (
	ElevationLow  = Elevation{{Y: 2, Blur: 6, Alpha: 0.3}}
	ElevationMid  = Elevation{{Y: 12, Blur: 32, Alpha: 0.45}, {Y: 2, Blur: 6, Alpha: 0.3}}
	ElevationHigh = Elevation{{Y: 24, Blur: 56, Alpha: 0.55}, {Y: 4, Blur: 12, Alpha: 0.35}}
)

// Paint paints each shadow under a box of size with corners of radius.
func (e Elevation) Paint(gtx C, size image.Point, radius int) {
	for _, s := range e {
		s.Paint(gtx, size, radius)
	}
}

// FloatRingOpacity is the 1px ring in Foreground every floating surface
// wears over its shadow, the menus' ring-foreground/10: a shadow is lost on a
// light theme, the ring is not.
const FloatRingOpacity = menuRingOpacity

// Reach is how far under its box the elevation's shadow falls: what a
// surface standing over something it must not darken keeps clear.
func (e Elevation) Reach() unit.Dp {
	var r unit.Dp
	for _, s := range e {
		r = max(r, s.Y+s.Blur/2)
	}
	return r
}

// BoxShadow is CSS's box-shadow 0 Y Blur rgba(0,0,0,Alpha).
type BoxShadow struct {
	Y, Blur unit.Dp
	Alpha   float64
}

// Paint stands in for the blur with rings growing from half the blur inside
// the box of size to half outside it, so that all of them together are black
// at Alpha, compensated for the linear-light blend as dialogScrimOpacity is.
// The shadow lands on whatever the window holds, not a known surface, so it
// stays translucent.
func (s BoxShadow) Paint(gtx C, size image.Point, radius int) {
	blur := float32(gtx.Dp(s.Blur))
	down := gtx.Dp(s.Y)
	layers := max(1, int(s.Blur/shadowStep))
	shade := Alpha(black, float32(1-math.Pow(math.Pow(1-s.Alpha, 2.4), 1/float64(layers))))
	for i := range layers {
		grow := int(blur*(float32(i)+0.5)/float32(layers) - blur/2)
		box := image.Rect(-grow, down-grow, size.X+grow, size.Y+down+grow)
		paint.FillShape(gtx.Ops, shade, clip.UniformRRect(box, max(radius+grow, 0)).Op(gtx.Ops))
	}
}
