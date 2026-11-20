package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// InsetBorder strokes a 1px line just inside size, a box-shadow inset 0 0 0
// 1px.
func InsetBorder(gtx C, size image.Point, r int, c color.NRGBA) {
	InsetRing(gtx, size, r, gtx.Dp(1), c)
}

// InsetRing strokes a line width pixels wide just inside size, ring-inset:
// a stroke twice as wide on the edge, clipped to the inside half.
func InsetRing(gtx C, size image.Point, r, width int, c color.NRGBA) {
	rr := clip.UniformRRect(image.Rectangle{Max: size}, r)
	defer rr.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(2 * width)}.Op())
}

// IslandBorder strokes a 1px line just inside size, its corners of radius r.
func IslandBorder(gtx C, size image.Point, r int, c color.NRGBA) {
	w := float32(gtx.Dp(1))
	h := w / 2
	x0, y0, x1, y1 := h, h, float32(size.X)-h, float32(size.Y)-h
	fr := float32(r)
	q := float32(bezierQuarter)
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(x0+fr, y0))
	p.LineTo(f32.Pt(x1-fr, y0))
	p.CubeTo(f32.Pt(x1-fr+q*fr, y0), f32.Pt(x1, y0+fr-q*fr), f32.Pt(x1, y0+fr))
	p.LineTo(f32.Pt(x1, y1-fr))
	p.CubeTo(f32.Pt(x1, y1-fr+q*fr), f32.Pt(x1-fr+q*fr, y1), f32.Pt(x1-fr, y1))
	p.LineTo(f32.Pt(x0+fr, y1))
	p.CubeTo(f32.Pt(x0+fr-q*fr, y1), f32.Pt(x0, y1-fr+q*fr), f32.Pt(x0, y1-fr))
	p.LineTo(f32.Pt(x0, y0+fr))
	p.CubeTo(f32.Pt(x0, y0+fr-q*fr), f32.Pt(x0+fr-q*fr, y0), f32.Pt(x0+fr, y0))
	p.Close()
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: w}.Op())
}
