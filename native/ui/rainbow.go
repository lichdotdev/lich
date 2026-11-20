package ui

import (
	"image"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// rainbowHues is GraphLanes in hue order, red to magenta: the lanes are
// ordered for telling branches apart, not as a spectrum.
var rainbowHues = [...]int{5, 2, 1, 4, 0, 3}

// Rainbow draws the label in the rainbow across its width: the mark of the
// mode that runs without asking.
func (l LabelStyle) Rainbow() LabelStyle { l.rainbow = true; return l }

// RainbowIcon draws ic filled, in the rainbow across its width.
func (th *Theme) RainbowIcon(gtx C, ic *icons.Icon, size unit.Dp) D {
	px := gtx.Dp(size)
	return ic.LayoutPainted(gtx, size, func(gtx C) { th.paintRainbow(gtx, px) })
}

// rainbowMaterial is the text material of a rainbow label width wide. It ends
// on a transparent source, as shimmerMaterial does, for the label paints the
// material once more after it.
func (th *Theme) rainbowMaterial(gtx C, width int) op.CallOp {
	m := op.Record(gtx.Ops)
	th.paintRainbow(gtx, width)
	paint.ColorOp{}.Add(gtx.Ops)
	return m.Stop()
}

// paintRainbow paints the current clip with the hues spread over [0, width],
// each pair of neighbours a gradient region side by side with the next, so
// every pixel is painted once; the first and last regions reach past the ends.
func (th *Theme) paintRainbow(gtx C, width int) {
	n := len(rainbowHues) - 1
	for i := range n {
		from, to := i*width/n, (i+1)*width/n
		x1, x2 := from, to
		if i == 0 {
			from = -shimmerReach
		}
		if i == n-1 {
			to = shimmerReach
		}
		c1, c2 := th.GraphLanes[rainbowHues[i]], th.GraphLanes[rainbowHues[i+1]]
		r := clip.Rect{Min: image.Pt(from, -shimmerReach), Max: image.Pt(to, shimmerReach)}.Push(gtx.Ops)
		paint.LinearGradientOp{Stop1: f32.Pt(float32(x1), 0), Stop2: f32.Pt(float32(x2), 0), Color1: c1, Color2: c2}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		r.Pop()
	}
}
