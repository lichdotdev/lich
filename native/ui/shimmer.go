package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// ShimmerLap is how long the highlight takes to cross a text and start again.
const ShimmerLap = 1600 * time.Millisecond

// shimmerBand is the highlight's width as a share of the text's, and
// shimmerMinBand its floor in pixels, so a short word still catches it.
const (
	shimmerBand    = 0.4
	shimmerMinBand = 28
	// shimmerLight is how far the base color goes toward the foreground at
	// the highlight's centre.
	shimmerLight = 0.85
)

// Shimmer sweeps a light band over the text from left to right, once every
// ShimmerLap, for as long as it is drawn: a text that is alive. It asks for
// the looping frame itself, so a caller draws it only while the thing it
// names is alive.
func (l LabelStyle) Shimmer() LabelStyle { l.shimmer = true; return l }

// shimmerAt is where the highlight's centre stands at now over a text width
// wide, and how wide the band is: it enters from the left of the text, crosses
// it and leaves by the right, once a lap.
func shimmerAt(now time.Time, width int) (centre, band int) {
	phase := float32(now.UnixNano()%int64(ShimmerLap)) / float32(ShimmerLap)
	band = max(int(float32(width)*shimmerBand), shimmerMinBand)
	return int(-float32(band) + phase*float32(width+2*band)), band
}

// shimmerReach is how far, in pixels, the material's regions reach: past any
// text.
const shimmerReach = 1 << 15

// shimmerMaterial is the text material of a shimmering label of base color
// over size: the base color, and across it a band that is light at its
// centre, which moves with gtx.Now.
//
// Each pixel of the text is painted once. Whatever paints over a glyph's
// coverage adds that coverage again at its antialiased edges, so layers stacked
// on one another (the base, then the band over it) make the text heavier than
// a plain one. The band is instead two regions side by side, a ramp from the
// base up to the light on the left of its centre and one back down on the
// right, each clamped to its end colors beyond its ramp. The label paints the
// material once more after it, so the material ends on a transparent source,
// which paints nothing.
func (th *Theme) shimmerMaterial(gtx layout.Context, base color.NRGBA, size image.Point) op.CallOp {
	th.Loop(gtx)
	centre, band := shimmerAt(gtx.Now, size.X)
	light := MixOKLab(base, th.Foreground, shimmerLight)
	half := band / 2
	m := op.Record(gtx.Ops)
	region := func(from, to int, c1, c2 color.NRGBA, x1, x2 int) {
		defer clip.Rect{Min: image.Pt(from, -shimmerReach), Max: image.Pt(to, shimmerReach)}.Push(gtx.Ops).Pop()
		paint.LinearGradientOp{Stop1: f32.Pt(float32(x1), 0), Stop2: f32.Pt(float32(x2), 0), Color1: c1, Color2: c2}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}
	region(-shimmerReach, centre, base, light, centre-half, centre)
	region(centre, shimmerReach, light, base, centre, centre+half)
	paint.ColorOp{}.Add(gtx.Ops)
	return m.Stop()
}
