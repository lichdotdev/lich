package ui

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const (
	ringDash = unit.Dp(5)
	// ringMarch is how long the dashes of a MarchingRing take to move one dash
	// and gap along.
	ringMarch = 600 * time.Millisecond
	// ringMarchFrame paces the march at 20 frames a second: every frame of it
	// redraws the whole window, which at the display's rate cost 30 to 60% of
	// a core (measured 2026-10-27).
	ringMarchFrame = 50 * time.Millisecond
)

// FillRRect veils ring's inside in c, its corners rounded to radius.
func FillRRect(gtx C, ring image.Rectangle, radius unit.Dp, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(ring, gtx.Dp(radius)).Op(gtx.Ops))
}

// DashedRing draws ring's dashes in c, over a halo so they read on a dark
// ground too.
func DashedRing(gtx C, ring image.Rectangle, radius, width unit.Dp, c, halo color.NRGBA) {
	dashedRing(gtx, ring, radius, width, c, halo, 0)
}

// MarchingRing is a DashedRing whose dashes move with the clock.
func MarchingRing(gtx C, ring image.Rectangle, radius, width unit.Dp, c, halo color.NRGBA) {
	dash := float32(gtx.Dp(ringDash))
	cycle := gtx.Now.UnixNano() % int64(ringMarch)
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(ringMarchFrame)})
	dashedRing(gtx, ring, radius, width, c, halo, 2*dash*float32(cycle)/float32(ringMarch))
}

func dashedRing(gtx C, ring image.Rectangle, radius, width unit.Dp, c, halo color.NRGBA, phase float32) {
	dash := float32(gtx.Dp(ringDash))
	rr := roundedRect{min: f32.Pt(float32(ring.Min.X), float32(ring.Min.Y)), max: f32.Pt(float32(ring.Max.X), float32(ring.Max.Y)), radius: float32(gtx.Dp(radius))}
	w := float32(gtx.Dp(width))
	// One stroke a dash: Gio's stroker bridges the subpaths of one path, which
	// drew lines across the ring from dash to dash.
	rr.eachDash(dash, phase, func(from, to float32) {
		paint.FillShape(gtx.Ops, halo, clip.Stroke{Path: rr.span(gtx.Ops, from, to), Width: w + float32(gtx.Dp(1))}.Op())
	})
	rr.eachDash(dash, phase, func(from, to float32) {
		paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.span(gtx.Ops, from, to), Width: w}.Op())
	})
}

// DashedCircle is a ring of the given outer radius in dashes of dash, centered
// on center in pixels, one stroke a dash (DashedRing says why).
func DashedCircle(gtx C, center f32.Point, outer, stroke, dash unit.Dp, c color.NRGBA) {
	w := float32(gtx.Dp(stroke))
	r := float32(gtx.Dp(outer)) - w/2
	ring := roundedRect{min: center.Sub(f32.Pt(r, r)), max: center.Add(f32.Pt(r, r)), radius: r}
	ring.eachDash(float32(gtx.Dp(dash)), 0, func(from, to float32) {
		paint.FillShape(gtx.Ops, c, clip.Stroke{Path: ring.span(gtx.Ops, from, to), Width: w}.Op())
	})
}

// roundedRect is a rectangle with round corners, walked clockwise from the
// end of its top left corner.
type roundedRect struct {
	min, max f32.Point
	radius   float32
}

func (r roundedRect) dx() float32 { return r.max.X - r.min.X }
func (r roundedRect) dy() float32 { return r.max.Y - r.min.Y }

func (r roundedRect) clamped() float32 {
	return max(min(r.radius, r.dx()/2, r.dy()/2), 0)
}

// perimeter is the length of its outline.
func (r roundedRect) perimeter() float32 {
	rad := r.clamped()
	return 2*(r.dx()-2*rad) + 2*(r.dy()-2*rad) + 2*math.Pi*rad
}

// at is the point t along the outline, wrapping past its perimeter.
func (r roundedRect) at(t float32) f32.Point {
	rad := r.clamped()
	w, h := r.dx()-2*rad, r.dy()-2*rad
	arc := math.Pi / 2 * rad
	t = float32(math.Mod(float64(t), float64(r.perimeter())))
	if t < 0 {
		t += r.perimeter()
	}
	x0, y0, x1, y1 := r.min.X, r.min.Y, r.max.X, r.max.Y
	corner := func(cx, cy, from float32, s float32) f32.Point {
		a := float64(from) + float64(s/max(rad, 1e-3))
		return f32.Pt(cx+rad*float32(math.Cos(a)), cy+rad*float32(math.Sin(a)))
	}
	switch {
	case t < w:
		return f32.Pt(x0+rad+t, y0)
	case t < w+arc:
		return corner(x1-rad, y0+rad, -math.Pi/2, t-w)
	case t < w+arc+h:
		return f32.Pt(x1, y0+rad+t-w-arc)
	case t < w+2*arc+h:
		return corner(x1-rad, y1-rad, 0, t-w-arc-h)
	case t < 2*w+2*arc+h:
		return f32.Pt(x1-rad-(t-w-2*arc-h), y1)
	case t < 2*w+3*arc+h:
		return corner(x0+rad, y1-rad, math.Pi/2, t-2*w-2*arc-h)
	case t < 2*w+3*arc+2*h:
		return f32.Pt(x0, y1-rad-(t-2*w-3*arc-h))
	}
	return corner(x0+rad, y0+rad, math.Pi, t-2*w-3*arc-2*h)
}

// eachDash calls fn with the start and end of each dash of length dash, with
// gaps as long, shifted along the outline by phase.
func (r roundedRect) eachDash(dash, phase float32, fn func(from, to float32)) {
	total := r.perimeter()
	if total <= 0 || dash <= 0 {
		return
	}
	for start := phase - 2*dash; start < total; start += 2 * dash {
		if from, to := max(start, 0), min(start+dash, total); to > from {
			fn(from, to)
		}
	}
}

// span is the outline from from to to, as a path to stroke.
func (r roundedRect) span(ops *op.Ops, from, to float32) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(r.at(from))
	for s := from + 2; s < to; s += 2 {
		p.LineTo(r.at(s))
	}
	p.LineTo(r.at(to))
	return p.End()
}
