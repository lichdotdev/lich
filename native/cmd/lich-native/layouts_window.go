package main

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The window's side of a split: picking the session, the room check, and
// the question before a session leaves the layout it is in.

func (u *window) hasLayout() bool {
	_, ok := u.model.shownLayoutTab()
	return ok
}

// hotkeyUnsplit takes the focused pane out of the layout on screen.
func (u *window) hotkeyUnsplit(C) {
	active, ok := u.model.activeSession()
	if !ok {
		return
	}
	u.model.unsplit(u.ctx, active.ID)
	u.model.invalidate()
}

// pickBeside opens the session picker for a pane against the active
// session's edge.
func (u *window) pickBeside(edge paneEdge) {
	active, ok := u.model.activeSession()
	if !ok {
		return
	}
	u.settings.hide()
	u.palette.showBeside(turnsAmong(u.model.paletteWorld().sessions), active.ID, edge)
}

// besidePicked splits the requested pane with session id: refused with a
// toast when that pane has no room for one more beside it, asked about first
// when id sits in another layout.
func (u *window) besidePicked(r besideRequest, id string) {
	if id == r.target {
		return
	}
	if !u.fitsBeside(r.target, r.edge) {
		u.toaster.Show(ui.Toast{Title: i18n.T("layouts.split.noRoom")})
		return
	}
	from := u.model.layoutOf(id)
	if from != "" && from != u.model.layoutOf(r.target) {
		u.askMove(r, id, from)
		return
	}
	if from != "" {
		u.model.unsplit(u.ctx, id)
	}
	u.model.splitBeside(u.ctx, r.target, id, r.edge, false)
	u.model.invalidate()
}

// fitsBeside reports whether target's pane, as last drawn, has room to split
// against edge.
func (u *window) fitsBeside(target string, edge paneEdge) bool {
	return u.layoutStage.pointer.fits(target, edge)
}

// moveConfirm is the question before a split takes a session out of the
// layout it is in: the user built that layout, so it never loses a pane
// unasked.
type moveConfirm struct {
	dialog  ui.DialogState
	open    bool
	req     besideRequest
	session string
	label   string
	from    string
	cancel  widget.Clickable
	confirm widget.Clickable
}

func (u *window) askMove(r besideRequest, id, from string) {
	c := &u.layoutStage.move
	c.open, c.req, c.session, c.from = true, r, id, u.model.layoutName(from)
	c.label = u.model.sessionLabel(id)
}

// layoutMoveConfirm draws the question and makes the move it is answered
// with.
func (u *window) layoutMoveConfirm(gtx C, th *ui.Theme) {
	c := &u.layoutStage.move
	if c.confirm.Clicked(gtx) && c.open {
		c.open = false
		u.model.splitBeside(u.ctx, c.req.target, c.session, c.req.edge, true)
		u.model.invalidate()
	}
	if c.cancel.Clicked(gtx) {
		c.open = false
	}
	on := th.On(th.Popover)
	cancel := on.Button(&c.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantOutline
	move := on.Button(&c.confirm, i18n.T("layouts.move.confirm"))
	body := i18n.T("layouts.move.body", i18n.Args{"label": c.label, "layout": c.from})
	d := th.Dialog(&c.dialog, c.open, i18n.T("layouts.move.title", i18n.Args{"label": c.label}), body)
	d.Actions = []layout.Widget{cancel.Layout, move.Layout}
	if d.Layout(gtx) {
		c.open = false
	}
}
