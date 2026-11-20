package main

import (
	"image"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The stop reasons a chat turn ends with (api.ChatTurn.StopReason).
const (
	stopCancelled = "cancelled"
	stopError     = "error"
)

// chatDoneWord is how a turn's line names its end, by the stop reason the
// event carried: cancelled is Stopped and error is Failed, and any other stop
// a provider reports (a refusal, a token limit) still ended the turn.
func chatDoneWord(reason string) string {
	switch reason {
	case stopCancelled:
		return i18n.T("chat.done.stopped")
	case stopError:
		return i18n.T("chat.done.failed")
	}
	return i18n.T("chat.done.done")
}

// chatChangesText is the files of a turn's count, "3 files".
func chatChangesText(c api.ChatTurnChanges) string {
	return i18n.T("chat.done.files", i18n.Args{"count": c.Files})
}

// chatDone is the line that closes a finished turn: the provider's avatar, the
// word that says how it ended and, muted, what the turn changed in the checkout,
// with a hairline to the column's edge as the turn's tool calls have. The count
// of files and lines is the click: it opens Review, where the footer's changes
// item does. A turn with no count to give (the checkout could not be read, or
// the turn was read back with the history) says its word alone. The avatar,
// word and count are drawn at opacity shown, and the hairline over share rule
// of its room from its left end: both 1 at rest, less while the live line
// turns into this one.
func (u *window) chatDone(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow, shown, rule float32) D {
	end, _ := u.model.chat.endOf(sv.ID, r.turn)
	f := p.fold("done:" + strconv.Itoa(r.turn))
	if f.click.Clicked(gtx) {
		u.dock.show(dockReview)
	}
	tint, mark, _ := u.agentLook(sv.ID)
	word := th.Text(ui.TextSM, chatDoneWord(end.StopReason)).Weight(font.Medium)
	if end.StopReason == stopError {
		word = word.In(th.Destructive)
	}
	faded := func(w layout.Widget) layout.FlexChild {
		if shown == 1 {
			return layout.Rigid(w)
		}
		return layout.Rigid(func(gtx C) D {
			defer paint.PushOpacity(gtx.Ops, shown).Pop()
			return w(gtx)
		})
	}
	line := []layout.FlexChild{
		faded(func(gtx C) D { return chatAvatar(gtx, th, tint, mark) }),
		ui.Gap(2),
		faded(word.Layout),
	}
	if c := end.Changes; c != nil {
		line = append(line, ui.Gap(2), faded(th.Text(ui.TextSM, "·").Muted().Layout), ui.Gap(2), faded(func(gtx C) D { return u.chatDoneCount(gtx, th, f, *c) }))
	}
	line = append(line, ui.Gap(3), layout.Flexed(1, func(gtx C) D {
		d := D{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))}
		gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X)*rule + 0.5)
		chatHairline(gtx, th)
		return d
	}))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, line...)
}

// chatDoneCount is "no changes", or the clickable files with the lines added in
// TonePass and removed in Destructive.
func (u *window) chatDoneCount(gtx C, th *ui.Theme, f *chatFold, c api.ChatTurnChanges) D {
	if c.Files == 0 {
		return th.Text(ui.TextSM, i18n.T("chat.done.noChanges")).Muted().Layout(gtx)
	}
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
}
