package main

import (
	"runtime"
	"strings"

	"gioui.org/app"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// dropHintFor is the hint for a file drag over the terminal of the session
// named label, its detail line what the drop does there.
func dropHintFor(th *ui.Theme, label string, confined bool) ui.DropHintStyle {
	detail := i18n.T("terminal.dropHint.pasted")
	if confined {
		detail = i18n.T("terminal.dropHint.confined")
	}
	before, after := attachToSplit()
	return th.DropHint(icAttach, before, label, after, detail)
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
	u.logErr("drop files", u.terms.writePaste(u.ctx, target.sessionID, paste))
}
