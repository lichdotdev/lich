package main

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// A request (kind="request") is what the person asked for with a button: drawn
// as their own message, a box holding the action's title, its facts, its rows
// and, folded, the whole text the agent read.

var (
	icRequestChevronUp = icons.Lucide("chevron-up")
	// requestGlyphs mark each topic's box.
	requestGlyphs = map[string]*icons.Icon{
		api.RequestConflicts:      icons.Lucide("git-merge"),
		api.RequestChecks:         icons.Lucide("circle-x"),
		api.RequestReviewThread:   icons.Lucide("message-square"),
		api.RequestReviewComments: icons.Lucide("pencil"),
		api.RequestBrowserNotes:   icons.Lucide("crosshair"),
		api.RequestIssue:          icons.Lucide("circle-dot"),
	}
	icRequestFailed = icons.Lucide("circle-x")
)

// requestGlyphSize is the topic glyph and the chevron, on the summary's line.
var requestGlyphSize = unit.Dp(14)

func (u *window) chatRequest(gtx C, th *ui.Theme, p *chatPane, key string, env envelope) D {
	f := p.fold("request:" + key)
	f.update(gtx)
	maxW := int(float32(gtx.Constraints.Max.X) * chatUserShare)
	on := th.On(th.Accent)
	return layout.E.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = maxW
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, th.Accent, ui.RadiusLG) },
			func(gtx C) D {
				return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(3), Right: ui.Space(3)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, u.requestRows(gtx, on, p, key, f, env)...)
				})
			})
	})
}

// requestRows are the box's lines: the summary with the topic's glyph and the
// chevron, the facts, the rows, the sketch, and the text when unfolded.
func (u *window) requestRows(gtx C, th *ui.Theme, p *chatPane, key string, f *chatFold, env envelope) []layout.FlexChild {
	chevron := icChatOpen
	if f.open {
		chevron = icRequestChevronUp
	}
	glyph := requestGlyphs[env.topic]
	head := func(gtx C) D {
		return f.click.Layout(gtx, func(gtx C) D {
			row := []layout.FlexChild{}
			if glyph != nil {
				row = append(row, layout.Rigid(func(gtx C) D { return glyph.Layout(gtx, requestGlyphSize, th.Foreground) }), ui.Gap(1.5))
			}
			row = append(row,
				layout.Rigid(th.Text(ui.TextSM, env.text["summary"]).Weight(font.Medium).Layout),
				ui.Gap(3),
				layout.Rigid(func(gtx C) D { return chevron.Layout(gtx, requestGlyphSize, th.MutedForeground) }),
			)
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
		})
	}
	rows := []layout.FlexChild{layout.Rigid(head)}
	for _, fact := range requestFacts(env) {
		rows = append(rows, ui.VGap(0.5), layout.Rigid(th.Text(ui.TextXS, fact).Mono().Muted().Layout))
	}
	if len(env.items) > 0 {
		rows = append(rows, ui.VGap(1))
	}
	for i, it := range env.items {
		rows = append(rows, ui.VGap(0.5), layout.Rigid(func(gtx C) D { return requestItemRow(gtx, th, env.topic, i, it) }))
	}
	if img := env.attrs["image"]; img != "" {
		rows = append(rows, ui.VGap(1.5), layout.Rigid(func(gtx C) D {
			return u.fileCard(gtx, th, p.card(key+"\x00"+img), img, th.Background, false)
		}))
	}
	if f.open {
		rows = append(rows, ui.VGap(3),
			layout.Rigid(th.Text(ui.TextXS, expandTabs(env.text["prompt"])).Muted().Wrapped().Layout))
	}
	return rows
}

// requestFacts are the muted lines under the summary: the pull request and
// its branches, the page, or the issue.
func requestFacts(env envelope) []string {
	a := env.attrs
	switch env.topic {
	case api.RequestConflicts, api.RequestChecks, api.RequestReviewThread:
		fact := "#" + a["pr"] + " · " + a["branch"]
		if a["base"] != "" {
			fact += " → " + a["base"]
		}
		return []string{fact}
	case api.RequestBrowserNotes:
		return nonEmpty(a["url"])
	case api.RequestIssue:
		return nonEmpty(a["title"], a["url"])
	}
	return nil
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// requestItemRow is one row of the box's list, in the words of its topic.
func requestItemRow(gtx C, th *ui.Theme, topic string, i int, it envelopeItem) D {
	a := it.attrs
	quote := func(s string) string { return "\"" + strings.ReplaceAll(strings.TrimSpace(s), "\n", " ") + "\"" }
	var row []layout.FlexChild
	mono := func(s string) layout.FlexChild { return layout.Rigid(th.Text(ui.TextXS, s).Mono().Layout) }
	muted := func(s string) layout.FlexChild { return layout.Rigid(th.Text(ui.TextXS, s).Muted().Layout) }
	switch topic {
	case api.RequestConflicts:
		row = append(row, mono(a["path"]))
	case api.RequestChecks:
		row = append(row, layout.Rigid(func(gtx C) D { return icRequestFailed.Layout(gtx, unit.Dp(12), th.Destructive) }), ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, a["check"]).Layout))
	case api.RequestReviewThread:
		where := a["path"] + ":" + a["lines"]
		said := quote(it.text)
		if a["outdated"] == "true" {
			said = "outdated · " + said
		}
		row = append(row, mono(where), ui.Gap(2), muted(said))
	case api.RequestReviewComments:
		where := a["path"]
		if a["line"] != "" {
			where += ":" + a["line"]
		}
		if where != "" {
			row = append(row, mono(where), ui.Gap(2))
		}
		row = append(row, muted(quote(it.text)))
	case api.RequestBrowserNotes:
		row = append(row, layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(i+1)+".").Muted().Layout), ui.Gap(1.5), mono(a["label"]))
		if it.text != "" {
			row = append(row, ui.Gap(2), muted(quote(it.text)))
		}
	default:
		row = append(row, muted(it.text))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
}
