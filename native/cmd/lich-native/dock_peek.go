package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
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
	edge  windowEdge
	panel hoverArea
	// stage is the pointer on the stage beside the dock: back there from
	// out of the window, it is no longer on its way to the dock.
	stage hoverArea
	// tab is the tab the dock was on last frame: a tab opened from anywhere
	// (a balloon, a hotkey, a path's Ctrl+click) brings a collapsed dock out.
	tab dockTab
	// box is where the dock floats this frame, in window pixels, empty
	// while it does not.
	box image.Rectangle
}

// windowEdge is the strip at the window's right edge that brings the
// collapsed dock out the moment the pointer touches it. out is the pointer
// gone out of the window across it, which counts as still on it.
type windowEdge struct {
	over, out bool
	strip     image.Rectangle
}

// read takes the strip's events, and reports whether the pointer touched it
// since the last frame, even if it went on past it within the same frame.
func (e *windowEdge) read(gtx C) (touched bool) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: e, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel})
		if !ok {
			return touched
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		e.over = pe.Kind == pointer.Enter
		touched = touched || e.over
		// Out of the window, the pointer's last place is still on the strip;
		// onto the stage or the dock, it is left of it.
		e.out = pe.Kind == pointer.Leave && pe.Position.X >= float32(e.strip.Min.X)
	}
}

// watch lays the strip over r, letting every event through to what is under
// it.
func (e *windowEdge) watch(gtx C, r image.Rectangle) {
	e.strip = r
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect(r).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, e)
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

// show opens the dock on t from anywhere but its own tabs: a balloon, a
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
	p.box = image.Rectangle{}
	if d.tab == dockClosed || v.home || !d.collapsed || d.fullscreen {
		p.panelPeek, p.edge, p.panel, p.stage, p.tab = panelPeek{}, windowEdge{}, hoverArea{}, hoverArea{}, d.tab
		p.slide.snap(false)
		return nil
	}
	if d.tab != p.tab {
		d.bringOut()
	}
	p.tab = d.tab
	touched := p.edge.read(gtx)
	p.panel.read(gtx)
	p.stage.read(gtx)
	if p.stage.over || p.panel.over {
		p.edge.out = false
	}
	p.point(gtx.Now, p.edge.over || p.edge.out || p.panel.over)
	if touched || p.edge.over || p.edge.out {
		// The window's edge is a place you throw the pointer at, not one it
		// crosses on its way: it brings the dock out at once.
		p.openNow(gtx.Now)
	}
	_, commenting := u.comments.edited()
	if wake := p.update(gtx.Now, d.holdsKeys(gtx) || d.resize.Dragging() || commenting); !wake.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: wake})
	}
	size := gtx.Constraints.Max
	p.stage.watch(gtx, image.Rectangle{Max: size})
	// The edge is the gutter between the stage and the window's right edge,
	// where the pointer lands when it is thrown to the side.
	p.edge.watch(gtx, image.Rect(size.X, 0, size.X+gtx.Dp(gutter), size.Y))
	share := p.slide.at(gtx, p.shown)
	if share == 0 {
		return nil
	}
	// It slides in as the expanded dock does (stageRow), over the stage
	// rather than beside it.
	w := d.peekWidth(gtx, size.X)
	x := size.X - int(share*float32(w+gtx.Dp(gutter))) + gtx.Dp(gutter)
	p.box = image.Rectangle{Min: image.Pt(x, 0), Max: image.Pt(max(min(x+w, size.X), x), size.Y)}.Add(u.islandAt)
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(x, 0)).Push(gtx.Ops).Pop()
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
	in := dockInput{sessionID: v.in.sessionID, inject: u.injector(v.in.sessionID), submit: u.terms.submitter(v.in.sessionID), checkout: v.in.path, git: v.in.git, isGit: v.in.isGit, light: u.light, code: u.code, placed: placedIn(v.in.placed, extension.PlaceDock)}
	save, err := u.dock.layout(gtx, u.theme, &u.root, in)
	u.applyExtension(u.dock.takeExtensionAction())
	if save {
		rem := u.dock.widthRem
		go func() { u.logErr("dock width", saveDockWidth(u.ctx, u.client, rem)) }()
	}
	return err
}
