package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// SplitPane is a ratio split with a draggable gap between its two sides, its
// drag state kept across frames. While a drag is on, Rects uses its ratio and
// not the stored one; the caller stores the ratio Divider reports on release.
type SplitPane struct {
	// MinRatio and MaxRatio bound a drag; a zero MaxRatio means 1.
	MinRatio, MaxRatio float64

	dragging          bool
	press             float32
	startRatio, ratio float64
}

// Rects divides r along axis at ratio, gap pixels apart, keeping each side
// its smallest size while r has room for both.
func (s *SplitPane) Rects(r image.Rectangle, axis layout.Axis, ratio float64, gap, minFirst, minSecond int) (first, second, divider image.Rectangle) {
	if s.dragging {
		ratio = s.ratio
	}
	span := r.Dy() - gap
	if axis == layout.Horizontal {
		span = r.Dx() - gap
	}
	at := int(ratio * float64(span))
	if minFirst+minSecond <= span {
		at = min(max(at, minFirst), span-minSecond)
	}
	if axis == layout.Horizontal {
		x := r.Min.X + at
		return image.Rect(r.Min.X, r.Min.Y, x, r.Max.Y), image.Rect(x+gap, r.Min.Y, r.Max.X, r.Max.Y), image.Rect(x, r.Min.Y, x+gap, r.Max.Y)
	}
	y := r.Min.Y + at
	return image.Rect(r.Min.X, r.Min.Y, r.Max.X, y), image.Rect(r.Min.X, y+gap, r.Max.X, r.Max.Y), image.Rect(r.Min.X, y, r.Max.X, y+gap)
}

// Divider runs this frame's drag over divider, inside r, and makes it the
// target of the next one. released is true on the frame the drag ended;
// the ratio returned is then the one to store.
func (s *SplitPane) Divider(gtx C, r, divider image.Rectangle, axis layout.Axis, ratio float64) (next float64, released bool) {
	span := float32(r.Dy() - divider.Dy())
	cursor := pointer.CursorRowResize
	if axis == layout.Horizontal {
		span = float32(r.Dx() - divider.Dx())
		cursor = pointer.CursorColResize
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: s, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && span > 0 {
			released = s.apply(e, axis, ratio, span) || released
		}
	}
	defer clip.Rect(divider).Push(gtx.Ops).Pop()
	cursor.Add(gtx.Ops)
	event.Op(gtx.Ops, s)
	return s.ratio, released
}

func (s *SplitPane) apply(e pointer.Event, axis layout.Axis, ratio float64, span float32) (released bool) {
	at := e.Position.Y
	if axis == layout.Horizontal {
		at = e.Position.X
	}
	switch e.Kind {
	case pointer.Press:
		s.dragging, s.press, s.startRatio, s.ratio = true, at, ratio, ratio
	case pointer.Drag:
		if s.dragging {
			s.ratio = s.clamp(s.startRatio + float64((at-s.press)/span))
		}
	case pointer.Release:
		if s.dragging {
			s.dragging, released = false, true
		}
	case pointer.Cancel:
		s.dragging = false
	}
	return released
}

func (s *SplitPane) clamp(r float64) float64 {
	hi := s.MaxRatio
	if hi == 0 {
		hi = 1
	}
	return min(hi, max(s.MinRatio, r))
}

// Layout splits the constraint's maximum between first and second, storing
// the dragged ratio in *ratio on release.
func (s *SplitPane) Layout(gtx C, axis layout.Axis, ratio *float64, gap, minFirst, minSecond int, first, second layout.Widget) D {
	r := image.Rectangle{Max: gtx.Constraints.Max}
	a, b, divider := s.Rects(r, axis, *ratio, gap, minFirst, minSecond)
	place := func(rect image.Rectangle, w layout.Widget) {
		defer op.Offset(rect.Min).Push(gtx.Ops).Pop()
		g := gtx
		g.Constraints = layout.Exact(rect.Size())
		w(g)
	}
	place(a, first)
	place(b, second)
	if next, released := s.Divider(gtx, r, divider, axis, *ratio); released {
		*ratio = next
	}
	return D{Size: r.Max}
}
