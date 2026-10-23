package main

import (
	"crypto/rand"
	"slices"
	"strings"
	"time"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const undoToastDuration = 10 * time.Second

// closedToast is closeSession's toast: Undo puts s back at index of its
// project once parked reports the park done.
func (u *window) closedToast(projectID string, s backend.Session, index int, parked <-chan error) ui.Toast {
	return ui.Toast{Title: i18n.T("shell.projects.closed", i18n.Args{"label": s.Label}), Duration: undoToastDuration,
		Action: &ui.ToastButton{Label: i18n.T("shell.projects.undo"), OnClick: func() {
			go u.undoClose(projectID, s, index, parked)
		}}}
}

// undoClose restores a closed session. It waits on the park: the
// store can sit on a lock for seconds, and a reopen that overtook it would
// find the row still open and do nothing. The row comes back under a fresh
// id, unspawned, so the spawn gate offers the conversation's resume as for
// any restored card; when another open card already runs that conversation,
// the store hands that card back instead and the undo only focuses it.
func (u *window) undoClose(projectID string, s backend.Session, index int, parked <-chan error) {
	if err := <-parked; err != nil {
		return // closeSession logged it; there is no parked row to bring back
	}
	restored, err := u.client.ReopenSession(u.ctx, s.ID, strings.ToLower(rand.Text()))
	if err != nil || restored == nil {
		u.logErr("reopen session", err)
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.projects.bringBackFailed", i18n.Args{"label": s.Label})})
		return
	}
	ids, ok := u.model.restoreSession(projectID, *restored, index)
	if !ok {
		return // the project was closed while the toast was up
	}
	u.model.invalidate()
	// The reopen appends, so the slot the card went back to is a reorder.
	u.logErr("reorder sessions", u.client.ReorderSessions(u.ctx, projectID, ids))
}

// slotOf is the open session with id and its place in its project.
func (m *model) slotOf(id string) (backend.Session, int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, i := m.sessionAt(id)
	if p == nil {
		return backend.Session{}, -1, false
	}
	return p.Sessions[i], i, true
}

// restoreSession puts s back into its project at index, or the end when the
// project has fewer, and makes it the project's active session. It returns
// the project's card order, and false when the project is no longer open.
func (m *model) restoreSession(projectID string, s backend.Session, index int) ([]string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pi := slices.IndexFunc(m.projects, func(p backend.Project) bool { return p.ID == projectID })
	if pi < 0 {
		return nil, false
	}
	p := &m.projects[pi]
	if !slices.ContainsFunc(p.Sessions, func(o backend.Session) bool { return o.ID == s.ID }) {
		s.Path = checkoutPath(s.Path, p.Path)
		p.Sessions = slices.Insert(p.Sessions, min(index, len(p.Sessions)), s)
	}
	if m.shownProject() == p {
		m.active = s.ID
	}
	p.ActiveSessionID = s.ID
	return sessionIDs(*p), true
}
