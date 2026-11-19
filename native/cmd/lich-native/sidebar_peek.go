package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// sidebarPeek is the collapsed sidebar brought out: the pointer resting on
// the rail draws the open sidebar over it, and the stage gives it room as it
// does the open one, since a page in the stage is a window of the system's,
// which nothing the window draws can cover.
type sidebarPeek struct {
	panelPeek
	slide dockSlide
	rail  hoverArea
	panel hoverArea
	// width is how far the panel reaches from the rail's left edge this
	// frame, 0 while it is in.
	width int
}

// watchRail lays the rail's hover area over the rail just drawn.
func (s *sidebar) watchRail(gtx C, size image.Point) {
	s.peek.rail.read(gtx)
	s.peek.rail.watch(gtx, image.Rectangle{Max: size})
}

// peekLayout draws the open sidebar over the rail while the collapsed one is
// out, its corner at the rail's, at, and as tall as the rail. It runs before
// the stage, which makes room for it in the same frame (stageReserve).
func (s *sidebar) peekLayout(gtx C, th *ui.Theme, m *model, active string, at image.Point) sidebarAction {
	p := &s.peek
	if s.open || s.morph.active() {
		// A collapse lands under the pointer, which has to leave the rail
		// before the panel can come back out.
		p.panelPeek, p.rail, p.panel, p.width = panelPeek{muted: !s.open, since: gtx.Now}, hoverArea{}, hoverArea{}, 0
		p.slide.snap(false)
		return sidebarAction{}
	}
	p.panel.read(gtx)
	p.point(gtx.Now, p.rail.over || p.panel.over)
	if wake := p.update(gtx.Now, s.peekInUse()); !wake.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: wake})
	}
	share := p.slide.at(gtx, p.shown)
	size := image.Pt(s.boxWidth(gtx, true), max(gtx.Constraints.Max.Y-at.Y-gtx.Dp(gutter), 0))
	p.width = int(share * float32(size.X))
	if share == 0 {
		return sidebarAction{}
	}
	defer op.Offset(at).Push(gtx.Ops).Pop()
	// It slides out from under the rail's left edge.
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(int((share-1)*float32(size.X)), 0)).Push(gtx.Ops).Pop()
	// The panel's own area takes the pointer from the rail under it, and
	// reads it over every row it draws.
	area := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops)
	event.Op(gtx.Ops, &p.panel)
	gtx.Constraints = layout.Exact(size)
	if share < 1 {
		// A moving panel takes no input; the slide asked for its frames.
		gtx = gtx.Disabled()
	}
	_, act := s.panel(gtx, th, m, active)
	area.Pop()
	if act.activate != "" || act.launch.kind != "" || act.worktree {
		p.dismiss()
	}
	return act
}

// peekInUse is whether something in the sidebar is in use, which keeps it out
// after the pointer has left: a menu, a field, a dialog, a drag.
func (s *sidebar) peekInUse() bool {
	if s.holdsKeys() || s.carry.Carrying() != "" || s.sort.Dragging() != "" {
		return true
	}
	for _, m := range s.moreMenus {
		if m.Opened() {
			return true
		}
	}
	return false
}

// collapseToggle runs the header's collapse button and says how to draw it.
// Out over the stage the sidebar is collapsed already, and the same button
// keeps it open instead.
func (s *sidebar) collapseToggle(gtx C, act *sidebarAction) (*icons.Icon, string) {
	if s.header.collapse.Clicked(gtx) {
		if s.open {
			s.setOpen(gtx, false, act)
		} else {
			s.pinPeek(gtx, act)
		}
	}
	if !s.open {
		return icExpand, i18n.T("sidebar.sidebarRail.expand")
	}
	return icCollapse, i18n.T("sidebar.sessionSidebar.collapse")
}

// pinPeek keeps the sidebar that is out open, in place: no morph, since the
// panel is already where the open sidebar sits, and the stage already gives
// it the room.
func (s *sidebar) pinPeek(gtx C, act *sidebarAction) {
	s.open, act.saveOpen = true, true
	gtx.Execute(op.InvalidateCmd{})
}
