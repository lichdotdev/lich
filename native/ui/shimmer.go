package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
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

// shimmerMaterial is the text material of a shimmering label of base color
// over size: the base color, and across it a band that is light at its
// centre, which moves with gtx.Now.
func (th *Theme) shimmerMaterial(gtx layout.Context, base color.NRGBA, size image.Point) op.CallOp {
	th.Loop(gtx)
	centre, band := shimmerAt(gtx.Now, size.X)
	light := MixOKLab(base, th.Foreground, shimmerLight)
	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: base}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	// The light ramps in up to the centre and stays; the base ramps back in
	// from the centre and stays. Each ramp is clamped by its own end colors, so
	// the band needs no clip of its own to stay inside the text.
	half := band / 2
	ramp := func(from, to int, c color.NRGBA) {
		paint.LinearGradientOp{Stop1: f32.Pt(float32(from), 0), Stop2: f32.Pt(float32(to), 0), Color1: Alpha(c, 0), Color2: c}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}
	ramp(centre-half, centre, light)
	ramp(centre, centre+half, base)
	return m.Stop()
}
