package main

import (
	"strconv"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// chatChangesText is the files of a turn's count, "3 files".
func chatChangesText(c api.ChatTurnChanges) string {
	return i18n.T("chat.changes.files", i18n.Args{"count": c.Files})
}

// chatChanges is what a finished turn changed in the checkout, after its last
// item: the files, muted and lit on hover, then the lines added in TonePass and
// removed in Destructive. It opens Review, where the footer's changes item does.
func (u *window) chatChanges(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow) D {
	c, ok := u.model.chat.changesOf(sv.ID, r.turn)
	if !ok {
		return D{}
	}
	f := p.fold("changes:" + strconv.Itoa(r.turn))
	if f.click.Clicked(gtx) {
		u.dock.show(dockReview)
	}
	return layout.W.Layout(gtx, func(gtx C) D {
		return f.click.Layout(gtx, func(gtx C) D {
			fg := th.MutedForeground
			if f.click.Hovered() {
				fg = th.Foreground
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextSM, chatChangesText(c)).In(fg).Layout),
				ui.Gap(2),
				layout.Rigid(th.Text(ui.TextSM, "+"+strconv.Itoa(c.Added)).In(th.TonePass).Layout),
				ui.Gap(1.5),
				layout.Rigid(th.Text(ui.TextSM, "−"+strconv.Itoa(c.Removed)).In(th.Destructive).Layout),
			)
		})
	})
}
