package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

// DropdownPanelState is a DropdownPanel's open state, owned by the caller like a
// MenuState.
type DropdownPanelState struct {
	open     bool
	focus    bool
	origin   image.Point
	anchor   image.Rectangle
	trigger  struct{ _ byte }
	backdrop popupBackdrop
	panel    menuLevel // only its shown time, for the animate-in
}

// Opened reports whether the panel is showing.
func (s *DropdownPanelState) Opened() bool { return s.open }

// Close hides the panel, as a row of its content does once chosen.
func (s *DropdownPanelState) Close() { s.open = false }

// DropdownPanelStyle is a DropdownMenuContent whose rows the caller draws: the
// same trigger, surface, placement and dismissal as DropdownMenu, for rows a
// MenuEntry cannot say (two lines, a control of their own). Content lays the
// rows out on the popover, inside its p-1, Width wide.
type DropdownPanelStyle struct {
	Root    *Root
	State   *DropdownPanelState
	Width   unit.Dp
	Content func(gtx C, th *Theme) D
	// AlignEnd lines the panel's right edge up with the trigger's
	// (align="end"); the default is its left edge.
	AlignEnd bool
	theme    *Theme
}

func (th *Theme) DropdownPanel(root *Root, state *DropdownPanelState, width unit.Dp, content func(gtx C, th *Theme) D) DropdownPanelStyle {
	return DropdownPanelStyle{Root: root, State: state, Width: width, Content: content, theme: th}
}

// Layout lays out trigger, and the panel when open, which a press on the
// trigger opens and a press outside it or Escape closes.
func (p DropdownPanelStyle) Layout(gtx C, trigger layout.Widget) D {
	s := p.State
	dims := menuLayoutTrigger(gtx, &s.trigger, trigger)
	if at, ok := menuTriggerPress(gtx, &s.trigger, pointer.ButtonPrimary); ok && !s.open {
		origin := p.Root.origin(at)
		s.open, s.focus, s.origin = true, true, origin
		s.anchor = image.Rectangle{Min: origin, Max: origin.Add(dims.Size)}
		s.panel = menuLevel{}
	}
	if !s.open {
		return dims
	}
	if s.backdrop.pressed(gtx) || p.escaped(gtx) {
		s.open = false
		return dims
	}
	macro := op.Record(gtx.Ops)
	p.overlay(gtx)
	op.Defer(gtx.Ops, macro.Stop())
	return dims
}

func (p DropdownPanelStyle) escaped(gtx C) bool {
	escaped := false
	for {
		ev, ok := gtx.Event(key.FocusFilter{Target: &p.State.backdrop}, key.Filter{Focus: &p.State.backdrop, Name: key.NameEscape})
		if !ok {
			return escaped
		}
		if ke, ok := ev.(key.Event); ok && ke.State == key.Press {
			escaped = true
		}
	}
}

// overlay runs deferred, in window coordinates: the modal backdrop, then the
// panel below the trigger.
func (p DropdownPanelStyle) overlay(gtx C) {
	s := p.State
	defer op.Offset(s.origin.Mul(-1)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Constraints{Max: p.Root.window}
	s.backdrop.layout(gtx, p.Root.window)
	if s.focus {
		gtx.Execute(key.FocusCmd{Tag: &s.backdrop})
		s.focus = false
	}
	core := menuCore{th: p.theme}
	on := p.theme.On(p.theme.Popover)
	macro := op.Record(gtx.Ops)
	size := p.panel(gtx, core, on)
	if !s.open {
		// A row closed the panel while it was drawn, open, in this frame.
		gtx.Execute(op.InvalidateCmd{})
	}
	call := macro.Stop()
	anchor := s.anchor
	if p.AlignEnd {
		anchor.Min.X = anchor.Max.X - size.X
	}
	box, side := dropdownGeometry.root.box(gtx, anchor, size, image.Rectangle{Max: p.Root.window})
	area := clip.Rect(box).Push(gtx.Ops)
	event.Op(gtx.Ops, &s.panel)
	area.Pop()
	defer op.Offset(box.Min).Push(gtx.Ops).Pop()
	defer menuEnter(gtx, &s.panel, side, s.anchor.Min.Sub(box.Min), size)()
	call.Add(gtx.Ops)
}

// panel draws the surface and the content in it, and answers its size.
func (p DropdownPanelStyle) panel(gtx C, core menuCore, on *Theme) image.Point {
	pad := gtx.Dp(menuPanelPad)
	inner := gtx
	inner.Constraints = layout.Exact(image.Pt(gtx.Dp(p.Width)-2*pad, 0))
	inner.Constraints.Max.Y = p.Root.window.Y
	macro := op.Record(gtx.Ops)
	d := p.Content(inner, on)
	content := macro.Stop()
	size := image.Pt(gtx.Dp(p.Width), d.Size.Y+2*pad)
	core.drawSurface(gtx, size)
	defer op.Offset(image.Pt(pad, pad)).Push(gtx.Ops).Pop()
	content.Add(gtx.Ops)
	return size
}
