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
	"gioui.org/widget"
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
	// scroll holds the content of a panel taller than the room it has, when the
	// content did not keep to the height it was given.
	scroll widget.List
	box    image.Rectangle
}

// Box is where the panel last stood, in window coordinates: empty until it has
// been drawn.
func (s *DropdownPanelState) Box() image.Rectangle { return s.box }

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
	// MaxHeight caps the panel's height, 0 for the room it has alone. Either
	// way Content is laid out with the height it may take as its maximum, and
	// scrolls inside it if it asks for more.
	MaxHeight unit.Dp
	above     bool
	span      bool
	theme     *Theme
}

// Above makes the panel prefer the room above its trigger to the room below.
func (p DropdownPanelStyle) Above() DropdownPanelStyle { p.above = true; return p }

// Spanning makes the panel as wide as its trigger, on the same left and right
// edges and following it as it resizes; Width is then ignored.
func (p DropdownPanelStyle) Spanning() DropdownPanelStyle { p.span = true; return p }

func (th *Theme) DropdownPanel(root *Root, state *DropdownPanelState, width unit.Dp, content func(gtx C, th *Theme) D) DropdownPanelStyle {
	return DropdownPanelStyle{Root: root, State: state, Width: width, Content: content, theme: th}
}

// Layout lays out trigger, and the panel when open, which a press on the
// trigger opens and a press outside it or Escape closes.
func (p DropdownPanelStyle) Layout(gtx C, trigger layout.Widget) D {
	s := p.State
	dims := menuLayoutTrigger(gtx, &s.trigger, func(gtx C) D { return hand(gtx, trigger) })
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
	p.theme.Overlays.count()
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
	if p.span {
		p.Width = gtx.Metric.PxToDp(s.anchor.Dx())
	}
	core := menuCore{th: p.theme}
	on := p.theme.On(p.theme.Popover)
	place := dropdownGeometry.root
	if p.above {
		place.side = SideTop
	}
	window := image.Rectangle{Max: p.Root.window}
	maxH := p.Root.window.Y
	if p.MaxHeight > 0 {
		maxH = min(maxH, gtx.Dp(p.MaxHeight))
	}
	size, call := p.measure(gtx, core, on, maxH)
	anchor := s.anchor
	if p.AlignEnd {
		anchor.Min.X = anchor.Max.X - size.X
	}
	// A panel taller than the room on its side is laid out again in what the
	// room is, so its content holds to it, never moved over the trigger.
	if lim := place.limit(gtx, anchor, size, window); size.Y > lim.Y {
		size, call = p.measure(gtx, core, on, min(maxH, lim.Y))
	}
	anchor = s.anchor
	if p.AlignEnd {
		anchor.Min.X = anchor.Max.X - size.X
	}
	box, side := place.box(gtx, anchor, size, window)
	s.box = box
	p.Root.popups = append(p.Root.popups, box)
	area := clip.Rect(box).Push(gtx.Ops)
	event.Op(gtx.Ops, &s.panel)
	area.Pop()
	defer op.Offset(box.Min).Push(gtx.Ops).Pop()
	defer menuEnter(gtx, &s.panel, side, s.anchor.Min.Sub(box.Min), size)()
	call.Add(gtx.Ops)
}

// measure records the panel, no taller than maxH, and answers its size.
func (p DropdownPanelStyle) measure(gtx C, core menuCore, on *Theme, maxH int) (image.Point, op.CallOp) {
	s := p.State
	macro := op.Record(gtx.Ops)
	size := p.panel(gtx, core, on, maxH)
	if !s.open {
		// A row closed the panel while it was drawn, open, in this frame.
		gtx.Execute(op.InvalidateCmd{})
	}
	return size, macro.Stop()
}

// panel draws the surface and the content in it, and answers its size. The
// content gets maxH less the padding as its height; one that is taller anyway
// is put in a scroll area.
func (p DropdownPanelStyle) panel(gtx C, core menuCore, on *Theme, maxH int) image.Point {
	pad := gtx.Dp(menuPanelPad)
	inner := gtx
	inner.Constraints = layout.Exact(image.Pt(gtx.Dp(p.Width)-2*pad, 0))
	inner.Constraints.Max.Y = max(maxH-2*pad, 0)
	macro := op.Record(gtx.Ops)
	d := p.Content(inner, on)
	content := macro.Stop()
	if d.Size.Y > inner.Constraints.Max.Y {
		p.State.scroll.Axis = layout.Vertical
		macro = op.Record(gtx.Ops)
		d = on.ThinScrollArea(&p.State.scroll).Layout(inner, 1, func(gtx C, _ int) D { return p.Content(gtx, on) })
		content = macro.Stop()
	}
	size := image.Pt(gtx.Dp(p.Width), d.Size.Y+2*pad)
	core.drawSurface(gtx, size)
	defer op.Offset(image.Pt(pad, pad)).Push(gtx.Ops).Pop()
	content.Add(gtx.Ops)
	return size
}

// PopoverPanel draws content on the surface a DropdownPanel draws, inside its
// p-1, as wide as the room it is given: for a panel with no trigger that
// takes no focus, as a list over a field that keeps the keyboard. The caller
// places it.
func (th *Theme) PopoverPanel(gtx C, content func(gtx C, th *Theme) D) D {
	pad := gtx.Dp(menuPanelPad)
	inner := gtx
	inner.Constraints = layout.Exact(image.Pt(max(gtx.Constraints.Max.X-2*pad, 0), 0))
	inner.Constraints.Max.Y = max(gtx.Constraints.Max.Y-2*pad, 0)
	macro := op.Record(gtx.Ops)
	d := content(inner, th.On(th.Popover))
	call := macro.Stop()
	size := image.Pt(gtx.Constraints.Max.X, d.Size.Y+2*pad)
	menuCore{th: th}.drawSurface(gtx, size)
	defer op.Offset(image.Pt(pad, pad)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return D{Size: size}
}
