package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const (
	dropTrayBorder = unit.Dp(1.5)
	// bezierQuarter is the control point distance that makes a cubic Bézier
	// follow a quarter circle.
	bezierQuarter = 0.5523
)

// Tray lays w on a rounded tray of fill, clipped to its corners.
func Tray(gtx C, fill color.NRGBA, w layout.Widget) D {
	rec := op.Record(gtx.Ops)
	d := w(gtx)
	content := rec.Stop()
	shape := clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(RadiusLG))
	paint.FillShape(gtx.Ops, fill, shape.Op(gtx.Ops))
	defer shape.Push(gtx.Ops).Pop()
	content.Add(gtx.Ops)
	return d
}

// DropTray is a Tray with a border in edge, for a block a carried item would
// go to if let go.
func DropTray(gtx C, fill, edge color.NRGBA, w layout.Widget) D {
	d := Tray(gtx, fill, w)
	rr := gtx.Dp(RadiusLG)
	width := float32(gtx.Dp(dropTrayBorder))
	inset := int(width / 2)
	edgeRect := image.Rectangle{Min: image.Pt(inset, inset), Max: d.Size.Sub(image.Pt(inset, inset))}
	paint.FillShape(gtx.Ops, edge, clip.Stroke{Path: clip.UniformRRect(edgeRect, rr-inset).Path(gtx.Ops), Width: width}.Op())
	return d
}

// FillIf is Fill unless c is transparent, a box at rest.
func FillIf(gtx C, c color.NRGBA, radius unit.Dp) D {
	if c.A == 0 {
		return D{Size: gtx.Constraints.Min}
	}
	return Fill(gtx, c, radius)
}

// DashedBorder strokes a 1px dashed line just inside a rounded box: dashes of
// dash along the straight edges, gap apart, the corners' arcs whole.
func DashedBorder(gtx C, size image.Point, r int, c color.NRGBA, dashLen, gapLen unit.Dp) {
	w := gtx.Dp(1)
	dash, gap := gtx.Dp(dashLen), gtx.Dp(gapLen)
	for _, corner := range []image.Rectangle{
		{Max: image.Pt(r, r)}, {Min: image.Pt(size.X-r, 0), Max: image.Pt(size.X, r)},
		{Min: image.Pt(0, size.Y-r), Max: image.Pt(r, size.Y)}, {Min: size.Sub(image.Pt(r, r)), Max: size},
	} {
		cl := clip.Rect(corner).Push(gtx.Ops)
		InsetRing(gtx, size, r, w, c)
		cl.Pop()
	}
	for x := r; x < size.X-r; x += dash + gap {
		end := min(x+dash, size.X-r)
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(x, 0), Max: image.Pt(end, w)}.Op())
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(x, size.Y-w), Max: image.Pt(end, size.Y)}.Op())
	}
	for y := r; y < size.Y-r; y += dash + gap {
		end := min(y+dash, size.Y-r)
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(0, y), Max: image.Pt(w, end)}.Op())
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(size.X-w, y), Max: image.Pt(size.X, end)}.Op())
	}
}

// Fillet fills the r×r box at p minus the quarter disc around its top corner
// on the side away from the tab: right is the fillet right of the tab.
func Fillet(gtx C, p image.Point, r int, right bool, c color.NRGBA) {
	defer op.Offset(p).Push(gtx.Ops).Pop()
	s := float32(r)
	var path clip.Path
	path.Begin(gtx.Ops)
	if right {
		path.MoveTo(f32.Pt(0, 0))
		path.CubeTo(f32.Pt(0, s*bezierQuarter), f32.Pt(s-s*bezierQuarter, s), f32.Pt(s, s))
		path.LineTo(f32.Pt(0, s))
	} else {
		path.MoveTo(f32.Pt(s, 0))
		path.CubeTo(f32.Pt(s, s*bezierQuarter), f32.Pt(s*bezierQuarter, s), f32.Pt(0, s))
		path.LineTo(f32.Pt(s, s))
	}
	path.Close()
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: path.End()}.Op())
}

// Checker is a card ground under muted squares of cell, so a transparent pixel
// reads as transparent rather than as the panel behind it.
func (th *Theme) Checker(gtx C, size image.Point, cell unit.Dp) {
	paint.FillShape(gtx.Ops, th.Card, clip.Rect{Max: size}.Op())
	px := gtx.Dp(cell)
	var p clip.Path
	p.Begin(gtx.Ops)
	for y := 0; y*px < size.Y; y++ {
		for x := y % 2; x*px < size.X; x += 2 {
			r := image.Rect(x*px, y*px, min(size.X, (x+1)*px), min(size.Y, (y+1)*px))
			p.MoveTo(layout.FPt(r.Min))
			p.LineTo(layout.FPt(image.Pt(r.Max.X, r.Min.Y)))
			p.LineTo(layout.FPt(r.Max))
			p.LineTo(layout.FPt(image.Pt(r.Min.X, r.Max.Y)))
			p.Close()
		}
	}
	paint.FillShape(gtx.Ops, th.Muted, clip.Outline{Path: p.End()}.Op())
}
