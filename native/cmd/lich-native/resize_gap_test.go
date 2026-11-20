package main

import (
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
)

// dragFrom presses at from, moves by dx a frame at a time and releases.
func dragFrom(r *frameRig, from f32.Point, dx float32) {
	r.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: from},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: from},
	)
	r.frame()
	to := from.Add(f32.Pt(dx, 0))
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: to})
	r.frame()
	r.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: to})
	r.frame()
}

// hoverPaintsNothing fails t if the pointer resting at p changes what the
// frame paints.
func hoverPaintsNothing(t *testing.T, r *frameRig, p f32.Point) {
	t.Helper()
	still := r.settle(t)
	r.hover(p)
	if r.settle(t) != still {
		t.Errorf("the pointer resting at %v changed what is painted", p)
	}
}

func TestSidebarResizesFromTheGapBesideItAndPaintsNoStrip(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	s.open = true
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1}}
	w, g := s.boxWidth(gtx, true), gtx.Dp(gutter)
	gap := f32.Pt(float32(w+g/2), 300)
	hoverPaintsNothing(t, r, gap)
	// Where the strip used to light up, on the panel's own edge.
	hoverPaintsNothing(t, r, f32.Pt(float32(w-2), 300))
	dragFrom(r, f32.Pt(float32(w-2), 300), 40)
	if got := s.boxWidth(gtx, true); got != w {
		t.Fatalf("a drag inside the panel's edge resized it to %d, want the %d it had", got, w)
	}
	dragFrom(r, gap, 40)
	if got := s.boxWidth(gtx, true); got != w+40 {
		t.Errorf("a drag from the gap left the sidebar %dpx wide, want %d", got, w+40)
	}
}

func TestDockResizesFromTheGapBesideItAndPaintsNoStrip(t *testing.T) {
	u, r := stageFrames(t, dockReview)
	r.waitFor(t, "No uncommitted changes")
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1}}
	w, g := u.dock.width(gtx), gtx.Dp(gutter)
	at := rigWindow.X - w
	gap := f32.Pt(float32(at-g/2), 300)
	hoverPaintsNothing(t, r, gap)
	hoverPaintsNothing(t, r, f32.Pt(float32(at+2), 300))
	dragFrom(r, f32.Pt(float32(at+2), 300), -40)
	if got := u.dock.width(gtx); got != w {
		t.Fatalf("a drag inside the dock's edge resized it to %d, want the %d it had", got, w)
	}
	dragFrom(r, gap, -40)
	if got := u.dock.width(gtx); got != w+40 {
		t.Errorf("a drag from the gap left the dock %dpx wide, want %d", got, w+40)
	}
}
