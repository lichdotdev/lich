package markdown

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/x/styledtext"

	"github.com/lichdotdev/lich/native/ui"
)

const markdownCodeBackgroundMix = 0.1

// codeFill is the ground of code, inline or in a block.
func codeFill(th *ui.Theme) color.NRGBA {
	return ui.MixOKLab(th.Background, th.Foreground, markdownCodeBackgroundMix)
}

// codePanelPad is DESIGN.md's padding of a code block.
var codePanelPad = ui.Space(2)

// layoutPanel draws the cell as a code block: across the body, on a fill of
// its own.
func (c *markdownCell) layoutPanel(gtx C, th *ui.Theme, open func(string)) D {
	pad, width := gtx.Dp(codePanelPad), gtx.Constraints.Max.X
	macro := op.Record(gtx.Ops)
	inner := gtx
	inner.Constraints.Max.X = max(width-2*pad, 0)
	stack := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
	d := c.layout(inner, th, open)
	stack.Pop()
	text := macro.Stop()
	size := image.Pt(width, d.Size.Y+2*pad)
	paint.FillShape(gtx.Ops, codeFill(th), clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusSM)).Op(gtx.Ops))
	text.Add(gtx.Ops)
	return D{Size: size}
}

// A blockquote as GitHub draws one: a .25em bar, then 1em before the text.
var (
	quoteBar   = ui.Space(1)
	quoteInset = ui.Space(4)
)

// layoutQuote draws the cell as a blockquote, behind a bar down its left.
func (c *markdownCell) layoutQuote(gtx C, th *ui.Theme, open func(string)) D {
	bar, lead := gtx.Dp(quoteBar), gtx.Dp(quoteBar+quoteInset)
	inner := gtx
	inner.Constraints.Max.X = max(gtx.Constraints.Max.X-lead, 0)
	stack := op.Offset(image.Pt(lead, 0)).Push(gtx.Ops)
	d := c.layout(inner, th, open)
	stack.Pop()
	paint.FillShape(gtx.Ops, th.Border, clip.Rect{Max: image.Pt(bar, d.Size.Y)}.Op())
	return D{Size: image.Pt(lead+d.Size.X, d.Size.Y)}
}

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
	style.LineHeightScale = c.lineHeightScale()
	style.Layout(gtx, func(gtx layout.Context, i int, dims layout.Dimensions) {
		if !c.code[i] || dims.Size.X == 0 || dims.Size.Y == 0 {
			return
		}
		rect := image.Rectangle{Max: dims.Size}.Inset(gtx.Dp(1))
		paint.FillShape(gtx.Ops, codeFill(th), clip.UniformRRect(rect, gtx.Dp(ui.RadiusSM)).Op(gtx.Ops))
	})
}
