package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/x/styledtext"

	"github.com/lichdotdev/lich/native/ui"
)

func (c *markdownCell) codeBackgrounds(gtx C, th *ui.Theme) {
	hasCode := false
	for _, code := range c.code {
		hasCode = hasCode || code
	}
	if !hasCode {
		return
	}
	spans := make([]styledtext.SpanStyle, len(c.spans))
	for i, span := range c.spans {
		spans[i] = styledtext.SpanStyle{Font: span.Font, Size: span.Size, Color: color.NRGBA{}, Content: span.Content}
	}
	style := styledtext.Text(th.Shaper, spans...)
	style.LineHeightScale = markdownLineHeightScale
	style.Layout(gtx, func(gtx layout.Context, i int, dims layout.Dimensions) {
		if !c.code[i] || dims.Size.X == 0 || dims.Size.Y == 0 {
			return
		}
		rect := image.Rectangle{Max: dims.Size}.Inset(gtx.Dp(1))
		paint.FillShape(gtx.Ops, ui.MixOKLab(th.Background, th.Foreground, 0.1), clip.UniformRRect(rect, gtx.Dp(3)).Op(gtx.Ops))
	})
}
