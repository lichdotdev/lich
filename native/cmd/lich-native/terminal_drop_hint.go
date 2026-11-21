package main

import (
	"context"
	"runtime"
	"strings"

	"gioui.org/app"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
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
		go u.dropInChat(target, paths)
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

// dropInChat resolves a drop into cards in the chat's composer.
func (u *window) dropInChat(target dropTarget, paths []string) {
	if r := u.resolved(target, paths); len(r.files) > 0 {
		u.chats.attachIn(target.sessionID, r.files)
		u.model.invalidate()
	}
}

// attachAtPrompt puts file a at session id's prompt: a card in its chat's
// composer when it is drawn as a chat, which has no terminal to write to,
// else its path pasted at its terminal.
func (u *window) attachAtPrompt(ctx context.Context, id string, a backend.Attachment) error {
	if sv, ok := u.model.sessionViewOf(id); ok && sv.View == api.ViewChat {
		u.chats.attachIn(id, []backend.Attachment{a})
		u.model.invalidate()
		return nil
	}
	return u.terms.writePaste(ctx, id, composeDroppedPaths([]string{a.Path}, runtime.GOOS == "windows", []string{a.Notice}))
}

// typeInChat hands a paste to session id's composer, which types it at its
// caret on the next frame: the text the terminal would have taken, without
// the paste's brackets.
func (u *window) typeInChat(_ context.Context, id, block string) error {
	u.chats.typeIn(id, strings.TrimSuffix(strings.TrimPrefix(block, pasteStart), pasteEnd))
	u.model.invalidate()
	return nil
}

// droppedPaste resolves a drop into the paste of its paths, toasting what
// yielded none; "" when nothing did.
func (u *window) droppedPaste(target dropTarget, paths []string) string {
	r := u.resolved(target, paths)
	return composeDroppedPaths(r.paths, runtime.GOOS == "windows", r.notices)
}

// resolved resolves a drop, toasting what yielded no path.
func (u *window) resolved(target dropTarget, paths []string) dropResult {
	r := resolveDrop(u.ctx, u.client, target, paths)
	if len(r.skipped) > 0 {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("terminal.drop.notAttached", i18n.Args{"files": strings.Join(r.skipped, ", ")})})
	}
	return r
}

// injector is what writes a reference at session id's prompt from the Code
// tab, the editor or a pull request's files: its chat's composer when it is
// drawn as a chat, else its terminal (terminals.injector). The view is read
// at the write, which may come after a switch.
func (u *window) injector(id string) func(string) bool {
	term := u.terms.injector(id)
	return func(text string) bool {
		if sv, ok := u.model.sessionViewOf(id); ok && sv.View == api.ViewChat {
			u.logErr("inject", u.typeInChat(u.ctx, id, safeToWrite(text)))
			return true
		}
		return term(text)
	}
}
