package main

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

const (
	markdownBodySize        = unit.Sp(15)
	markdownH1              = unit.Sp(20)
	markdownH2              = unit.Sp(19)
	markdownLineHeightScale = 1.5
	// markdownHeadingLineHeightScale is GitHub's line-height for headings,
	// which keeps an h1 or h2's rule close to its text.
	markdownHeadingLineHeightScale = 1.25
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
}

// markdownCell is rendered Markdown and the link state it is laid out with.
type markdownCell struct {
	spans []richtext.SpanStyle
	code  []bool
	state richtext.InteractiveText
	// lineHeight is the line height scale, markdownLineHeightScale when 0.
	lineHeight float32
}

// markdownCache keeps each body's markdownText by a key of the caller's, so
// a frame does not parse the same text again and a link's hover survives.
type markdownCache map[string]*markdownText

// markdownBlockGap is the room between blocks.
const markdownBlockGap = 4

// markdownWindow is the stretch of a body, in its own pixels, a frame can
// show. A block outside it is not laid out and keeps the height it last had.
type markdownWindow struct{ top, bottom int }

var markdownWhole = markdownWindow{top: math.MinInt, bottom: math.MaxInt}

// viewportWindow is the window of a body whose top sits top pixels above
// the viewport's, a viewport of room either side so a scroll lands on drawn
// blocks.
func viewportWindow(top, viewport int) markdownWindow {
	return markdownWindow{top: top - viewport, bottom: top + 2*viewport}
}

// layout draws src as Markdown; a link clicked lands in open.
func (m markdownCache) layout(gtx C, th *ui.Theme, key, src string, open func(url string)) D {
	return m.layoutWindow(gtx, th, key, src, open, markdownWhole)
}

// layoutWindow is layout drawing only the blocks within window.
func (m markdownCache) layoutWindow(gtx C, th *ui.Theme, key, src string, open func(url string), window markdownWindow) D {
	t := m.text(th, key, src)
	gap, estimate := gtx.Dp(ui.Space(markdownBlockGap)), t.estimate(gtx)
	y, width := 0, gtx.Constraints.Min.X
	for i, b := range t.blocks {
		if i > 0 {
			y += gap
		}
		h := t.heights[i]
		if h == 0 {
			h = estimate
		}
		if y+h < window.top || y > window.bottom {
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
func (m markdownCache) text(th *ui.Theme, key, src string) *markdownText {
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
		return 2 * gtx.Sp(markdownBodySize)
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
	return markdownLineHeightScale
}
