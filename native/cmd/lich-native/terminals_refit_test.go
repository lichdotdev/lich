package main

import (
	"image"
	"testing"
	"time"
)

func TestTerminalRefitsOnceTheAreaHoldsStill(t *testing.T) {
	var tv termView
	t0 := time.Unix(1000, 0)
	if due, at := tv.refitDue(image.Pt(800, 600), t0); due || !at.Equal(t0.Add(refitSettle)) {
		t.Fatalf("first new size: due %v at %v, want a wait until %v", due, at, t0.Add(refitSettle))
	}
	t1 := t0.Add(refitSettle / 2)
	if due, at := tv.refitDue(image.Pt(810, 600), t1); due || !at.Equal(t1.Add(refitSettle)) {
		t.Fatalf("size moved again: due %v at %v, want the wait restarted to %v", due, at, t1.Add(refitSettle))
	}
	if due, _ := tv.refitDue(image.Pt(810, 600), t1.Add(refitSettle)); !due {
		t.Fatal("size held for the whole settle: not due, want the refit")
	}
}
