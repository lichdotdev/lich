package main

import (
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

func slideAt(s *dockSlide, now time.Time, open bool) float32 {
	return s.at(layout.Context{Ops: new(op.Ops), Now: now}, open)
}

func TestDockSlideEasesInAndTurnsBackFromWhereItIs(t *testing.T) {
	var s dockSlide
	t0 := time.Unix(100, 0)
	if got := slideAt(&s, t0, true); got > 1e-6 {
		t.Fatalf("first frame of an opening: got %v, want 0", got)
	}
	half := slideAt(&s, t0.Add(sidebarMorphDuration/2), true)
	if half <= 0.5 || half >= 1 {
		t.Fatalf("halfway on an ease-out curve: got %v, want past 0.5 and short of 1", half)
	}
	back := t0.Add(sidebarMorphDuration / 2)
	if got := slideAt(&s, back, false); got != half {
		t.Fatalf("a close halfway starts from the share on screen: got %v, want %v", got, half)
	}
	if got := slideAt(&s, back.Add(sidebarMorphDuration), false); got != 0 {
		t.Fatalf("after the slide: got %v, want 0", got)
	}
	if got := slideAt(&s, back.Add(2*sidebarMorphDuration), true); got > 1e-6 {
		t.Fatalf("a new opening starts closed: got %v, want 0", got)
	}
}
