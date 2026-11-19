package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

func TestPopupPlace(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	window := image.Rect(0, 0, 200, 100)
	size := image.Pt(40, 20)
	cases := []struct {
		name     string
		place    popupPlace
		anchor   image.Rectangle
		want     image.Rectangle
		wantSide Side
	}{
		{"top, centered, sideOffset", popupPlace{side: SideTop, sideOffset: 4}, image.Rect(80, 50, 120, 60), image.Rect(80, 26, 120, 46), SideTop},
		{"bottom, start, alignOffset", popupPlace{side: SideBottom, align: alignStart, sideOffset: 4, alignOffset: -3}, image.Rect(80, 20, 120, 30), image.Rect(77, 34, 117, 54), SideBottom},
		{"top flips below", popupPlace{side: SideTop, sideOffset: 4}, image.Rect(80, 10, 120, 20), image.Rect(80, 24, 120, 44), SideBottom},
		{"right flips left", popupPlace{side: SideRight, align: alignStart}, image.Rect(180, 40, 180, 40), image.Rect(140, 40, 180, 60), SideLeft},
		// Neither side has room for the whole popup: it takes the side with the
		// more room, cut to it, and never covers the anchor.
		{"no room either way, a tie keeps the preferred side", popupPlace{side: SideBottom, align: alignStart}, image.Rect(10, 10, 20, 90), image.Rect(10, 90, 50, 95), SideBottom},
		{"not enough below, whole above", popupPlace{side: SideBottom, align: alignStart, sideOffset: 2}, image.Rect(10, 40, 20, 90), image.Rect(10, 18, 50, 38), SideTop},
		{"the preferred side is cut when the opposite has less", popupPlace{side: SideBottom, align: alignStart}, image.Rect(10, 5, 20, 20), image.Rect(10, 20, 50, 40), SideBottom},
		{"clamped on the cross axis", popupPlace{side: SideTop}, image.Rect(0, 50, 10, 60), image.Rect(5, 30, 45, 50), SideTop},
		{"clamped against the far edge", popupPlace{side: SideRight, align: alignStart, alignOffset: 4}, image.Rect(10, 95, 10, 95), image.Rect(10, 75, 50, 95), SideRight},
	}
	for _, tc := range cases {
		box, side := tc.place.box(gtx, tc.anchor, size, window)
		if box != tc.want || side != tc.wantSide {
			t.Errorf("%s: got %v on side %d, want %v on side %d", tc.name, box, side, tc.want, tc.wantSide)
		}
	}
}

// A backdrop laid out over a clickable takes a press outside the popup drawn
// above it, which the clickable never sees; a press on the popup reaches
// neither.
func TestPopupBackdropBlocksAndReportsOutsidePress(t *testing.T) {
	var (
		router   input.Router
		ops      op.Ops
		under    widget.Clickable
		backdrop popupBackdrop
		popup    struct{ _ byte }
	)
	window := image.Pt(100, 100)
	frame := func() (outside, underClicked bool) {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(window)}
		outside, underClicked = backdrop.pressed(gtx), under.Clicked(gtx)
		under.Layout(gtx, func(gtx C) D { return D{Size: window} })
		backdrop.layout(gtx, window)
		area := clip.Rect{Min: image.Pt(40, 40), Max: image.Pt(60, 60)}.Push(gtx.Ops)
		event.Op(gtx.Ops, &popup)
		area.Pop()
		router.Frame(&ops)
		return outside, underClicked
	}
	click := func(at image.Point) {
		pos := f32.Pt(float32(at.X), float32(at.Y))
		router.Queue(
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos},
		)
	}
	frame()
	click(image.Pt(10, 10))
	if outside, clicked := frame(); !outside || clicked {
		t.Errorf("press outside: reported %v, content clicked %v; want reported, not clicked", outside, clicked)
	}
	click(image.Pt(50, 50))
	if outside, clicked := frame(); outside || clicked {
		t.Errorf("press on the popup: reported %v, content clicked %v; want neither", outside, clicked)
	}
}

// A popup taller than the room of either side is cut to the larger one, never
// moved over the anchor, and the limit says how tall it may be.
func TestPopupNeverCoversItsAnchorAndLimitsItsHeight(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	window := image.Rect(0, 0, 200, 400)
	anchor := image.Rect(80, 300, 120, 330)
	place := popupPlace{side: SideTop, sideOffset: 4}
	for _, h := range []int{50, 250, 290, 400, 1000} {
		size := image.Pt(60, h)
		box, side := place.box(gtx, anchor, size, window)
		if !box.Intersect(anchor).Empty() {
			t.Errorf("a popup %dpx tall covers its anchor: %v over %v", h, box, anchor)
		}
		if box.Min.Y < 5 || box.Max.Y > 395 {
			t.Errorf("a popup %dpx tall leaves the window: %v", h, box)
		}
		if lim := place.limit(gtx, anchor, size, window); h > lim.Y && box.Dy() != lim.Y {
			t.Errorf("a popup %dpx tall is %dpx in its box, limit says %d", h, box.Dy(), lim.Y)
		}
		if side == SideTop && box.Max.Y != anchor.Min.Y-4 {
			t.Errorf("a popup %dpx tall on top ends at y=%d, want a gap of 4 over the anchor (y=%d)", h, box.Max.Y, anchor.Min.Y-4)
		}
	}
	if lim := place.limit(gtx, anchor, image.Pt(60, 1000), window); lim.Y != 300-4-5 {
		t.Errorf("limit %d, want the 291px above the anchor", lim.Y)
	}
	// An anchor low in the window: the popup that fits above stays above, and
	// one that fits only below goes below.
	if _, side := place.box(gtx, image.Rect(80, 20, 120, 40), image.Pt(60, 100), window); side != SideBottom {
		t.Errorf("a popup with room only below landed on side %d", side)
	}
	// Beside an anchor the width is what is cut.
	beside := popupPlace{side: SideRight}
	if lim := beside.limit(gtx, image.Rect(150, 100, 190, 130), image.Pt(120, 40), window); lim.X != 145 || lim.Y != 390 {
		t.Errorf("limit %v for a popup beside an anchor near the right edge: the left has the room: 145 wide, and the window inside tall", lim)
	}
}
