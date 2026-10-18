package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
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
	cancel := on.Button(&c.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantOutline
	anyway := on.Button(&c.confirm, i18n.T("tabs.projectTabs.closeAnyway"))
	anyway.Variant = ui.VariantDestructive
	d := th.Dialog(&c.dialog, c.open, i18n.T("tabs.projectTabs.closeTitle"), closeBody(c.name, c.running))
	d.Actions = []layout.Widget{cancel.Layout, anyway.Layout}
	if d.Layout(gtx) {
		c.open = false
	}
	return closed
}

func closeBody(name string, running int) string {
	return i18n.T("tabs.projectTabs.closeBody", i18n.Args{"name": name, "count": running})
}

// applyTopbar carries out what the strip's clicks asked for.
func (u *window) applyTopbar(act topbarAction) {
	u.applySettingsAction(act)
	if act.home || act.show != "" {
		u.pulls.hide()
	}
	if act.pulls {
		u.togglePulls()
	}
	if act.home {
		u.model.showHome()
		u.model.invalidate()
		u.saveScreen("")
	}
	if act.show != "" {
		u.showStripTab(act.show)
	}
	if act.ask != "" {
		u.requestClose(act.ask)
	}
	if r := act.renamed; r.id != "" && u.model.renameLayout(r.id, r.name) {
		u.model.saveLayouts()
		u.model.invalidate()
	}
	if act.close != "" {
		u.closeProject(act.close)
	}
	if d := act.drop; d.ID != "" && u.model.moveLayout(d.ID, d.Over) {
		u.model.saveLayouts()
		u.model.invalidate()
	} else if d.ID != "" {
		if ids := u.model.moveProject(d.ID, d.Over); ids != nil {
			u.model.invalidate()
			go func() {
				if err := u.model.storeOrder(u.ctx, ids); err != nil {
					u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("tabs.topbarApply.reorderFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
					u.model.invalidate()
				}
			}()
		}
	}
	if n := act.notify; n != nil {
		u.openNotification(*n)
	}
	if act.dismiss != "" {
		u.model.dismiss(u.ctx, act.dismiss)
		u.model.invalidate()
	}
	if act.pick {
		go u.pickProject()
	}
	if r := act.reopen; r != nil {
		go u.reopenProject(*r)
	}
}

// applySettingsAction opens or closes Settings from the strip: its button,
// a tab's "Project settings…", and a tab or Home leaving it for the stage.
func (u *window) applySettingsAction(act topbarAction) {
	open := func(scope string) {
		tabs, _ := u.model.projectTabs()
		u.settings.show(scope, settingsProjectsOf(tabs))
	}
	leaving := act.home || act.show != ""
	switch {
	case act.projectSettings != "":
		open(act.projectSettings)
	case act.settings && u.settings.open, leaving && u.settings.open:
		u.settings.hide()
	case act.settings:
		open("")
	default:
		return
	}
	u.model.invalidate()
}

// customizeFooter is the footer menu's Customize footer…: Settings opens on
// the Footer row, lit.
func (u *window) customizeFooter() {
	tabs, _ := u.model.projectTabs()
	u.settings.reveal(sectionAppearance, i18n.T("settings.footerSettings.title"), settingsProjectsOf(tabs))
	u.model.invalidate()
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

// showStripTab puts a strip tab on screen: a project, or a layout.
func (u *window) showStripTab(id string) {
	if u.model.showLayout(u.ctx, id) {
		u.model.invalidate()
		return
	}
	u.showProject(id)
}

func (u *window) saveScreen(projectID string) {
	go func() { u.logErr("save last screen", saveLastScreen(u.ctx, u.client, projectID)) }()
}

// requestClose closes a project at once, or asks first while any of its
// sessions is mid-turn.
func (u *window) requestClose(id string) {
	if u.model.isLayout(id) {
		u.model.closeLayout(u.ctx, id)
		u.model.invalidate()
		return
	}
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

// closeProject has the backend close a project. The tab leaves, and the
// screen moves, when project-closed comes back (projectClosed); a refusal is a
// toast and the strip stays as it was.
func (u *window) closeProject(id string) {
	go func() {
		sessions, err := u.model.askClose(u.ctx, id)
		if err != nil {
			u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("tabs.topbarApply.closeProjectFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
			return
		}
		for _, s := range sessions {
			u.logErr("close terminal", u.client.CloseTerminal(u.ctx, s))
		}
	}()
}

// projectClosed carries out what a project leaving the strip leaves to do:
// stop its sessions' terminals and move the screen off it.
func (u *window) projectClosed(out closedProject) {
	for _, s := range out.sessions {
		u.terms.drop(s)
	}
	if out.moved {
		if out.next == "" || !u.model.setActiveProject(out.next) {
			u.model.showHome()
		}
		u.saveScreen(out.next)
	}
}

// openNotification routes a queue row to its project and session.
func (u *window) openNotification(n notification) { u.openSession(n.projectID, n.ID) }

// openSession puts a session of any open project on screen.
func (u *window) openSession(projectID, id string) {
	if !u.model.setActiveProject(projectID) {
		return
	}
	u.model.setActive(u.ctx, id)
	u.model.invalidate()
	u.saveScreen(projectID)
	go func() { u.logErr("set active session", u.client.SetActiveSession(u.ctx, projectID, id)) }()
}

// focusSession is `lich focus`: the card comes up, over Settings too, which
// the next frame takes down because only the frame may touch it.
func (u *window) focusSession(projectID, id string) {
	u.hideSettings.Store(true)
	u.openSession(projectID, id)
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
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.projects.relocateFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
		return
	}
	if moved == nil {
		return
	}
	if err := u.adopt(*moved); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.projects.relocateFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
		return
	}
	u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("shell.projects.relocated", i18n.Args{"name": moved.Name, "path": displayPath(moved.Path)})})
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
