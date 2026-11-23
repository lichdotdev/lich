package main

import (
	"strings"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// vetoOutcome draws a provider's subagent call lich opened as a card as
// opened, with a click that opens the card, rather than as the failed call
// the provider reports for a veto (docs/hooks/subagent-cards.md, What the
// model and the window see). An ACP chat names its tool items "tool-" and
// the provider's call id.
func (u *window) vetoOutcome(p *chatPane, sv sessionView) func(backend.ChatItem) layout.Widget {
	return func(it backend.ChatItem) layout.Widget {
		opened, ok := u.model.vetoes.opened(sv.ID, it.ID)
		if !ok {
			opened, ok = u.model.vetoes.opened(sv.ID, strings.TrimPrefix(it.ID, "tool-"))
		}
		if !ok {
			return nil
		}
		click := p.note("veto:" + it.ID)
		return func(gtx C) D {
			if click.Clicked(gtx) {
				if card, ok := u.model.sessionViewOf(opened[0].ID); ok {
					u.openSession(card.projectID, card.ID)
				}
			}
			labels := make([]string, len(opened))
			for i, o := range opened {
				labels[i] = o.Label
			}
			text := i18n.T("chat.view.openedAs", i18n.Args{"label": strings.Join(labels, ", ")})
			th := u.theme
			return layout.Inset{Left: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				return click.Layout(gtx, th.Text(ui.TextXS, text).Muted().Layout)
			})
		}
	}
}
