package main

import (
	"runtime"
	"strings"

	"gioui.org/app"

	"github.com/lichdotdev/lich/api"
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
// a session at its prompt, the terminal's or the chat's composer, focusing
// its pane as a click would.
func (u *window) onDrag(e app.DropEvent, invalidate func()) {
	was := u.drop.overSession()
	id, paths := u.drop.handle(e)
	if u.drop.overSession() != was {
		invalidate()
	}
	if len(paths) == 0 {
		return
	}
	sv, ok := u.model.sessionViewOf(id)
	if !ok {
		return
	}
	u.model.focusPane(u.ctx, id)
	invalidate()
	target := dropTarget{sessionID: sv.ID, checkout: sv.path, confined: sv.sandbox.Confined}
	if sv.View == api.ViewChat {
		go u.dropInChat(u.chats.pane(sv.ID), target, paths, invalidate)
		return
	}
	go u.dropFiles(target, paths)
}

// dropFiles resolves a drop and writes it at the session's prompt.
func (u *window) dropFiles(target dropTarget, paths []string) {
	if paste := u.droppedPaste(target, paths); paste != "" {
		u.logErr("drop files", u.terms.writePaste(u.ctx, target.sessionID, paste))
	}
}

// dropInChat resolves a drop and hands it to the chat's composer p, which
// types it at its caret on the next frame: the text the terminal would have
// pasted, without the paste's brackets. An image is a path like any file;
// the chat sends the agent text alone.
func (u *window) dropInChat(p *chatPane, target dropTarget, paths []string, invalidate func()) {
	paste := u.droppedPaste(target, paths)
	if paste == "" {
		return
	}
	p.mu.Lock()
	p.dropped += strings.TrimSuffix(strings.TrimPrefix(paste, pasteStart), pasteEnd)
	p.mu.Unlock()
	invalidate()
}

// droppedPaste resolves a drop into the paste of its paths, toasting what
// yielded none; "" when nothing did.
func (u *window) droppedPaste(target dropTarget, paths []string) string {
	r := resolveDrop(u.ctx, u.client, target, paths)
	if len(r.skipped) > 0 {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("terminal.drop.notAttached", i18n.Args{"files": strings.Join(r.skipped, ", ")})})
	}
	return composeDroppedPaths(r.paths, runtime.GOOS == "windows", r.notices)
}
