package ui

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const (
	statusDotBox    = 14
	statusDotRadius = 3.5
	markDotSize     = unit.Dp(6) // Space(1.5)
)

// The beat of a StatusDot: a ring grows out of it for pulseLength, drawn at
// LoopFrame, then rests until pulseEvery comes round. It beats for as long as
// what it marks lasts, hours, so the rest is what keeps it cheap: no frame is
// asked for in it. Every frame costs the window the same whatever it shows
// (about 0.3% of a laptop GPU, measured 2026-11-01), and a beat every 2.4s was
// 8 frames a second all day for a 14dp dot.
const (
	pulseEvery  = 6 * time.Second
	pulseLength = 800 * time.Millisecond
)

func statusDotGeometry(gtx C) (size int, center f32.Point, radius float32) {
	size = gtx.Dp(statusDotBox)
	return size, f32.Pt(float32(size)/2, float32(size)/2), float32(gtx.Dp(statusDotRadius))
}

// StatusDot is a solid 7dp dot in c, centered in a 14dp box.
func StatusDot(gtx C, c color.NRGBA) D {
	size, center, dot := statusDotGeometry(gtx)
	StrokeArc(gtx, center, dot/2, dot, 0, 2*math.Pi, c)
	return D{Size: image.Pt(size, size)}
}

// HollowDot is StatusDot's outline: a 1.5dp ring of the same size.
func HollowDot(gtx C, c color.NRGBA) D {
	size, center, dot := statusDotGeometry(gtx)
	StrokeArc(gtx, center, dot-float32(gtx.Dp(0.75)), float32(gtx.Dp(1.5)), 0, 2*math.Pi, c)
	return D{Size: image.Pt(size, size)}
}

// BeatingDot is StatusDot with its beat.
func (th *Theme) BeatingDot(gtx C, c color.NRGBA) D {
	size, center, dot := statusDotGeometry(gtx)
	th.PulseRing(gtx, center, dot, dot, c)
	StrokeArc(gtx, center, dot/2, dot, 0, 2*math.Pi, c)
	return D{Size: image.Pt(size, size)}
}

// PulseRing is that beat around center: a ring from radius to radius+reach
// that fades as it grows, and the next frame it needs.
func (th *Theme) PulseRing(gtx C, center f32.Point, radius, reach float32, c color.NRGBA) {
	at := time.Duration(gtx.Now.UnixMilli()%pulseEvery.Milliseconds()) * time.Millisecond
	if at >= pulseLength {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(pulseEvery - at)})
		return
	}
	phase := float32(at) / float32(pulseLength)
	eased := 1 - (1-phase)*(1-phase)
	StrokeArc(gtx, center, radius+reach*eased, float32(gtx.Dp(1.5)), 0, 2*math.Pi, Alpha(c, 0.7*(1-phase)))
	th.Loop(gtx)
}

// MarkDot is the 6dp dot beside a title that marks something it holds.
func MarkDot(gtx C, c color.NRGBA) D {
	size := gtx.Dp(markDotSize)
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(size, size)}.Op(gtx.Ops))
	return D{Size: image.Pt(size, size)}
}

// Dot is a solid circle of the given diameter, drawn as a rounded fill.
func Dot(gtx C, c color.NRGBA, size unit.Dp) D {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	return Fill(gtx, c, size/2)
}

// DotAt is a solid circle of radius r at a point: a stroke's round cap.
func DotAt(gtx C, at f32.Point, r float32, c color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(at.Add(f32.Pt(r, 0)))
	p.ArcTo(at, at, 2*math.Pi)
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
}

// Swatch is the 10dp rounded square of a color.
func Swatch(gtx C, c color.NRGBA) D {
	size := image.Pt(gtx.Dp(10), gtx.Dp(10))
	gtx.Constraints = layout.Exact(size)
	return Fill(gtx, c, unit.Dp(3))
}

// NotifyDot is the red dot on a button's top right corner, ringed in the
// surface it sits on.
func (th *Theme) NotifyDot(gtx C, button image.Point) {
	inset, dot, ring := gtx.Dp(5), gtx.Dp(8), gtx.Dp(2)
	center := f32.Pt(float32(button.X-inset-dot/2), float32(inset+dot/2))
	circle := func(radius int, c color.NRGBA) {
		r := float32(radius)
		box := image.Rect(int(center.X-r), int(center.Y-r), int(center.X+r), int(center.Y+r))
		paint.FillShape(gtx.Ops, c, clip.Ellipse(box).Op(gtx.Ops))
	}
	circle(dot/2+ring, th.Surface)
	circle(dot/2, th.Destructive)
}

// CornerDot is the 6dp dot on the top right corner of the minimum box, in c
// and ringed in the surface it sits on, so it reads off the glyph under it.
func (th *Theme) CornerDot(gtx C, c color.NRGBA) D {
	size := gtx.Constraints.Min
	r := float32(gtx.Dp(3))
	center := f32.Pt(float32(size.X)-r, r)
	StrokeArc(gtx, center, r+float32(gtx.Dp(1)), float32(gtx.Dp(2)), 0, 2*math.Pi, th.Surface)
	StrokeArc(gtx, center, r/2, r, 0, 2*math.Pi, c)
	return D{Size: size}
}

// PulsingCornerDot is CornerDot with the beat around it, for something live.
func (th *Theme) PulsingCornerDot(gtx C, c color.NRGBA) D {
	r := float32(gtx.Dp(3))
	th.PulseRing(gtx, f32.Pt(float32(gtx.Constraints.Min.X)-r, r), r, r, c)
	return th.CornerDot(gtx, c)
}
