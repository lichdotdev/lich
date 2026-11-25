package main

import (
	"gioui.org/layout"
	"gioui.org/op/paint"
	"github.com/lichdotdev/lich/native/ui/icons"
	"image/color"

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
// chatDoneGlyph is how a turn ended, in its tone: a check, the stop square
// muted, or the failed cross.
func chatDoneGlyph(th *ui.Theme, reason string) (*icons.Icon, color.NRGBA) {
	switch reason {
	case stopCancelled:
		return icChatStop, th.MutedForeground
	case stopError:
		return icToolsFailed, th.Destructive
	}
	return icCheck, th.TonePass
}

func chatDoneWord(reason string) string {
	switch reason {
	case stopCancelled:
		return i18n.T("chat.done.stopped")
	case stopError:
		return i18n.T("chat.done.failed")
	}
	return i18n.T("chat.done.done")
}

// chatDone is the line that closes a finished turn: the glyph that says how it
// ended, its word for whoever reads the window without seeing it, then, muted, its calls and how long it took, and "no changes" when
// the checkout says the turn changed nothing; what it did change is on its
// edits' rows (chatEdit). A turn with no count to give (the checkout could not
// be read, or the turn was read back with the history) shows its glyph alone.
// The line is drawn at opacity shown: 1 at rest, less while the live line
// turns into this one.
func (u *window) chatDone(gtx C, th *ui.Theme, sv sessionView, r chatRow, shown float32) D {
	end, _ := u.model.chat.endOf(sv.ID, r.turn)
	ic, ink := chatDoneGlyph(th, end.StopReason)
	word := func(gtx C) D {
		return labeledGlyph(gtx, chatDoneWord(end.StopReason), func(gtx C) D { return ic.Layout(gtx, ui.Space(4), ink) })
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
	first := true
	muted := func(s string) []layout.FlexChild {
		sep := []layout.FlexChild{ui.Gap(2)}
		if !first {
			sep = append(sep, faded(th.Text(ui.TextSM, "·").Muted().Layout), ui.Gap(2))
		}
		first = false
		return append(sep, faded(th.Text(ui.TextSM, s).Muted().Layout))
	}
	line := []layout.FlexChild{faded(word)}
	if len(r.tools) > 0 {
		line = append(line, muted(i18n.T("chat.view.toolCalls", i18n.Args{"count": len(r.tools)})+" · "+chatTook(r.took))...)
	}
	if c := end.Changes; c != nil && c.Files == 0 {
		line = append(line, muted(i18n.T("chat.done.noChanges"))...)
	}
	// a line of text tall even when the glyph stands alone, so the live line
	// turns into it in its slot
	gtx.Constraints.Min.Y = gtx.Sp(chatRowLine)
	return layout.W.Layout(gtx, func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, line...) })
}
