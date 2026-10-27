package main

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

// A pick's outline is a dashed ring a few dp outside the element, so it never
// merges with the element's own border, its dashes marching while the pointer
// is on the element or its note is in focus.

var (
	noteRingGap    = unit.Dp(5)
	noteRingRadius = unit.Dp(8)
	noteDash       = unit.Dp(5)
	// noteMarch is how long the dashes take to move one dash and gap along.
	noteMarch = 600 * time.Millisecond
	// noteMarchFrame paces the march at 20 frames a second: every frame of it
	// redraws the whole window, which at the display's rate cost 30 to 60% of
	// a core (measured 2026-10-27).
	noteMarchFrame = 50 * time.Millisecond
)

// noteRing is the ring around the element at r.
func noteRing(gtx C, r image.Rectangle) image.Rectangle {
	return r.Inset(-gtx.Dp(noteRingGap))
}

// ringFill veils the ring's inside.
func ringFill(gtx C, ring image.Rectangle, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(ring, gtx.Dp(noteRingRadius)).Op(gtx.Ops))
}

// dashedRing draws ring's dashes in c, over a white halo so they read on a
// dark page too; marching moves them with the clock.
func dashedRing(gtx C, ring image.Rectangle, c color.NRGBA, width unit.Dp, marching bool) {
	dash := float32(gtx.Dp(noteDash))
	phase := float32(0)
	if marching {
		cycle := gtx.Now.UnixNano() % int64(noteMarch)
		phase = 2 * dash * float32(cycle) / float32(noteMarch)
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(noteMarchFrame)})
	}
	rr := roundedRect{min: f32.Pt(float32(ring.Min.X), float32(ring.Min.Y)), max: f32.Pt(float32(ring.Max.X), float32(ring.Max.Y)), radius: float32(gtx.Dp(noteRingRadius))}
	w := float32(gtx.Dp(width))
	// One stroke a dash: Gio's stroker bridges the subpaths of one path, which
	// drew lines across the ring from dash to dash.
	rr.eachDash(dash, phase, func(from, to float32) {
		paint.FillShape(gtx.Ops, noteHalo, clip.Stroke{Path: rr.span(gtx.Ops, from, to), Width: w + float32(gtx.Dp(1))}.Op())
	})
	rr.eachDash(dash, phase, func(from, to float32) {
		paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.span(gtx.Ops, from, to), Width: w}.Op())
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
