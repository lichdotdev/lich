package main

import (
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

// A surface grows in over panelEnter from the frame that opens it, a change
// of what it shows fades in, and shut then opened again it grows in anew.
func TestPanelMotionGrowsInAndFadesAChange(t *testing.T) {
	var m panelMotion
	t0 := time.UnixMilli(1000)
	at := func(now time.Time) float32 { return m.pose(layout.Context{Ops: new(op.Ops), Now: now}) }
	m.track(t0, true, "a")
	if p := at(t0); p > 1e-6 {
		t.Errorf("pose on the opening frame = %v, want 0", p)
	}
	if p := at(t0.Add(panelEnter / 2)); p <= 0 || p >= 1 {
		t.Errorf("pose halfway = %v, want between 0 and 1", p)
	}
	if p := at(t0.Add(panelEnter)); p != 1 {
		t.Errorf("pose at the end = %v, want 1", p)
	}
	later := t0.Add(time.Second)
	m.track(later, true, "a")
	if !m.faded.IsZero() {
		t.Error("the same content faded")
	}
	m.track(later, true, "b")
	if m.faded != later {
		t.Error("a change of content did not fade")
	}
	m.track(later, false, "b")
	m.track(later.Add(time.Second), true, "b")
	if m.at != later.Add(time.Second) {
		t.Error("opened again, it did not grow in anew")
	}
}
