package main

import (
	"runtime"
	"strings"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/i18n"
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
	detail := i18n.T("terminal.dropHint.pasted")
	if confined {
		detail = i18n.T("terminal.dropHint.confined")
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{Left: ui.Space(4), Right: ui.Space(4)}.Layout(gtx, func(gtx C) D {
			rows := []layout.FlexChild{
				layout.Rigid(func(gtx C) D { return icAttach.Layout(gtx, ui.Space(5), on.Foreground) }),
				ui.VGap(1.5),
				layout.Rigid(func(gtx C) D {
					before, after := attachToSplit()
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(on.Text(ui.TextSM, before).Layout),
						layout.Rigid(on.Text(ui.TextSM, label).Weight(font.SemiBold).Layout),
						layout.Rigid(on.Text(ui.TextSM, after).Layout),
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

// attachToSplit is the hint's headline around the session's name, which is
// laid out in its own weight between the two halves.
func attachToSplit() (before, after string) {
	const marker = "\x00"
	before, after, _ = strings.Cut(i18n.T("terminal.dropHint.attachTo", i18n.Args{"label": marker}), marker)
	return before, after
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
func (u *window) dropFiles(target dropTarget, paths []string) {
	r := resolveDrop(u.ctx, u.client, target, paths)
	if len(r.skipped) > 0 {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("terminal.drop.notAttached", i18n.Args{"files": strings.Join(r.skipped, ", ")})})
	}
	paste := composeDroppedPaths(r.paths, runtime.GOOS == "windows", r.notices)
	if paste == "" {
		return
	}
	u.logErr("drop files", u.client.Write(u.ctx, target.sessionID, paste))
}
