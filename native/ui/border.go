package ui

import (
	"image"
	"image/color"

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
