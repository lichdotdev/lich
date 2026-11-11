package ui

import (
	"image"

	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// VerticalRule is a 1px border-r seam the height of gtx.
func (th *Theme) VerticalRule(gtx C) D {
	size := image.Pt(gtx.Dp(1), gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: size}.Op())
	return D{Size: size}
}

// RuleAbove is border-t: a 1px line of border across gtx. It draws the same
// pixels as Separator.
func (th *Theme) RuleAbove(gtx C) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: size}.Op())
	return D{Size: size}
}
