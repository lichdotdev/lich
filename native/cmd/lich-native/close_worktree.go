package main

import (
	"slices"
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// worktreeLeftBy is the project and the worktree whose last open session is
// id, the close that decides that checkout's fate. path is empty when the
// session shares its checkout or runs in the project's own directory.
func (m *model) worktreeLeftBy(id string) (project backend.Project, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, i := m.sessionAt(id)
	if p == nil || p.Sessions[i].Path == "" {
		return backend.Project{}, ""
	}
	path = p.Sessions[i].Path
	if slices.ContainsFunc(p.Sessions, func(s backend.Session) bool { return s.ID != id && s.Path == path }) {
		return backend.Project{}, ""
	}
	return *p, path
}

// removeClosedWorktree is the closed toast's Remove worktree: the close
// already parked the session, so what is left is the checkout. One lich did
// not create, or one holding uncommitted work, is named in a dialog before it
// goes, as useWorktreeClose does; a failed check reads as the half that asks.
func (u *window) removeClosedWorktree(project backend.Project, path string, parked <-chan error) {
	if err := <-parked; err != nil {
		return // closeSession logged it; the session still holds the checkout
	}
	adopted, err := u.client.WorktreeAdopted(u.ctx, path)
	adopted = adopted || err != nil
	dirty, err := u.client.WorktreeDirty(u.ctx, path)
	u.logErr("worktree dirty", err)
	if adopted || dirty {
		u.closeWorktree.ask(project, path, adopted, dirty)
		u.model.invalidate()
		return
	}
	u.removeCheckout(project, path, false, false)
}

// closeWorktreeConfirm asks before the closed toast removes a worktree lich
// did not create or one with uncommitted changes.
type closeWorktreeConfirm struct {
	dialog ui.DialogState
	// mu guards the question, which the removal asks off the frame.
	mu              sync.Mutex
	open            bool
	project         backend.Project
	path            string
	adopted, dirty  bool
	cancel, confirm widget.Clickable
}

func (c *closeWorktreeConfirm) ask(project backend.Project, path string, adopted, dirty bool) {
	c.mu.Lock()
	c.open, c.project, c.path, c.adopted, c.dirty = true, project, path, adopted, dirty
	c.mu.Unlock()
}

// copy is the dialog's title, description and action for what the checkout
// holds. A dirty one is removed with force, and the adopted sentence leads
// when both apply, so the directory is named as the user's before it goes.
func (c *closeWorktreeConfirm) copy() (title, desc, action string) {
	title, action = i18n.T("sidebar.worktreeCloseDialogs.closeTitle"), i18n.T("sidebar.worktreeCloseDialogs.remove")
	if c.adopted {
		desc = i18n.T("sidebar.closeWorktree.adopted", i18n.Args{"path": c.path})
	}
	if c.dirty {
		title, action = i18n.T("sidebar.worktreeCloseDialogs.dirtyTitle"), i18n.T("sidebar.worktreeCloseDialogs.discardAndRemove")
		desc = strings.TrimSpace(desc + " " + i18n.T("sidebar.worktreeCloseDialogs.dirtyDescription", i18n.Args{"path": c.path}))
	}
	return title, desc, action
}

// layoutCloseWorktreeConfirm draws the question and removes on its yes. The
// removal acknowledges an adopted checkout: the dialog just named it.
func (u *window) layoutCloseWorktreeConfirm(gtx C, th *ui.Theme) {
	c := &u.closeWorktree
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.confirm.Clicked(gtx) && c.open {
		c.open = false
		project, path, force := c.project, c.path, c.dirty
		go u.removeCheckout(project, path, force, true)
	}
	if c.cancel.Clicked(gtx) {
		c.open = false
	}
	title, desc, action := c.copy()
	on := th.On(th.Popover)
	cancel := on.Button(&c.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantOutline
	remove := on.Button(&c.confirm, action)
	remove.Variant = ui.VariantDestructive
	d := th.Dialog(&c.dialog, c.open, title, desc)
	d.Actions = []layout.Widget{cancel.Layout, remove.Layout}
	if d.Layout(gtx) {
		c.open = false
	}
}
