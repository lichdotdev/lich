package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/x/richtext"

	"github.com/lichdotdev/lich/native/ui"
)

// markdownH1 is mdProse's h1 text-lg.
const markdownH1 = unit.Sp(18)

// markdownText is one rendered Markdown body: its blocks, parsed once per
// text and theme, each keeping the link state it is laid out with.
type markdownText struct {
	src    string
	fg     [2]color.NRGBA
	blocks []markdownBlock
	// heights is each block's height as last laid out, 0 for one never laid
	// out.
	heights []int
}

// markdownBlock is one top-level node: prose, or a GFM table, which flat
// richtext has no layout for.
type markdownBlock struct {
	prose *markdownCell
	table *markdownTable
}

// markdownCell is rendered Markdown and the link state it is laid out with.
type markdownCell struct {
	spans []richtext.SpanStyle
	state richtext.InteractiveText
}

// markdownCache keeps each body's markdownText by a key of the caller's, so
// a frame does not parse the same text again and a link's hover survives.
type markdownCache map[string]*markdownText

// markdownBlockGap is the room between blocks, mdProse's my-3.
const markdownBlockGap = 3

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

// layout draws src as Markdown, mdProse's text-sm muted body with the links
// in the foreground (h1 text-lg, h2 text-base, the rest text-sm) and its
// blocks my-3 apart; a link clicked lands in open.
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

// text is the cache's entry for key, parsed again when src or the theme's
// colors changed.
func (m markdownCache) text(th *ui.Theme, key, src string) *markdownText {
	t, ok := m[key]
	if !ok {
		t = new(markdownText)
		m[key] = t
	}
	fg := [2]color.NRGBA{th.MutedForeground, th.Foreground}
	if t.src != src || t.fg != fg || t.blocks == nil {
		t.src, t.fg, t.blocks = src, fg, renderBlocks(th, src)
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
		return 2 * gtx.Sp(ui.TextSM)
	}
	return sum / measured
}

func (b markdownBlock) layout(gtx C, th *ui.Theme, open func(url string)) D {
	if b.table != nil {
		return b.table.layout(gtx, th, open)
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
	return richtext.Text(&c.state, th.Shaper, c.spans...).Layout(gtx)
}
