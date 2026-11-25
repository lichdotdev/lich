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
// the rail runs the expand button's morph over the stage, which keeps its
// width, so the terminal is not resized and nothing the agent printed
// rewraps. Leaving runs the collapse back.
type sidebarPeek struct {
	panelPeek
	rail  hoverArea
	panel hoverArea
	// out is the shape the peek last morphed to: shown, as of the frame's
	// start, so every part of the frame reads the same.
	out bool
	// morph is whether the morph running is the peek's, drawn over the stage.
	morph bool
	// box is where the panel floats this frame, in window pixels, empty
	// while it does not.
	box image.Rectangle
}

// watchRail lays the rail's hover area over the rail's place.
func (s *sidebar) watchRail(gtx C, size image.Point) {
	s.peek.rail.read(gtx)
	s.peek.rail.watch(gtx, image.Rectangle{Max: size})
}

// wide is whether the sidebar shows its panel: open, or out from the rail.
func (s *sidebar) wide() bool { return s.open || s.peek.out }

// floats is whether the sidebar is drawn over the stage this frame, out from
// the rail or morphing to or from it, while the stage keeps the rail's room.
func (s *sidebar) floats() bool {
	return !s.open && (s.peek.out || s.peek.morph && s.morph.active())
}

// peekDecide decides from the pointer whether the collapsed sidebar is out,
// before anything of the sidebar is laid out, and starts the morph toward
// the shape that changes it to.
func (s *sidebar) peekDecide(gtx C) {
	p := &s.peek
	if s.open || s.morph.active() && !p.morph {
		// A collapse lands under the pointer, which has to leave the rail
		// before the panel can come back out.
		p.panelPeek, p.rail, p.panel, p.out, p.morph = panelPeek{muted: !s.open, since: gtx.Now}, hoverArea{}, hoverArea{}, false, false
		return
	}
	p.rail.read(gtx)
	p.panel.read(gtx)
	p.point(gtx.Now, p.rail.over || p.panel.over)
	if wake := p.update(gtx.Now, s.peekInUse()); !wake.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: wake})
	}
	if p.shown != p.out {
		p.out, p.morph = p.shown, true
		s.morph.begin(gtx)
	}
}

// peekLayout draws the sidebar over the stage while it floats, its corner at
// the rail's, at, and as tall as the rail.
func (s *sidebar) peekLayout(gtx C, th *ui.Theme, m *model, active string, at image.Point) sidebarAction {
	p := &s.peek
	p.box = image.Rectangle{}
	if !s.floats() {
		return sidebarAction{}
	}
	size := image.Pt(s.boxWidth(gtx, true), max(gtx.Constraints.Max.Y-at.Y-gtx.Dp(gutter), 0))
	if s.morph.active() {
		size.X = s.morphWidth(gtx)
	}
	p.box = image.Rectangle{Min: at, Max: at.Add(size)}
	defer op.Offset(at).Push(gtx.Ops).Pop()
	// floating over the stage, the island stands at the menus' elevation
	ui.ElevationMid.Paint(gtx, size, gtx.Dp(ui.RadiusLG))
	// The panel's own area takes the pointer from the stage under it, and
	// reads it over every row it draws.
	area := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops)
	event.Op(gtx.Ops, &p.panel)
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, size.Y))
	gtx.Constraints.Min.X = 0
	_, act := s.draw(gtx, th, m, active)
	area.Pop()
	if act.activate != "" || act.launch.kind != "" || act.worktree {
		// The morph back starts with the next frame's decision.
		p.dismiss()
		gtx.Execute(op.InvalidateCmd{})
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
// panel is already where the open sidebar sits. The click is read after the
// rail and the stage were laid out, so the stage makes room on the next
// frame, asked for now.
func (s *sidebar) pinPeek(gtx C, act *sidebarAction) {
	s.open, act.saveOpen = true, true
	gtx.Execute(op.InvalidateCmd{})
}
