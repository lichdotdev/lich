package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n.
const (
	closeTitle     = "Sessions are still running"
	closeBodyOne   = "Closing %s stops a session mid-turn. Reopening the project offers to resume where each left off; the turn in flight is lost."
	closeBodyOther = "Closing %s stops %d sessions mid-turn. Reopening the project offers to resume where each left off; the turn in flight is lost."
	closeAnyway    = "Close anyway"
	closeCancel    = "Cancel"
)

// closeConfirm is ProjectTabs' ConfirmDialog: closing a project kills its
// terminals, and an agent mid-turn dies with them, so that close asks first.
type closeConfirm struct {
	dialog  ui.DialogState
	open    bool
	id      string
	name    string
	running int
	cancel  widget.Clickable
	confirm widget.Clickable
}

// layoutConfirm draws the dialog and answers the project its button closes,
// "" for none.
func (t *topbar) layoutConfirm(gtx C, th *ui.Theme) string {
	c := &t.confirm
	closed := ""
	if c.confirm.Clicked(gtx) && c.open {
		closed, c.open = c.id, false
	}
	if c.cancel.Clicked(gtx) {
		c.open = false
	}
	on := th.On(th.Popover)
	cancel := on.Button(&c.cancel, closeCancel)
	cancel.Variant = ui.VariantOutline
	anyway := on.Button(&c.confirm, closeAnyway)
	anyway.Variant = ui.VariantDestructive
	d := th.Dialog(&c.dialog, c.open, closeTitle, closeBody(c.name, c.running))
	d.Actions = []layout.Widget{cancel.Layout, anyway.Layout}
	if d.Layout(gtx) {
		c.open = false
	}
	return closed
}

func closeBody(name string, running int) string {
	if running == 1 {
		return fmt.Sprintf(closeBodyOne, name)
	}
	return fmt.Sprintf(closeBodyOther, name, running)
}

// applyTopbar carries out what the strip's clicks asked for.
func (u *window) applyTopbar(act topbarAction) {
	if act.home {
		u.model.showHome()
		u.model.invalidate()
		u.saveScreen("")
	}
	if act.show != "" {
		u.showProject(act.show)
	}
	if act.ask != "" {
		u.requestClose(act.ask)
	}
	if act.close != "" {
		u.closeProject(act.close)
	}
	if d := act.drop; d.ID != "" {
		if ids := u.model.moveProject(d.ID, d.Over); ids != nil {
			u.model.invalidate()
			go func() { u.logErr("reorder projects", u.client.ReorderProjects(u.ctx, ids)) }()
		}
	}
	if n := act.notify; n != nil {
		u.openNotification(*n)
	}
	if act.dismiss != "" {
		u.model.dismiss(u.ctx, act.dismiss)
		u.model.invalidate()
	}
	if act.theme {
		u.toggleTheme()
	}
	if act.pick {
		go u.pickProject()
	}
	if r := act.reopen; r != nil {
		go u.reopenProject(*r)
	}
}

// showProject puts a tab's project on screen and records it as the screen
// the next window opens on.
func (u *window) showProject(id string) {
	if !u.model.setActiveProject(id) {
		return
	}
	u.model.invalidate()
	u.saveScreen(id)
}

func (u *window) saveScreen(projectID string) {
	go func() { u.logErr("save last screen", saveLastScreen(u.ctx, u.client, projectID)) }()
}

// requestClose closes a project at once, or asks first while any of its
// sessions is mid-turn.
func (u *window) requestClose(id string) {
	running := u.model.runningIn(id)
	if running == 0 {
		u.closeProject(id)
		return
	}
	tabs, _ := u.model.projectTabs()
	for _, tab := range tabs {
		if tab.ID == id {
			c := &u.topbar.confirm
			c.open, c.id, c.name, c.running = true, id, tab.Name, running
		}
	}
}

// closeProject closes a project in the backend and stops its sessions'
// terminals, moving the screen to its neighbor when it was on it.
func (u *window) closeProject(id string) {
	out, ok := u.model.closeProject(id)
	if !ok {
		return
	}
	for _, s := range out.sessions {
		u.terms.drop(s)
	}
	if out.moved {
		if out.next == "" || !u.model.setActiveProject(out.next) {
			u.model.showHome()
		}
		u.saveScreen(out.next)
	}
	u.model.invalidate()
	go func() {
		u.logErr("close project", u.client.CloseProject(u.ctx, id))
		for _, s := range out.sessions {
			u.logErr("close terminal", u.client.CloseTerminal(u.ctx, s))
		}
	}()
}

// openNotification routes a queue row to its project and session.
func (u *window) openNotification(n notification) {
	if !u.model.setActiveProject(n.projectID) {
		return
	}
	u.model.setActive(u.ctx, n.ID)
	u.model.invalidate()
	u.saveScreen(n.projectID)
	go func() { u.logErr("set active session", u.client.SetActiveSession(u.ctx, n.projectID, n.ID)) }()
}

// pickProject opens the directory picker and adopts what it answers.
func (u *window) pickProject() {
	p, err := u.client.PickProject(u.ctx)
	if err != nil || p == nil {
		u.logErr("open project", err)
		return
	}
	u.logErr("open project", u.adopt(*p))
}

// reopenProject is projects.tsx openRecent: a project whose directory is
// gone asks the picker where it went, keeping its id.
func (u *window) reopenProject(r lichclient.RecentProject) {
	there, err := u.client.PathExists(u.ctx, r.Path)
	if err != nil {
		u.logErr("reopen project", err)
		return
	}
	if there {
		u.logErr("reopen project", u.adopt(r))
		return
	}
	moved, err := u.client.RelocateProject(u.ctx, r.ID)
	if err != nil {
		u.toaster.Show(failure(toastRelocateFailed, err))
		return
	}
	if moved == nil {
		return
	}
	if err := u.adopt(*moved); err != nil {
		u.toaster.Show(failure(toastRelocateFailed, err))
		return
	}
	u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: fmt.Sprintf(toastRelocated, moved.Name, displayPath(moved.Path))})
}

// adopt is projects.tsx adopt: add (or reopen) the project, reload, and put
// it on screen.
func (u *window) adopt(p lichclient.RecentProject) error {
	if err := u.client.AddProject(u.ctx, p.ID, p.Name, p.Path); err != nil {
		return fmt.Errorf("add project: %w", err)
	}
	if err := u.model.reload(u.ctx); err != nil {
		return fmt.Errorf("reload: %w", err)
	}
	u.logErr("load folds", u.model.loadFolds(u.ctx))
	u.showProject(p.ID)
	return nil
}
