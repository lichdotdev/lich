package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/ui"
)

// sessionBandHeight is what a lone agent session's terminal gives up at its
// top: the GUI | TUI selector with the island's edge over and under it, the
// mirror of balloonBand, in whole rows once the grid rounds it down.
var sessionBandHeight = balloonInset + chatSwitchBox + balloonInset

// sessionBand is the height the stage's lone session leaves the strip at its
// top, 0 where there is none. It follows the session's kind, as ptyBand does,
// so the grid is resized once however the agent in its PTY comes and goes. A
// layout's panes have their own header, a plain terminal, a service's console
// and Home carry no strip, and an open Git panel does not take it back: it
// rises from the foot.
func (u *window) sessionBand(gtx C, v stageView) int {
	if v.home || v.hasConsole || v.hasLayout || !v.hasActive || v.active.Kind == "shell" {
		return 0
	}
	return gtx.Dp(sessionBandHeight)
}

// bandedSession lays out session sv over gtx's area under the strip it names
// itself in, band tall: the terminal or the chat takes what the strip leaves.
func (u *window) bandedSession(gtx C, sv sessionView, band int, bg color.NRGBA, keys bool) error {
	if band == 0 {
		return u.content(gtx, sv, keys)
	}
	size := gtx.Constraints.Max
	strip := gtx
	strip.Constraints = layout.Exact(image.Pt(size.X, band))
	u.sessionStrip(strip, u.theme.On(bg), sv)
	defer op.Offset(image.Pt(0, band)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(size.X, max(size.Y-band, 0)))
	return u.content(gtx, sv, keys)
}

// sessionStrip is the band's row: the provider and the session's name floating
// at the left, with no ground, cut with "…" when the room is short, and the
// selector at the right, which takes its room first.
func (u *window) sessionStrip(gtx C, th *ui.Theme, sv sessionView) {
	gtx.Constraints.Min = gtx.Constraints.Max
	layout.Inset{Left: balloonInset, Right: balloonInset}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return u.sessionName(gtx, th, sv) }),
			layout.Rigid(func(gtx C) D { return u.chatSwitch(gtx, th, sv) }),
		)
	})
}

// sessionName is "Claude Code · Cart totals a cent off": the provider as the
// window names it, in MutedForeground, then the card's label.
func (u *window) sessionName(gtx C, th *ui.Theme, sv sessionView) D {
	gtx.Constraints.Min.X = 0
	name := layout.Flexed(1, th.Text(ui.TextSM, sv.Label).In(th.Foreground).Truncated().Layout)
	p, ok := u.model.providers.find(sv.Kind)
	if !ok {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, name)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextSM, p.name+" · ").Muted().Truncated().Layout),
		name,
	)
}

// paneSwitch is a layout pane's selector, at the right of its header, which
// already names the session: no strip of its own, and the pane's terminal
// keeps every row.
func (u *window) paneSwitch(gtx C, th *ui.Theme, sv sessionView) D {
	if !u.model.chat.offers(sv.Kind) {
		return D{}
	}
	return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5)}.Layout(gtx, func(gtx C) D { return u.chatSwitch(gtx, th, sv) })
}
