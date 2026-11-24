package main

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icOpenCard = icons.Lucide("arrow-up-right")

// chatOpened is how a call lich opened as a card draws in its row: the card's
// provider mark in place of the tool's glyph, and a pill per card that opens
// it.
type chatOpened struct {
	mark, links layout.Widget
}

// vetoOutcome draws a provider's subagent call lich opened as a card as
// opened, with a click that opens the card, rather than as the failed call
// the provider reports for a veto (docs/hooks/subagent-cards.md, What the
// model and the window see). An ACP chat names its tool items "tool-" and
// the provider's call id.
func (u *window) vetoOutcome(p *chatPane, sv sessionView) func(backend.ChatItem) *chatOpened {
	return func(it backend.ChatItem) *chatOpened {
		opened, ok := u.model.vetoes.opened(sv.ID, it.ID)
		if !ok {
			opened, ok = u.model.vetoes.opened(sv.ID, strings.TrimPrefix(it.ID, "tool-"))
		}
		if !ok {
			return nil
		}
		th := u.theme
		first, known := u.model.sessionViewOf(opened[0].ID)
		out := &chatOpened{links: func(gtx C) D {
			row := make([]layout.FlexChild, 0, 2*len(opened))
			for i, o := range opened {
				row = append(row, ui.Gap(1.5), layout.Rigid(func(gtx C) D {
					return u.openedPill(gtx, th, p.note("veto:"+it.ID+":"+strconv.Itoa(i)), o.ID, o.Label)
				}))
			}
			return layout.Inset{Left: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
			})
		}}
		if known {
			out.mark = func(gtx C) D { return providerGlyph(gtx, th, childKind(first), ui.Space(3.5), markPixel) }
		}
		return out
	}
}

// openedPill is a card lich opened: its state's dot, its name and an arrow,
// in a pill tinted by the row's ink, lit on hover; a click opens the card.
func (u *window) openedPill(gtx C, th *ui.Theme, click *ui.Clickable, id, label string) D {
	if click.Clicked(gtx) {
		if card, ok := u.model.sessionViewOf(id); ok {
			u.openSession(card.projectID, card.ID)
		}
	}
	sv, _ := u.model.sessionViewOf(id)
	ink := th.Destructive
	fill, edge := ui.Alpha(ink, 0.08), ui.Alpha(ink, 0.35)
	if click.Hovered() {
		fill, edge = ui.Alpha(ink, 0.18), ink
	}
	return click.Layout(gtx, func(gtx C) D {
		m := op.Record(gtx.Ops)
		gtx.Constraints.Min = image.Point{}
		d := layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return ui.StatusDot(gtx, childDot(th, sv)) }),
				ui.Gap(0.5),
				layout.Rigid(th.Text(ui.TextXS, label).In(ink).Layout),
				ui.Gap(1),
				layout.Rigid(func(gtx C) D { return icOpenCard.Layout(gtx, ui.Space(3), ink) }),
			)
		})
		call := m.Stop()
		paint.FillShape(gtx.Ops, th.Over(fill), clip.UniformRRect(image.Rectangle{Max: d.Size}, d.Size.Y/2).Op(gtx.Ops))
		ui.InsetBorder(gtx, d.Size, d.Size.Y/2, th.Over(edge))
		call.Add(gtx.Ops)
		return d
	})
}
