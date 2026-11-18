package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

// dockPeek is the collapsed dock brought out over the stage from the window's
// right edge. Like the sidebar's, it keeps the terminal at its width.
type dockPeek struct {
	panelPeek
	slide dockSlide
	edge  hoverArea
	panel hoverArea
	// tab is the tab the dock was on last frame: a tab opened from anywhere
	// (the footer, a hotkey, a path's Ctrl+click) brings a collapsed dock out.
	tab dockTab
}

// visible is whether the dock is on screen: beside the terminal, full screen,
// or out over the stage.
func (d *dock) visible() bool {
	return d.tab != dockClosed && (!d.collapsed || d.fullscreen || d.peek.shown)
}

// bringOut brings a collapsed dock out until the pointer has been in it.
func (d *dock) bringOut() {
	if d.collapsed {
		d.peek.hold()
	}
}

// setCollapsed takes the dock off the stage, or puts it back. Put back from
// out over the stage, it is already where it stands, so it lands without a
// slide.
func (d *dock) setCollapsed(collapsed bool) {
	d.collapsed, d.saveCollapsed, d.peek.tab = collapsed, true, d.tab
	if !collapsed && d.peek.slide.shown > 0 {
		d.slide.snap(true)
	}
	d.peek.panelPeek = panelPeek{}
	d.peek.slide.snap(false)
}

// show opens the dock on t from anywhere but its own tabs: the footer, a
// hotkey, a path's Ctrl+click, the Git panel. A collapsed dock comes out.
func (d *dock) show(t dockTab) {
	d.tab = t
	d.bringOut()
}

// docked is whether the dock stands beside the terminal this frame, or
// covers it in full screen.
func (d *dock) docked(home bool) bool {
	return d.tab != dockClosed && !home && (!d.collapsed || d.fullscreen)
}

// dockPeek draws the collapsed dock out over the stage row, whose size
// gtx holds, while it has a tab to show.
func (u *window) dockPeek(gtx C, v stageView) error {
	d := u.dock
	p := &d.peek
	if d.tab == dockClosed || v.home || !d.collapsed || d.fullscreen {
		p.panelPeek, p.edge, p.panel, p.tab = panelPeek{}, hoverArea{}, hoverArea{}, d.tab
		p.slide.snap(false)
		return nil
	}
	if d.tab != p.tab {
		d.bringOut()
	}
	p.tab = d.tab
	p.edge.read(gtx)
	p.panel.read(gtx)
	p.point(gtx.Now, p.edge.over || p.panel.over)
	_, commenting := u.comments.edited()
	if wake := p.update(gtx.Now, d.holdsKeys(gtx) || d.resize.Dragging() || commenting); !wake.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: wake})
	}
	size := gtx.Constraints.Max
	// The edge is the gutter between the stage and the window's right edge,
	// where the pointer lands when it is thrown to the side.
	p.edge.watch(gtx, image.Rect(size.X, 0, size.X+gtx.Dp(gutter), size.Y))
	share := p.slide.at(gtx, p.shown)
	if share == 0 {
		return nil
	}
	w := d.width(gtx)
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(size.X-int(share*float32(w)), 0)).Push(gtx.Ops).Pop()
	// The panel's own area takes the pointer from the terminal under it, and
	// reads it over everything the dock draws.
	area := clip.UniformRRect(image.Rectangle{Max: image.Pt(w, size.Y)}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops)
	event.Op(gtx.Ops, &p.panel)
	gtx.Constraints = layout.Exact(image.Pt(w, size.Y))
	if share < 1 {
		// A moving panel takes no input; the slide asked for its frames.
		gtx = gtx.Disabled()
	}
	err := u.layoutDock(gtx, v)
	area.Pop()
	return err
}

// layoutDock lays the dock out in gtx.Constraints.Max and keeps what it
// asked for: a width to store, an extension's action.
func (u *window) layoutDock(gtx C, v stageView) error {
	in := dockInput{sessionID: v.in.sessionID, inject: u.terms.injector(v.in.sessionID), submit: u.terms.submitter(v.in.sessionID), checkout: v.in.path, git: v.in.git, isGit: v.in.isGit, light: u.light, code: u.code, placed: placedIn(v.in.placed, extension.PlaceDock)}
	save, err := u.dock.layout(gtx, u.theme, &u.root, in)
	u.applyExtension(u.dock.takeExtensionAction())
	if save {
		rem := u.dock.widthRem
		go func() { u.logErr("dock width", saveDockWidth(u.ctx, u.client, rem)) }()
	}
	return err
}
