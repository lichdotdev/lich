package main

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

type sessionDuplicate = api.SessionDuplicate

// duplicateWarning is what the warning names and where switching lands.
type duplicateWarning struct {
	id, label, holder, holderLabel, holderProject string
}

// duplicateOf reads the warning off the open sessions, false once either
// card is gone: a warning about a card no longer on screen has nothing left
// to switch from.
func (m *model) duplicateOf(d sessionDuplicate) (duplicateWarning, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, i := m.sessionAt(d.ID)
	hp, hi := m.sessionAt(d.HeldBy)
	if p == nil || hp == nil {
		return duplicateWarning{}, false
	}
	return duplicateWarning{
		id: d.ID, label: p.Sessions[i].Label,
		holder: d.HeldBy, holderLabel: hp.Sessions[hi].Label, holderProject: hp.ID,
	}, true
}

// warnDuplicate raises the warning in the bottom-right toaster, where it
// stays until answered: the conversation goes on in two cards until then.
func (u *window) warnDuplicate(d sessionDuplicate) {
	w, ok := u.model.duplicateOf(d)
	if !ok {
		return
	}
	u.duplicates.Show(ui.Toast{
		ID:       "duplicate:" + w.id,
		Variant:  ui.ToastWarning,
		Title:    i18n.T("terminal.duplicate.title", i18n.Args{"label": w.label, "holder": w.holderLabel}),
		Duration: ui.ToastForever,
		Cancel:   &ui.ToastButton{Label: i18n.T("terminal.duplicate.keepBoth")},
		Action:   &ui.ToastButton{Label: i18n.T("terminal.duplicate.switch"), OnClick: func() { u.switchToHolder(w) }},
	})
}

// switchToHolder drops the card that resumed and puts the holder on screen.
// The row is parked and then forgotten rather than left parked: a parked copy
// of a conversation another card runs is all the history hides and a reopen
// refuses, so keeping it would keep nothing.
func (u *window) switchToHolder(w duplicateWarning) {
	projectID, active, ok := u.model.closeSession(w.id)
	if !ok {
		return
	}
	u.terms.drop(w.id)
	u.gate.forget(w.id)
	u.openSession(w.holderProject, w.holder)
	go func() {
		u.logErr("close session", u.client.CloseSession(u.ctx, projectID, w.id, active))
		u.logErr("forget session", u.client.ForgetSession(u.ctx, w.id))
		u.logErr("close terminal", u.client.CloseTerminal(u.ctx, w.id))
	}()
}
