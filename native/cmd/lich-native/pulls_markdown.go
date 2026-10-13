package main

import (
	"image/color"
	"log"

	"gioui.org/font"
	"gioui.org/unit"
	"gioui.org/x/markdown"
	"gioui.org/x/richtext"

	"github.com/omartelo/lich/native/ui"
)

// markdownH1 is mdProse's h1 text-lg.
const markdownH1 = unit.Sp(18)

// markdownText is one rendered Markdown body: its spans, parsed once per
// text and theme, and the link state it is laid out with.
type markdownText struct {
	src   string
	fg    [2]color.NRGBA
	spans []richtext.SpanStyle
	state richtext.InteractiveText
}

// markdownCache keeps each body's markdownText by a key of the caller's, so
// a frame does not parse the same text again and a link's hover survives.
type markdownCache map[string]*markdownText

// layout draws src as Markdown, mdProse's text-sm muted body with the links
// in the foreground (h1 text-lg, h2 text-base, the rest text-sm); a link
// clicked lands in open.
func (m markdownCache) layout(gtx C, th *ui.Theme, key, src string, open func(url string)) D {
	t, ok := m[key]
	if !ok {
		t = new(markdownText)
		m[key] = t
	}
	fg := [2]color.NRGBA{th.MutedForeground, th.Foreground}
	if t.src != src || t.fg != fg || t.spans == nil {
		t.src, t.fg, t.spans = src, fg, renderMarkdown(th, src)
	}
	for {
		span, ev, ok := t.state.Update(gtx)
		if !ok {
			break
		}
		if url, isLink := span.Get(markdown.MetadataURL).(string); isLink && url != "" && ev.Type == richtext.Click && open != nil {
			open(url)
		}
	}
	return richtext.Text(&t.state, th.Shaper, t.spans...).Layout(gtx)
}

func renderMarkdown(th *ui.Theme, src string) []richtext.SpanStyle {
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
		DefaultColor:     th.MutedForeground,
		InteractiveColor: th.Foreground,
	}
	spans, err := r.Render([]byte(src))
	if err != nil {
		log.Printf("markdown: %v", err)
		return []richtext.SpanStyle{{Content: src, Size: ui.TextSM, Color: th.MutedForeground, Font: font.Font{Typeface: th.Sans}}}
	}
	return spans
}
