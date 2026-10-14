package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// PanelEdge is lib/panel-width.ts's: the edge of the panel the handle sits
// on. A right-edge panel is anchored on its left and grows rightward, a
// left-edge one the other way.
type PanelEdge uint8

const (
	PanelEdgeRight PanelEdge = iota
	PanelEdgeLeft
)

// ResizeHandle geometry: `absolute inset-y-2 w-1.5 rounded-full`, on the
// panel's own edge.
var (
	ResizeWidth = Space(1.5)
	resizeInset = Space(2)
)

// ResizeHandle is common/ResizeHandle.tsx with usePanelWidth's drag: the
// strip along a panel's edge, invisible until hovered (hover:bg-accent, held
// while dragging, as pointer capture holds :hover on the web), that drags
// the panel's width. The caller owns the width, and persists it on release.
type ResizeHandle struct {
	Edge PanelEdge

	hovered, dragging bool
	pressX            float32
	startWidth        unit.Dp
	// laidOut is the px width of the box the last Layout drew, the frame
	// Gio transformed this frame's pointer positions in; pressLaidOut is it
	// at the press.
	laidOut, pressLaidOut int
	fill                  tween
}

// Update reads the frame's drag and returns the width it asks for, clamped
// to [minWidth, maxWidth], and whether the drag ended this frame. width is
// the width the panel was laid out with last frame. Call it before laying
// the panel out.
func (r *ResizeHandle) Update(gtx C, width, minWidth, maxWidth unit.Dp) (next unit.Dp, released bool) {
	next = width
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: r, Kinds: pointer.Enter | pointer.Leave | pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			return next, released
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Enter, pointer.Leave:
			r.hovered = e.Kind == pointer.Enter
		case pointer.Press:
			r.dragging, r.pressX, r.startWidth, r.pressLaidOut = true, e.Position.X, width, r.laidOut
		case pointer.Drag, pointer.Release:
			if !r.dragging {
				continue
			}
			next = r.drag(gtx, e.Position.X, minWidth, maxWidth)
			released = e.Kind == pointer.Release
			r.dragging = !released
		case pointer.Cancel:
			r.dragging = false
		}
	}
}

// drag is dragWidth of panel-width.ts. Positions are in the panel's own
// box, whose left edge stays put on a right-edge panel and travels with the
// width on a left-edge one, so that travel is taken back out. It is the
// box's drawn width that says how far, not the width the caller holds: a
// caller may place the panel before Update, and px rounding moves the edge
// by whole pixels.
func (r *ResizeHandle) drag(gtx C, x float32, minWidth, maxWidth unit.Dp) unit.Dp {
	travel := x - r.pressX
	if r.Edge == PanelEdgeLeft {
		travel = -(travel - float32(r.laidOut-r.pressLaidOut))
	}
	next := r.startWidth + unit.Dp(travel/gtx.Metric.PxPerDp)
	return min(max(next, minWidth), maxWidth)
}

// Dragging reports whether a drag is in flight.
func (r *ResizeHandle) Dragging() bool { return r.dragging }

// Layout draws the handle on the edge of the panel box gtx.Constraints.Max,
// in the panel's own coordinates, over whatever the panel drew: lay it out last, as ResizeHandle.tsx comes
// after the panel's content to win the tie on z-10.
func (r *ResizeHandle) Layout(gtx C, th *Theme) D {
	box := gtx.Constraints.Max
	r.laidOut = box.X
	w, inset := gtx.Dp(ResizeWidth), gtx.Dp(resizeInset)
	x := 0
	if r.Edge == PanelEdgeRight {
		x = box.X - w
	}
	strip := image.Rect(x, inset, x+w, max(box.Y-inset, inset))
	target := float32(0)
	if r.hovered || r.dragging {
		target = 1
	}
	// transition-colors: Tailwind's default 150ms, the curve the Switch
	// slides on.
	if lit := r.fill.at(gtx, target, toggleSlideDuration, toggleEase); lit > 0 {
		paint.FillShape(gtx.Ops, th.Over(Alpha(th.Accent, lit)), clip.UniformRRect(strip, w/2).Op(gtx.Ops))
	}
	defer clip.Rect(strip).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, r)
	pointer.CursorColResize.Add(gtx.Ops)
	return D{Size: box}
}
