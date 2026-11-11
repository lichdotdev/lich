package ui

import (
	"image"
	"math"
	"testing"

	"gioui.org/io/pointer"
	"gioui.org/layout"
)

func TestSplitPaneRects(t *testing.T) {
	var s SplitPane
	r := image.Rect(0, 0, 210, 100)
	first, second, gap := s.Rects(r, layout.Horizontal, 0.5, 10, 0, 0)
	if first != image.Rect(0, 0, 100, 100) || second != image.Rect(110, 0, 210, 100) || gap != image.Rect(100, 0, 110, 100) {
		t.Errorf("halves: %v %v %v", first, second, gap)
	}
	if first, _, _ := s.Rects(r, layout.Horizontal, 0.1, 10, 80, 20); first.Dx() != 80 {
		t.Errorf("first side %dpx, want its minimum 80", first.Dx())
	}
	if first, _, _ := s.Rects(r, layout.Horizontal, 0.9, 10, 20, 80); first.Dx() != 120 {
		t.Errorf("first side %dpx, want the room the second side's minimum leaves", first.Dx())
	}
	if _, second, _ := s.Rects(image.Rect(0, 0, 100, 210), layout.Vertical, 0.5, 10, 0, 0); second.Min.Y != 110 {
		t.Errorf("vertical second side starts at %d, want 110", second.Min.Y)
	}
}

func TestSplitPaneMinimumsGiveWayWithoutRoom(t *testing.T) {
	var s SplitPane
	first, _, _ := s.Rects(image.Rect(0, 0, 110, 100), layout.Horizontal, 0.5, 10, 80, 80)
	if first.Dx() != 50 {
		t.Errorf("first side %dpx, want the ratio's 50 when both minimums cannot fit", first.Dx())
	}
}

func TestSplitPaneDragMovesTheRatioAndReportsItOnRelease(t *testing.T) {
	var rig motionRig
	s := SplitPane{MinRatio: 0.05, MaxRatio: 0.95}
	ratio, released := 0.5, false
	frame := func() {
		rig.frame(func(gtx C) D {
			r := image.Rectangle{Max: image.Pt(200, 100)}
			_, _, gap := s.Rects(r, layout.Horizontal, ratio, 10, 0, 0)
			if next, done := s.Divider(gtx, r, gap, layout.Horizontal, ratio); done {
				ratio, released = next, true
			}
			return D{Size: r.Max}
		})
	}
	frame()
	rig.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(image.Pt(98, 50))},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(image.Pt(98, 50))},
	)
	frame()
	rig.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(image.Pt(145, 50))})
	frame()
	if released {
		t.Fatal("reported before the release")
	}
	rig.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pt(image.Pt(145, 50))})
	frame()
	if want := 0.5 + 47.0/190; !released || math.Abs(ratio-want) > 0.01 {
		t.Errorf("released %v, ratio %v, want %v", released, ratio, want)
	}
}
