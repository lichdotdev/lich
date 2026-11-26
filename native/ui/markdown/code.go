package markdown

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"

	"github.com/lichdotdev/lich/native/ui"
)

const markdownCodeBackgroundMix = 0.1

// codeFill is the ground of code, inline or in a block.
func codeFill(th *ui.Theme) color.NRGBA {
	return ui.MixOKLab(th.Background, th.Foreground, markdownCodeBackgroundMix)
}

// codePanelPad is DESIGN.md's padding of a code block.
var codePanelPad = ui.Space(2)

// codeShown is the lines a code block shows before its "Show N more lines",
// as an edit's diff in the chat does.
const codeShown = 20

// layoutPanel draws the block's cell as a code block: across the body, on a
// fill of its own, with room at its right for the copy button, which stands
// on the first line. A block past codeShown lines shows those until its fold
// is opened, the copy button still copying all of it.
func (b markdownBlock) layoutPanel(gtx C, th *ui.Theme, l Links) D {
	c, state := b.prose, b.copy
	if state.more.Clicked(gtx) {
		state.open = !state.open
	}
	pad, width, side := gtx.Dp(codePanelPad), gtx.Constraints.Max.X, gtx.Dp(codeCopySide)
	fill := codeFill(th)
	macro := op.Record(gtx.Ops)
	inner := gtx
	inner.Constraints.Max.X = max(width-3*pad-side, 0)
	stack := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
	d := c.layout(inner, th.On(fill), l)
	stack.Pop()
	text := macro.Stop()
	height, hidden := d.Size.Y, strings.Count(b.code, "\n")+1-codeShown
	var tops []int
	if b.diff != nil || hidden > 0 {
		tops = c.lineTops(inner, th.Shaper, d.Size.Y)
	}
	ink := pad + d.Size.Y + pad
	if hidden > 0 && !state.open && codeShown < len(tops) {
		height, ink = tops[codeShown], pad+c.glyphsEnd(inner, th.Shaper, codeShown)
	}
	size := image.Pt(width, height+2*pad)
	panel := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusSM)).Push(gtx.Ops)
	paint.ColorOp{Color: fill}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	b.paintDiffRows(gtx, th, fill, tops, pad, width)
	// a folded block's padding shows none of the line under its last
	shown := clip.Rect{Max: image.Pt(width, ink)}.Push(gtx.Ops)
	text.Add(gtx.Ops)
	shown.Pop()
	panel.Pop()
	line := int(float32(gtx.Sp(BodySize)) * markdownCodeLineHeightScale)
	state.layout(gtx, th, b.code, size, image.Pt(width-pad-side, pad+(line-side)/2))
	if hidden <= 0 {
		return D{Size: size}
	}
	off := op.Offset(image.Pt(0, size.Y)).Push(gtx.Ops)
	more := layoutCodeMore(gtx, th, state, hidden)
	off.Pop()
	return D{Size: image.Pt(width, size.Y+more.Size.Y)}
}

// paintDiffRows tints the added and removed lines of a diff block across the
// panel, each as tall as it wraps to.
func (b markdownBlock) paintDiffRows(gtx C, th *ui.Theme, fill color.NRGBA, tops []int, pad, width int) {
	for i, tone := range b.diff {
		if i+1 >= len(tops) || (tone != diffAdded && tone != diffDeleted) {
			continue
		}
		row := image.Rect(0, pad+tops[i], width, pad+tops[i+1])
		paint.FillShape(gtx.Ops, ui.MixOKLab(fill, tone.color(th, fill), diffRowMix), clip.Rect(row).Op())
	}
}

// layoutCodeMore opens a long code block's lines past codeShown, or shuts
// them again.
func layoutCodeMore(gtx C, th *ui.Theme, state *codePanel, hidden int) D {
	label := Words.More(hidden)
	if state.open {
		label = Words.Less()
	}
	return layout.Inset{Top: ui.Space(1), Left: codePanelPad}.Layout(gtx, func(gtx C) D {
		return state.more.Layout(gtx, func(gtx C) D {
			fg := th.MutedForeground
			if state.more.Hovered() {
				fg = th.Foreground
			}
			return th.Text(ui.TextXS, label).In(fg).Layout(gtx)
		})
	})
}

// glyphsEnd is how far down the glyphs of the cell's first lines reach:
// past the top of the next line, which a glyph's descent can.
func (c *markdownCell) glyphsEnd(gtx C, shaper *text.Shaper, lines int) int {
	boxes := c.geometry(gtx, shaper).boxes
	end, i, line := 0, 0, 0
	for _, s := range c.spans {
		for _, r := range s.Content {
			if line == lines || i >= len(boxes) {
				return end
			}
			end = max(end, boxes[i].top+boxes[i].height)
			if r == '\n' {
				line++
			}
			i++
		}
	}
	return end
}

// lineTops is the top of each line of the cell's text, a line wrapped on
// screen counting once, then height, where the last one ends.
func (c *markdownCell) lineTops(gtx C, shaper *text.Shaper, height int) []int {
	boxes := c.geometry(gtx, shaper).boxes
	tops, i := []int{0}, 0
	for _, s := range c.spans {
		for _, r := range s.Content {
			if r == '\n' && i+1 < len(boxes) {
				tops = append(tops, boxes[i+1].top)
			}
			i++
		}
	}
	return append(tops, height)
}

// A blockquote as GitHub draws one: a .25em bar, then 1em before the text.
var (
	quoteBar   = ui.Space(1)
	quoteInset = ui.Space(4)
)

// layoutQuote draws body as a blockquote, behind a bar in bar down its left.
func layoutQuote(gtx C, th *ui.Theme, bar color.NRGBA, body layout.Widget) D {
	width, lead := gtx.Dp(quoteBar), gtx.Dp(quoteBar+quoteInset)
	inner := gtx
	inner.Constraints.Max.X = max(gtx.Constraints.Max.X-lead, 0)
	stack := op.Offset(image.Pt(lead, 0)).Push(gtx.Ops)
	d := body(inner)
	stack.Pop()
	paint.FillShape(gtx.Ops, bar, clip.Rect{Max: image.Pt(width, d.Size.Y)}.Op())
	return D{Size: image.Pt(lead+d.Size.X, d.Size.Y)}
}
