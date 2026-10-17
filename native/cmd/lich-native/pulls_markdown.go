package main

import (
	"image/color"
	"log"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/x/markdown"
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
}

// markdownBlock is a run of prose or a GFM table, which x/markdown's flat
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

// layout draws src as Markdown, mdProse's text-sm muted body with the links
// in the foreground (h1 text-lg, h2 text-base, the rest text-sm) and its
// tables my-2 apart; a link clicked lands in open.
func (m markdownCache) layout(gtx C, th *ui.Theme, key, src string, open func(url string)) D {
	t, ok := m[key]
	if !ok {
		t = new(markdownText)
		m[key] = t
	}
	fg := [2]color.NRGBA{th.MutedForeground, th.Foreground}
	if t.src != src || t.fg != fg || t.blocks == nil {
		t.src, t.fg, t.blocks = src, fg, renderBlocks(th, src)
	}
	children := make([]layout.FlexChild, 0, 2*len(t.blocks))
	for i, b := range t.blocks {
		if i > 0 {
			children = append(children, ui.VGap(2))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			if b.table != nil {
				return b.table.layout(gtx, th, open)
			}
			return b.prose.layout(gtx, th, open)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func renderBlocks(th *ui.Theme, src string) []markdownBlock {
	var out []markdownBlock
	for _, seg := range splitMarkdownTables(src) {
		if seg.table == nil {
			out = append(out, markdownBlock{prose: &markdownCell{spans: renderMarkdown(th, seg.text, th.MutedForeground)}})
			continue
		}
		out = append(out, markdownBlock{table: renderTable(th, seg.table)})
	}
	return out
}

func (c *markdownCell) layout(gtx C, th *ui.Theme, open func(url string)) D {
	for {
		span, ev, ok := c.state.Update(gtx)
		if !ok {
			break
		}
		if url, isLink := span.Get(markdown.MetadataURL).(string); isLink && url != "" && ev.Type == richtext.Click && open != nil {
			open(url)
		}
	}
	return richtext.Text(&c.state, th.Shaper, c.spans...).Layout(gtx)
}

func renderMarkdown(th *ui.Theme, src string, fg color.NRGBA) []richtext.SpanStyle {
	r := markdown.NewRenderer()
	r.Config = markdown.Config{
		DefaultFont:      font.Font{Typeface: th.Sans},
		MonospaceFont:    font.Font{Typeface: th.Mono},
		DefaultSize:      ui.TextSM,
		H1Size:           markdownH1,
		H2Size:           ui.TextBase,
		H3Size:           ui.TextSM,
		H4Size:           ui.TextSM,
		H5Size:           ui.TextSM,
		H6Size:           ui.TextSM,
		DefaultColor:     fg,
		InteractiveColor: th.Foreground,
	}
	spans, err := r.Render([]byte(src))
	if err != nil {
		log.Printf("markdown: %v", err)
		return []richtext.SpanStyle{{Content: src, Size: ui.TextSM, Color: fg, Font: font.Font{Typeface: th.Sans}}}
	}
	return spans
}
