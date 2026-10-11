package ui

import (
	"image"
	"testing"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// resizeRig is a panel whose width the handle on its edge drives, laid out
// from x = 0 when the handle is on its right edge, and ending at x = 400
// when it is on its left.
type resizeRig struct {
	motionRig
	handle   ResizeHandle
	width    unit.Dp
	released bool
	// placedFirst places the panel from the width it had before Update,
	// as a row that sizes its other children first does.
	placedFirst bool
	widths      []unit.Dp
}

func (r *resizeRig) frame() {
	r.motionRig.frame(func(gtx C) D {
		gtx.Constraints = layout.Constraints{Max: image.Pt(400, 300)}
		w := gtx.Dp(r.width)
		next, released := r.handle.Update(gtx, r.width, 100, 300)
		r.width, r.released = next, r.released || released
		r.widths = append(r.widths, r.width)
		if !r.placedFirst {
			w = gtx.Dp(r.width)
		}
		x := 0
		if r.handle.Edge == PanelEdgeLeft {
			x = 400 - w
		}
		defer op.Offset(image.Pt(x, 0)).Push(gtx.Ops).Pop()
		gtx.Constraints = layout.Exact(image.Pt(w, 300))
		return r.handle.Layout(gtx, testTheme())
	})
}

func (r *resizeRig) drag(from image.Point, xs ...int) {
	r.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(from)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(from)},
	)
	r.frame()
	for _, x := range xs {
		r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(image.Pt(x, from.Y))})
		r.frame()
	}
}

func (r *resizeRig) release(x int) {
	r.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pt(image.Pt(x, 100))})
	r.frame()
}

// The handle moves with the edge it drags, and the width keeps following
// the pointer through it.
func TestResizeHandleFollowsThePointer(t *testing.T) {
	r := &resizeRig{width: 200}
	r.frame()
	r.drag(image.Pt(197, 100), 217, 247)
	if r.width != 250 || !r.handle.Dragging() {
		t.Errorf("dragged 50px right: width %v dragging %v, want 250 and dragging", r.width, r.handle.Dragging())
	}
	r.release(237)
	if r.width != 240 || !r.released || r.handle.Dragging() {
		t.Errorf("released 40px right: width %v released %v, want 240 and released", r.width, r.released)
	}
}

func TestResizeHandleClampsTheWidth(t *testing.T) {
	r := &resizeRig{width: 200}
	r.frame()
	r.drag(image.Pt(197, 100), 390)
	if r.width != 300 {
		t.Errorf("past max: width %v, want 300", r.width)
	}
	r.drag(image.Pt(297, 100), 0)
	if r.width != 100 {
		t.Errorf("past min: width %v, want 100", r.width)
	}
}

func TestResizeHandleOnTheLeftEdgeGrowsLeftward(t *testing.T) {
	r := &resizeRig{width: 200}
	r.handle.Edge = PanelEdgeLeft
	r.frame()
	r.drag(image.Pt(202, 100), 182, 152)
	if r.width != 250 {
		t.Errorf("dragged 50px left: width %v, want 250", r.width)
	}
}

// A left-edge panel placed before its width is read lays out one frame
// behind the drag; the width still tracks the pointer, step by step.
func TestResizeHandleOnTheLeftEdgeFollowsAPanelPlacedFirst(t *testing.T) {
	r := &resizeRig{width: 200, placedFirst: true}
	r.handle.Edge = PanelEdgeLeft
	r.frame()
	xs := []int{198, 194, 190, 186, 182, 178, 174, 170}
	r.drag(image.Pt(202, 100), xs...)
	got := r.widths[len(r.widths)-len(xs):]
	for i, x := range xs {
		if want := unit.Dp(200 + 202 - x); got[i] != want {
			t.Fatalf("widths %v while dragging left through %v: step %d is %v, want %v", got, xs, i, got[i], want)
		}
	}
}

func TestResizeHandleIgnoresAPressOffTheStrip(t *testing.T) {
	r := &resizeRig{width: 200}
	r.frame()
	r.drag(image.Pt(150, 100), 250)
	if r.width != 200 {
		t.Errorf("pressed inside the panel: width %v, want 200", r.width)
	}
}

func TestChipClickKeepsTheFocusWhereItWas(t *testing.T) {
	var r motionRig
	var click widget.Clickable
	clicked, th := false, testTheme()
	chip := func(gtx C) D {
		clicked = clicked || click.Clicked(gtx)
		return th.Chip(&click, "Working", 3, false).Layout(gtx)
	}
	r.frame(chip)
	r.click(image.Pt(5, 5))
	r.frame(chip)
	focused := layout.Context{Source: r.router.Source()}.Focused(&click)
	if !clicked || focused {
		t.Errorf("click: clicked %v focused %v, want a click and no focus", clicked, focused)
	}
}
