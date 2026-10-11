package main

import (
	"log"
	"runtime"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/ui"
)

const (
	// dropHintFill is TerminalDropHint's bg-accent/45.
	dropHintFill = 0.45
	// dropHintDetailMin is the pane height below which the detail line goes,
	// the web's @container(max-height:120px).
	dropHintDetailMin = unit.Dp(120)
)

// dropHint covers the terminal while a file drag is over it, as
// TerminalDropHint does: which session the file lands in, and under it what
// the drop does. th draws on the terminal's background.
func dropHint(gtx C, th *ui.Theme, label string, confined bool) D {
	// The fill stays translucent so the terminal reads through it, as the
	// web's does; th.Over is only the color the text measures its contrast
	// against.
	tint := ui.Alpha(th.Accent, dropHintFill)
	paint.Fill(gtx.Ops, tint)
	on := th.On(th.Over(tint))
	showDetail := gtx.Constraints.Max.Y >= gtx.Dp(dropHintDetailMin)
	detail := "Its path is pasted at the prompt"
	if confined {
		detail = "Outside the checkout it arrives as a copy"
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{Left: ui.Space(4), Right: ui.Space(4)}.Layout(gtx, func(gtx C) D {
			rows := []layout.FlexChild{
				layout.Rigid(func(gtx C) D { return icAttach.Layout(gtx, ui.Space(5), on.Foreground) }),
				ui.VGap(1.5),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(on.Text(ui.TextSM, "Attach to").Layout),
						ui.Gap(1),
						layout.Rigid(on.Text(ui.TextSM, label).Weight(font.SemiBold).Layout),
					)
				}),
			}
			if showDetail {
				rows = append(rows, ui.VGap(1.5), layout.Rigid(on.Text(ui.TextXS, detail).Muted().Layout))
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, rows...)
		})
	})
}

// onDrag follows one OS file drag over the window, and pastes what lands on
// the terminal at the active session's prompt.
func (u *window) onDrag(e app.DropEvent, invalidate func()) {
	was := u.drop.hinting()
	paths := u.drop.handle(e)
	if u.drop.hinting() != was {
		invalidate()
	}
	if len(paths) == 0 {
		return
	}
	sv, ok := u.model.activeSession()
	if !ok {
		return
	}
	target := dropTarget{sessionID: sv.ID, checkout: sv.path, confined: sv.sandbox.Confined}
	go u.dropFiles(target, paths)
}

// dropFiles resolves a drop and writes it at the session's prompt.
// TODO: the skipped entries go to the log; the web toasts them
// ("Not attached: …"), and they move there once this window has toasts.
func (u *window) dropFiles(target dropTarget, paths []string) {
	r := resolveDrop(u.ctx, u.client, target, paths)
	if len(r.skipped) > 0 {
		log.Printf("Not attached: %v", r.skipped)
	}
	paste := composeDroppedPaths(r.paths, runtime.GOOS == "windows", r.notices)
	if paste == "" {
		return
	}
	u.logErr("drop files", u.client.Write(u.ctx, target.sessionID, paste))
}
