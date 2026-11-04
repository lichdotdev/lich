package main

import (
	"slices"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// closeAll is a checkout menu's Close all sessions: it asks first when any of
// ids is mid-turn, and closes them all otherwise.
func (u *window) closeAll(ids []string) {
	if running := u.model.runningOf(ids); running > 0 {
		u.closeAllConfirm.ask(ids, running)
		return
	}
	u.closeSessions(ids)
}

func (u *window) closeSessions(ids []string) {
	for _, id := range ids {
		u.closeSession(id)
	}
}

// runningOf is how many of ids hold a turn.
func (m *model) runningOf(ids []string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.status.running(ids))
}

// closeAllConfirm asks before Close all sessions stops a turn in flight.
type closeAllConfirm struct {
	dialog  ui.DialogState
	open    bool
	ids     []string
	running int
	cancel  widget.Clickable
	confirm widget.Clickable
}

func (c *closeAllConfirm) ask(ids []string, running int) {
	c.open, c.ids, c.running = true, slices.Clone(ids), running
}

// layoutCloseAllConfirm draws the question and closes every session on its yes.
func (u *window) layoutCloseAllConfirm(gtx C, th *ui.Theme) {
	c := &u.closeAllConfirm
	if !c.open && !c.dialog.Animating(gtx) {
		return
	}
	if c.confirm.Clicked(gtx) && c.open {
		c.open = false
		u.closeSessions(c.ids)
	}
	if c.cancel.Clicked(gtx) {
		c.open = false
	}
	on := th.On(th.Popover)
	cancel := on.Button(&c.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantOutline
	anyway := on.Button(&c.confirm, i18n.T("sidebar.closeAllDialog.closeAnyway"))
	anyway.Variant = ui.VariantDestructive
	d := th.Dialog(&c.dialog, c.open, i18n.T("sidebar.closeAllDialog.title"), i18n.T("sidebar.closeAllDialog.running", i18n.Args{"count": c.running}))
	d.Actions = []layout.Widget{cancel.Layout, anyway.Layout}
	if d.Layout(gtx) {
		c.open = false
	}
}
