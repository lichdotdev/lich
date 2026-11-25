package ui

import (
	"image"
	"math"

	"gioui.org/f32"
)

// Cover is where a floating panel (a menu, a tooltip, a dialog, a toast)
// landed over the window, in the window's pixels: Box is its surface, Radius
// the rounding of its corners and Ring how far its ring or border reaches
// past Box.
type Cover struct {
	Box    image.Rectangle
	Radius int
	Ring   int
}

// Rects is the cover as rectangles, its ring included and its corners
// rounded (RoundedRows): a native window over the window's content is shaped
// by rectangles, and one cut by Box alone showed the window's background in
// the panel's corners and hid its ring.
func (c Cover) Rects() []image.Rectangle {
	b := c.Box.Inset(-c.Ring)
	if b.Empty() {
		return nil
	}
	return RoundedRows(b, c.Radius+c.Ring, c.Radius+c.Ring)
}

// RoundedRows is r with its top corners rounded by top and its bottom ones
// by bottom, as rows of pixels: a pixel the curve passes through is in, so
// the rows take all a rounded shape drawn there touches.
func RoundedRows(r image.Rectangle, top, bottom int) []image.Rectangle {
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

// RoundedCorners is what rounding r's corners by radius takes off it: in
// each corner, the pixels wholly outside the curve.
func RoundedCorners(r image.Rectangle, radius int) []image.Rectangle {
	radius = min(radius, r.Dy()/2, r.Dx()/2)
	var rows []image.Rectangle
	for i := range radius {
		in := cornerInset(radius, i)
		if in == 0 {
			continue
		}
		for _, y := range []int{r.Min.Y + i, r.Max.Y - i - 1} {
			rows = append(rows, image.Rect(r.Min.X, y, r.Min.X+in, y+1), image.Rect(r.Max.X-in, y, r.Max.X, y+1))
		}
	}
	return rows
}

// cornerInset is how many pixels of row i, counted from the edge a corner of
// radius rounds, lie wholly outside its curve: the row is as wide as the
// curve at its edge nearer the middle.
func cornerInset(radius, i int) int {
	dy := float64(radius - i - 1)
	return int(float64(radius) - math.Sqrt(float64(radius*radius)-dy*dy))
}

// coverOf is the cover of a panel of size drawn at at under a, a scale and
// an offset such as a panel's entrance, its rounding scaled with it.
func coverOf(a f32.Affine2D, at, size image.Point, radius, ring int) Cover {
	sx, _, _, _, _, _ := a.Elems()
	return Cover{Box: boundsOf(a, size).Add(at), Radius: int(sx * float32(radius)), Ring: ring}
}
