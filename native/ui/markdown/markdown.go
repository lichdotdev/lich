// Package markdown draws Markdown text: GFM tables and task lists, links handed
// back to the caller, and only the blocks a viewport can show.
package markdown

import (
	"image"
	"image/color"
	"math"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/x/richtext"

	"github.com/lichdotdev/lich/native/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const (
	BodySize        = unit.Sp(15)
	markdownH1      = unit.Sp(20)
	markdownH2      = unit.Sp(18)
	LineHeightScale = 1.5
	// markdownHeadingLineHeightScale is GitHub's line-height for headings,
	// which keeps an h1 or h2's rule close to its text.
	markdownHeadingLineHeightScale = 1.25
	// markdownCodeLineHeightScale sets a code block as the code view sets
	// source, on the mono face's own line, about 1.3 of its size, not prose's.
	markdownCodeLineHeightScale = 1.3
)

func markdownProseColor(th *ui.Theme) color.NRGBA {
	return ui.MixOKLab(th.Foreground, th.MutedForeground, 0.35)
}

// markdownText is one rendered Markdown body: its blocks, parsed once per
// text and theme, each keeping the link state it is laid out with.
type markdownText struct {
	src    string
	fg     [3]color.NRGBA
	fonts  [2]font.Typeface
	shaper *text.Shaper
	blocks []markdownBlock
	// heights is each block's height as last laid out, 0 for one never laid
	// out.
	heights []int
}

// markdownBlock is prose, a GFM table, or a list with individually laid-out items.
type markdownBlock struct {
	prose *markdownCell
	table *markdownTable
	list  *markdownList
	// rule underlines the prose across the body's width, GitHub's border
	// under an h1 or h2.
	rule bool
	// quote draws the prose behind a bar, a blockquote; panel draws it on a
	// fill of its own, a code block.
	quote, panel bool
	// blocks is what a quote holds when that is more than paragraphs, each
	// drawn as it is outside one; the quote then has no prose.
	blocks []markdownBlock
}

// markdownCell is rendered Markdown and the link state it is laid out with.
type markdownCell struct {
	spans []richtext.SpanStyle
	code  []bool
	state richtext.InteractiveText
	// lineHeight is the line height scale, LineHeightScale when 0.
	lineHeight float32
}

// Cache keeps each body's markdownText by a key of the caller's, so
// a frame does not parse the same text again and a link's hover survives.
type Cache map[string]*markdownText

// BlockGap is the room between blocks.
const BlockGap = 4

// Window is the stretch of a body, in its own pixels, a frame can
// show. A block outside it is not laid out and keeps the height it last had.
type Window struct{ Top, Bottom int }

var whole = Window{Top: math.MinInt, Bottom: math.MaxInt}

// ViewportWindow is the window of a body whose top sits top pixels above
// the viewport's, a viewport of room either side so a scroll lands on drawn
// blocks.
func ViewportWindow(top, viewport int) Window {
	return Window{Top: top - viewport, Bottom: top + 2*viewport}
}

// layout draws src as Markdown; a link clicked lands in open.
func (m Cache) Layout(gtx C, th *ui.Theme, key, src string, open func(url string)) D {
	return m.LayoutWindow(gtx, th, key, src, open, whole)
}

// LayoutWindow is layout drawing only the blocks within window.
func (m Cache) LayoutWindow(gtx C, th *ui.Theme, key, src string, open func(url string), window Window) D {
	t := m.text(th, key, src)
	gap, estimate := gtx.Dp(ui.Space(BlockGap)), t.estimate(gtx)
	y, width := 0, gtx.Constraints.Min.X
	for i, b := range t.blocks {
		if i > 0 {
			y += gap
		}
		h := t.heights[i]
		if h == 0 {
			h = estimate
		}
		if y+h < window.Top || y > window.Bottom {
			y += h
			continue
		}
		g := gtx
		g.Constraints.Min = image.Point{}
		stack := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		dims := b.layout(g, th, open)
		stack.Pop()
		t.heights[i] = dims.Size.Y
		y += dims.Size.Y
		width = max(width, dims.Size.X)
	}
	return D{Size: image.Pt(width, y)}
}

// text is the cache's entry for key, parsed again when src or the theme changes.
func (m Cache) text(th *ui.Theme, key, src string) *markdownText {
	t, ok := m[key]
	if !ok {
		t = new(markdownText)
		m[key] = t
	}
	fg := [3]color.NRGBA{markdownProseColor(th), th.Foreground, th.TonePass}
	fonts := [2]font.Typeface{th.Sans, th.Mono}
	if t.src != src || t.fg != fg || t.fonts != fonts || t.shaper != th.Shaper || t.blocks == nil {
		t.src, t.fg, t.fonts, t.shaper, t.blocks = src, fg, fonts, th.Shaper, renderBlocks(th, src)
		t.heights = make([]int, len(t.blocks))
	}
	return t
}

// estimate is the height a block not laid out yet is given: the average of
// those that were, or two lines of text.
func (t *markdownText) estimate(gtx C) int {
	sum, measured := 0, 0
	for _, h := range t.heights {
		if h > 0 {
			sum, measured = sum+h, measured+1
		}
	}
	if measured == 0 {
		return 2 * gtx.Sp(BodySize)
	}
	return sum / measured
}

func (b markdownBlock) layout(gtx C, th *ui.Theme, open func(url string)) D {
	if b.table != nil {
		return b.table.layout(gtx, th, open)
	}
	if b.list != nil {
		return b.list.layout(gtx, th, open)
	}
	if b.quote {
		return layoutQuote(gtx, th, func(gtx C) D {
			if b.blocks != nil {
				return layoutBlocks(gtx, th, open, b.blocks, BlockGap)
			}
			return b.prose.layout(gtx, th, open)
		})
	}
	if b.panel {
		return b.prose.layoutPanel(gtx, th, open)
	}
	if b.rule {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return b.prose.layout(gtx, th, open) }),
			ui.VGap(1),
			layout.Rigid(th.Separator),
		)
	}
	return b.prose.layout(gtx, th, open)
}

func (c *markdownCell) layout(gtx C, th *ui.Theme, open func(url string)) D {
	for {
		span, ev, ok := c.state.Update(gtx)
		if !ok {
			break
		}
		if url, isLink := span.Get(markdownURL).(string); isLink && url != "" && ev.Type == richtext.Click && open != nil {
			open(url)
		}
	}
	style := richtext.Text(&c.state, th.Shaper, c.spans...)
	style.LineHeightScale = c.lineHeightScale()
	c.codeBackgrounds(gtx, th)
	return style.Layout(gtx)
}

func (c *markdownCell) lineHeightScale() float32 {
	if c.lineHeight != 0 {
		return c.lineHeight
	}
	return LineHeightScale
}
