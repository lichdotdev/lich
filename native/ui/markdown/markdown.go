// Package markdown draws Markdown text: GFM tables and task lists, links handed
// back to the caller, and only the blocks a viewport can show.
package markdown

import (
	"image"
	"image/color"
	"math"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/x/richtext"
	"github.com/lichdotdev/godemirror"

	"github.com/lichdotdev/lich/native/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const (
	BodySize        = unit.Sp(15)
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
	look   look
	// heights is each block's height as last laid out, 0 for one never laid
	// out.
	heights []int
	// linked is the Paths.Version the path marks were checked against.
	linked any
	// highlight is the style the code blocks were highlighted with.
	highlight *godemirror.HighlightStyle
	// copies is each code block's copy button, in the order they are written.
	copies []*codePanel
	// cells are the blocks' cells a selection can be made in, in the order
	// they are written, and sels the selection of each.
	sels  []*textSelection
	cells []*markdownCell
	group selectGroup
}

// markdownBlock is prose, a GFM table, or a list with individually laid-out items.
type markdownBlock struct {
	prose *markdownCell
	table *markdownTable
	list  *markdownList
	// heading is a heading's prose, which sits closer to what it opens than
	// to what is above it.
	heading bool
	// props is a document's frontmatter, drawn in place of prose.
	props []property
	// rule is a thematic break, drawn as a line across the body.
	rule bool
	// quote draws the prose behind a bar, a blockquote; panel draws it on a
	// fill of its own, a code block.
	quote, panel bool
	// alert is the kind of GitHub alert a quote is, noAlert for a quote.
	alert Alert
	// code is a panel's code as written, and copy its copy button.
	code string
	copy *codePanel
	// diff is the tone of each line of a diff block, nil for other code.
	diff []diffTone
	// blocks is what a quote holds when that is more than paragraphs, each
	// drawn as it is outside one; the quote then has no prose.
	blocks []markdownBlock
}

// markdownCell is rendered Markdown and the link state it is laid out with.
type markdownCell struct {
	spans []richtext.SpanStyle
	code  []bool
	// decor is what each span is drawn with besides its text: an underline,
	// a mark before it.
	decor []decoration
	// paths is the pointer on each span that spells a path, by span.
	paths []pathPointer
	state richtext.InteractiveText
	// lineHeight is the line height scale, LineHeightScale when 0.
	lineHeight float32
	// sel is the cell's selection, nil for text no selection is made in.
	sel *textSelection
	geo *cellGeometry
	// inRow is a table cell, read after the cell on its left: a pointer
	// beside it is before or after it, not on one of its lines.
	inRow bool
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
	return m.layout(gtx, th, key, src, Links{Open: open}, whole)
}

// LayoutWindow is layout drawing only the blocks within window.
func (m Cache) LayoutWindow(gtx C, th *ui.Theme, key, src string, open func(url string), window Window) D {
	return m.layout(gtx, th, key, src, Links{Open: open}, window)
}

// LayoutDocumentWindow is LayoutWindow for a file opened to read: a reading
// column, its frontmatter as properties, headings a size up.
func (m Cache) LayoutDocumentWindow(gtx C, th *ui.Theme, key, src string, open func(url string), window Window) D {
	return m.layoutAs(gtx, th, key, src, Links{Open: open}, window, documentLook)
}

// LayoutLinks is layout handing what src links to, paths included, to l.
func (m Cache) LayoutLinks(gtx C, th *ui.Theme, key, src string, l Links) D {
	return m.layout(gtx, th, key, src, l, whole)
}

// LayoutLinksWindow is LayoutLinks drawing only the blocks within window.
func (m Cache) LayoutLinksWindow(gtx C, th *ui.Theme, key, src string, l Links, window Window) D {
	return m.layout(gtx, th, key, src, l, window)
}

func (m Cache) layout(gtx C, th *ui.Theme, key, src string, l Links, window Window) D {
	return m.layoutAs(gtx, th, key, src, l, window, inlineLook)
}

