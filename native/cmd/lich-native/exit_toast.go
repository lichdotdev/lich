package main

import (
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// conversationIDShown is how much of a conversation id the exit toast names:
// enough to tell two apart, as git's short hash is.
const conversationIDShown = 8

func exitToastID(id string) string { return "exit:" + id }

// exitFailed is a code other than 0, or none: a process killed by a signal
// left no clean exit either.
func exitFailed(e sessionExit) bool { return e.Code == nil || *e.Code != 0 }

func exitTitle(label string, e sessionExit) string {
	if e.Code == nil || *e.Code == 0 {
		return i18n.T("terminal.exitToast.ended", i18n.Args{"label": label})
	}
	return i18n.T("terminal.exitToast.endedWithCode", i18n.Args{"label": label, "code": *e.Code})
}

// exitToast asks what sv's card does once its process is gone. A card with a
// conversation offers it back; restart takes the conversation to reopen, ""
// for a new one.
func exitToast(sv sessionView, e sessionExit, restart func(resume string), closeCard func()) ui.Toast {
	t := ui.Toast{
		ID:       exitToastID(sv.ID),
		Variant:  ui.ToastInfo,
		Title:    exitTitle(sv.Label, e),
		Duration: ui.ToastForever,
		Cancel:   &ui.ToastButton{Label: i18n.T("terminal.exitToast.closeSession"), OnClick: closeCard},
	}
	if exitFailed(e) {
		t.Variant = ui.ToastError
	}
	if !resumable(sv.Session) {
		t.Action = &ui.ToastButton{Label: i18n.T("terminal.exitToast.restart"), OnClick: func() { restart("") }}
		return t
	}
	conversation := sv.ProviderSessionID
	short := conversation[:min(len(conversation), conversationIDShown)]
	t.Description = i18n.T("terminal.exitToast.resumes", i18n.Args{"id": short})
	if sv.profileName != "" {
		t.Description = i18n.T("terminal.exitToast.resumesIn", i18n.Args{"id": short, "profile": sv.profileName})
	}
	t.Secondary = &ui.ToastButton{Label: i18n.T("terminal.exitToast.startNew"), OnClick: func() { restart("") }}
	t.Action = &ui.ToastButton{Label: i18n.T("terminal.exitToast.resume"), OnClick: func() { restart(conversation) }}
	return t
}

func (u *window) sessionExited(id string, e sessionExit) {
	u.terms.exited(id, e)
	u.raiseExit(id)
}

// raiseExit puts card id's exit toast up while its terminal holds a process
// that is gone; a card never shown here has no terminal and nothing to ask.
func (u *window) raiseExit(id string) {
	e, ok := u.terms.exitOf(id)
	if !ok {
		return
	}
	sv, ok := u.model.sessionViewOf(id)
	if !ok {
		return
	}
	u.exitMu.Lock()
	if u.exitRaised == nil {
		u.exitRaised = map[string]bool{}
	}
	u.exitRaised[id] = true
	u.exitMu.Unlock()
	u.duplicates.Show(exitToast(sv, e,
		func(resume string) { go u.restartExited(sv, resume) },
		func() { u.closeSession(id) }))
}

// restartExited asks again when the process did not start: the card is still
// exited, and its toast went with the click.
func (u *window) restartExited(sv sessionView, resume string) {
	if !u.terms.restart(u.ctx, sv, resume) {
		u.raiseExit(sv.ID)
	}
}

// syncExitToasts runs each frame with the card that has focus. Focusing an
// exited card whose toast was dismissed raises it again; a toast whose card
// runs again, by whatever route, or whose terminal is gone, goes.
func (u *window) syncExitToasts(focused string) {
	if focused != u.exitFocus {
		u.exitFocus = focused
		if !u.duplicates.Live(exitToastID(focused)) {
			u.raiseExit(focused)
		}
	}
	u.exitMu.Lock()
	defer u.exitMu.Unlock()
	for id := range u.exitRaised {
		if _, exited := u.terms.exitOf(id); !exited {
			delete(u.exitRaised, id)
			u.duplicates.Dismiss(exitToastID(id))
		}
	}
}
