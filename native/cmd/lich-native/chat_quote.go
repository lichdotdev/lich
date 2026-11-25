package main

import (
	"image"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// Text selected in a message offers Quote over it, which puts it over the
// composer's text as a block taken off only whole. The message sent carries
// each quote as Markdown blockquote lines before the text, plain text every
// provider takes as it is, and its balloon draws them as a blockquote.

var (
	icQuote = icons.Lucide("quote")
	// chatQuoteBar is the blockquote's bar, as wide as the one Markdown draws.
	chatQuoteBar = ui.Space(1)
	// chatQuoteGap is the room between two quotes, and the cards over them.
	chatQuoteGap = ui.Space(2)
)

// chatQuoteLines is how many lines a quote in the composer shows.
const chatQuoteLines = 2

// chatQuoteRow is a quote's × in the composer.
type chatQuoteRow struct {
	remove    ui.Clickable
	removeTip ui.Tooltip
}

// quotedText is the message sent for quotes over typed: each quote's lines
// behind "> " and a blank line after it, then typed.
func quotedText(quotes []string, typed string) string {
	var b strings.Builder
	for _, q := range quotes {
		for line := range strings.SplitSeq(q, "\n") {
			b.WriteString(strings.TrimRight("> "+line, " "))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	b.WriteString(typed)
	return strings.TrimRight(b.String(), "\n")
}

// addQuote puts text under the quotes the composer holds; blank, nothing.
func (p *chatPane) addQuote(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	p.quotes = append(p.quotes, text)
	if len(p.quoteRows) < len(p.quotes) {
		p.quoteRows = append(p.quoteRows, chatQuoteRow{})
	}
}

// quoteActs takes off the quote whose × was clicked.
func (p *chatPane) quoteActs(gtx C) {
	for i := range p.quotes {
		if p.quoteRows[i].remove.Clicked(gtx) {
			p.quotes = append(p.quotes[:i], p.quotes[i+1:]...)
			return
		}
	}
}

// quoteButton is Links.Quote for p's messages: Quote on a popover, which
// takes the selection into the composer and gives it the keys. metric is the
// window's, which a message drawn at its own size has scaled.
func (u *window) quoteButton(p *chatPane, th *ui.Theme, metric unit.Metric) func(gtx C, text string) D {
	return func(gtx C, text string) D {
		gtx.Metric = metric
		if p.quoteSel.Clicked(gtx) {
			p.addQuote(text)
			p.md.DropSelection()
			gtx.Execute(key.FocusCmd{Tag: &p.editor})
			gtx.Execute(op.InvalidateCmd{})
		}
		on := th.On(th.Popover)
		b := on.Button(&p.quoteSel, i18n.T("chat.quote.add"))
		b.Icon, b.Variant, b.Size = icQuote, ui.VariantGhost, ui.SizeXS
		m := op.Record(gtx.Ops)
		d := b.Layout(gtx)
		call := m.Stop()
		gtx.Constraints.Max.X = d.Size.X + 2*gtx.Dp(ui.Space(1))
		return th.PopoverPanel(gtx, func(gtx C, _ *ui.Theme) D {
			call.Add(gtx.Ops)
			return d
		})
	}
}

// composerQuotes stacks the composer's quotes over its text, nothing without
// any.
func (u *window) composerQuotes(gtx C, th *ui.Theme, p *chatPane) D {
	if len(p.quotes) == 0 {
		return D{}
	}
	rows := make([]layout.FlexChild, 0, len(p.quotes))
	for i, q := range p.quotes {
		r := &p.quoteRows[i]
		rows = append(rows, layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: chatQuoteGap}.Layout(gtx, func(gtx C) D { return u.composerQuote(gtx, th, r, q) })
		}))
	}
	return layout.Inset{Left: chatComposerPadX, Right: chatComposerPadX, Top: chatComposerPadY - chatQuoteGap}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

// composerQuote is one quote: its text behind the blockquote's bar on Accent,
// cut at chatQuoteLines, and its × at the right.
func (u *window) composerQuote(gtx C, th *ui.Theme, r *chatQuoteRow, text string) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	on := th.On(th.Accent)
	x := on.Button(&r.remove, "")
	x.Icon, x.Variant, x.Size = icQueueDrop, ui.VariantGhost, ui.SizeIconXS
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			paint.FillShape(gtx.Ops, th.Accent, clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(ui.RadiusMD)).Op(gtx.Ops))
			return D{Size: gtx.Constraints.Min}
		},
		func(gtx C) D {
			return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: ui.Space(1.5), Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				d := layout.Flex{}.Layout(gtx,
					layout.Rigid(layout.Spacer{Width: chatQuoteBar}.Layout),
					ui.Gap(2),
					layout.Flexed(1, func(gtx C) D {
						l := on.Text(ui.TextSM, text).Muted()
						l.MaxLines = chatQuoteLines
						return l.Layout(gtx)
					}),
					ui.Gap(1),
					layout.Rigid(func(gtx C) D {
						return th.Tooltip(&u.root, &r.removeTip, i18n.T("chat.quote.remove")).Layout(gtx, x.Layout)
					}),
				)
				paint.FillShape(gtx.Ops, th.Border, clip.Rect{Max: image.Pt(gtx.Dp(chatQuoteBar), d.Size.Y)}.Op())
				return d
			})
		})
}