func (m Cache) layoutAs(gtx C, th *ui.Theme, key, src string, l Links, window Window, lk look) D {
	t := m.text(th, key, src, l.Paths, lk)
	t.selectKeys(gtx)
	t.group.kept = false
	if l.quoteAt = t.quoteAt(l); l.quoteAt >= 0 {
		l.quoteText = t.selection()
	}
	estimate := t.estimate(gtx)
	y, width, left := 0, gtx.Constraints.Min.X, 0
	if column := gtx.Dp(lk.column); column > 0 && gtx.Constraints.Max.X > column {
		left, gtx.Constraints.Max.X = (gtx.Constraints.Max.X-column)/2, column
	}
	for i, b := range t.blocks {
		if i > 0 {
			y += gtx.Dp(ui.Space(lk.gapBefore(t.blocks, i)))
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
		stack := op.Offset(image.Pt(left, y)).Push(gtx.Ops)
		dims := b.layout(g, th, l)
		stack.Pop()
		t.heights[i] = dims.Size.Y
		y += dims.Size.Y
		width = max(width, left+dims.Size.X)
	}
	return D{Size: image.Pt(width, y)}
}

// text is the cache's entry for key, parsed again when src, the theme or
// the listing paths looks paths up in changes.
func (m Cache) text(th *ui.Theme, key, src string, paths *Paths, lk look) *markdownText {
	t, ok := m[key]
	if !ok {
		t = new(markdownText)
		m[key] = t
	}
	fg := [3]color.NRGBA{markdownProseColor(th), th.Foreground, th.TonePass}
	fonts := [2]font.Typeface{th.Sans, th.Mono}
	var linked any
	if paths != nil {
		linked = paths.Version
	}
	if t.src != src || t.fg != fg || t.fonts != fonts || t.shaper != th.Shaper || t.blocks == nil || t.linked != linked || t.highlight != th.Code.Highlight || t.look != lk {
		// A body streamed in only grows at its last block: the ones before
		// keep their heights, or the blocks out of the window fall back to the
		// estimate and the body shrinks and grows back frame after frame.
		kept := t.heights
		if t.fonts != fonts || t.shaper != th.Shaper || t.look != lk || !strings.HasPrefix(src, t.src) {
			kept = nil
		}
		t.src, t.fg, t.fonts, t.shaper, t.look, t.blocks = src, fg, fonts, th.Shaper, lk, renderLook(th, src, lk)
		t.heights, t.linked, t.highlight = make([]int, len(t.blocks)), linked, th.Code.Highlight
		copy(t.heights, kept[:max(len(kept)-1, 0)])
		t.keepCopies()
		if paths != nil {
			unmarkUnlinked(t.blocks, paths.Link)
		}
		t.keepSelections()
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

func (b markdownBlock) layout(gtx C, th *ui.Theme, l Links) D {
	if b.table != nil {
		return b.table.layout(gtx, th, l)
	}
	if b.list != nil {
		return b.list.layout(gtx, th, l)
	}
	if b.quote {
		return layoutQuote(gtx, th, b.alert.tone(th), func(gtx C) D {
			if b.alert != noAlert {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return layoutAlertHead(gtx, th, b.alert) }),
					ui.VGap(alertHeadGap),
					layout.Rigid(func(gtx C) D { return layoutBlocks(gtx, th, l, b.blocks, BlockGap) }))
			}
			if b.blocks != nil {
				return layoutBlocks(gtx, th, l, b.blocks, BlockGap)
			}
			return b.prose.layout(gtx, th, l)
		})
	}
	if b.panel {
		return b.layoutPanel(gtx, th, l)
	}
	if b.props != nil {
		return layoutProps(gtx, th, b.props)
	}
	if b.rule {
		return th.Separator(gtx)
	}
	return b.prose.layout(gtx, th, l)
}

func (c *markdownCell) layout(gtx C, th *ui.Theme, l Links) D {
	c.selectEvents(gtx, th)
	if c.sel != nil {
		c.sel.keep = image.Rectangle{}
	}
	for {
		span, ev, ok := c.state.Update(gtx)
		if !ok {
			break
		}
		if url, isLink := span.Get(markdownURL).(string); isLink && url != "" && ev.Type == richtext.Click && l.Open != nil && !c.dragged() {
			l.Open(url)
		}
	}
	style := richtext.Text(&c.state, th.Shaper, c.spans...)
	style.LineHeight, style.LineHeightScale = c.lineSize(), c.lineHeightScale()
	m := op.Record(gtx.Ops)
	c.decorate(gtx, th, l.Paths)
	c.paintSelection(gtx, th)
	dims := style.Layout(gtx)
	c.selectArea(gtx, dims.Size, m.Stop())
	c.quote(gtx, th, l, dims.Size.X)
	return dims
}

// dragged is whether the press the cell last took became a selection, which
// opens nothing.
func (c *markdownCell) dragged() bool { return c.sel != nil && c.sel.dragged }

// lineSize is what the cell's line height scales: its largest span's size.
// styledtext otherwise scales each line's glyph height, ascent plus descent,
// which is already past the size and differs by font: a body line came to
// about 2.25 times its size, against the composer's 1.5.
func (c *markdownCell) lineSize() unit.Sp {
	var size unit.Sp
	for _, s := range c.spans {
		size = max(size, s.Size)
	}
	return size
}

func (c *markdownCell) lineHeightScale() float32 {
	if c.lineHeight != 0 {
		return c.lineHeight
	}
	return LineHeightScale
}
