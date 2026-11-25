package main

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"

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

// chatDone is the line that closes a finished turn: the word that says how it
// ended, then, muted, its calls and how long it took, and "no changes" when
// the checkout says the turn changed nothing; what it did change is on its
// edits' rows (chatEdit). A turn with no count to give (the checkout could not
// be read, or the turn was read back with the history) says its word alone.
// The line is drawn at opacity shown: 1 at rest, less while the live line
// turns into this one.
func (u *window) chatDone(gtx C, th *ui.Theme, sv sessionView, r chatRow, shown float32) D {
	end, _ := u.model.chat.endOf(sv.ID, r.turn)
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
	muted := func(s string) []layout.FlexChild {
		return []layout.FlexChild{ui.Gap(2), faded(th.Text(ui.TextSM, "·").Muted().Layout), ui.Gap(2), faded(th.Text(ui.TextSM, s).Muted().Layout)}
	}
	line := []layout.FlexChild{faded(word.Layout)}
	if len(r.tools) > 0 {
		line = append(line, muted(i18n.T("chat.view.toolCalls", i18n.Args{"count": len(r.tools)})+" · "+chatTook(r.took))...)
	}
	if c := end.Changes; c != nil && c.Files == 0 {
		line = append(line, muted(i18n.T("chat.done.noChanges"))...)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, line...)
}
