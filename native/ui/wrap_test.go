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
)

// box is a child of a fixed size that answers presses, so where a Wrap put it
// is read back through the router, with no GPU.
type box struct{ size image.Point }

func (b *box) layout(gtx C) D {
	defer clip.Rect{Max: b.size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, b)
	return D{Size: b.size}
}

// wrapRig lays a Wrap of boxes out under constraints and reports which box a
// press at a point lands on, -1 for none.
type wrapRig struct {
	wrap   Wrap
	cs     layout.Constraints
	boxes  []*box
	auto   int // index of the ml-auto child, -1 for none
	router input.Router
}

func newWrapRig(wrap Wrap, cs layout.Constraints, sizes ...image.Point) *wrapRig {
	r := &wrapRig{wrap: wrap, cs: cs, auto: -1}
	for _, s := range sizes {
		r.boxes = append(r.boxes, &box{size: s})
	}
	return r
}

func (r *wrapRig) frame() (D, int) {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Source: r.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: r.cs}
	kids := make([]WrapChild, len(r.boxes))
	for i, b := range r.boxes {
		kids[i] = WrapItem(b.layout)
		if i == r.auto {
			kids[i] = WrapMLAuto(b.layout)
		}
	}
	d := r.wrap.Layout(gtx, kids...)
	hit := -1
	for i, b := range r.boxes {
		for {
			ev, ok := gtx.Event(pointer.Filter{Target: b, Kinds: pointer.Press})
			if !ok {
				break
			}
			if _, ok := ev.(pointer.Event); ok {
				hit = i
			}
		}
	}
	r.router.Frame(&ops)
	return d, hit
}

func (r *wrapRig) at(p image.Point) int {
	r.frame()
	r.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(p.X), float32(p.Y))})
	_, hit := r.frame()
	r.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(float32(p.X), float32(p.Y))})
	r.frame()
	return hit
}

func upTo(w int) layout.Constraints { return layout.Constraints{Max: image.Pt(w, 1000)} }

func fullWidth(w int) layout.Constraints {
	return layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, 1000)}
}

func TestWrapBreaksOntoANewLineWhenTheWidthRunsOut(t *testing.T) {
	r := newWrapRig(Wrap{GapX: 10, GapY: 4}, upTo(100), image.Pt(40, 20), image.Pt(40, 20), image.Pt(40, 20))
	if d, _ := r.frame(); d.Size != image.Pt(100, 44) {
		t.Errorf("got %v, want two lines filling the width: 100x(20+4+20)", d.Size)
	}
	if got := r.at(image.Pt(10, 34)); got != 2 {
		t.Errorf("start of the second line holds box %d, want 2", got)
	}
}

func TestWrapOnOneLineIsAsWideAsItsChildren(t *testing.T) {
	r := newWrapRig(Wrap{GapX: 10}, upTo(500), image.Pt(40, 20), image.Pt(40, 20))
	if d, _ := r.frame(); d.Size != image.Pt(90, 20) {
		t.Errorf("got %v, want 90x20", d.Size)
	}
}

func TestWrapCentersEachChildOnItsLine(t *testing.T) {
	r := newWrapRig(Wrap{GapX: 10}, upTo(500), image.Pt(40, 10), image.Pt(40, 30))
	if got := r.at(image.Pt(5, 5)); got != -1 {
		t.Errorf("above the short box hit %d, want nothing", got)
	}
	if got := r.at(image.Pt(5, 15)); got != 0 {
		t.Errorf("middle of the line hit %d, want the short box", got)
	}
}

func TestWrapJustify(t *testing.T) {
	end := newWrapRig(Wrap{Justify: JustifyEnd}, fullWidth(200), image.Pt(40, 20))
	if got := end.at(image.Pt(180, 10)); got != 0 {
		t.Errorf("justify-end: right edge hit %d, want the box", got)
	}
	between := newWrapRig(Wrap{Justify: JustifyBetween}, fullWidth(200), image.Pt(40, 20), image.Pt(40, 20), image.Pt(40, 20))
	for at, want := range map[int]int{20: 0, 100: 1, 180: 2, 60: -1} {
		if got := between.at(image.Pt(at, 10)); got != want {
			t.Errorf("justify-between: x=%d hit %d, want %d", at, got, want)
		}
	}
}

// FooterBar: on a line of its own the right section hugs the right edge
// through ml-auto, where justify-between alone puts a lone item at the start.
func TestWrapMLAutoHoldsTheEndOnALineOfItsOwn(t *testing.T) {
	plain := newWrapRig(Wrap{Justify: JustifyBetween, GapY: 4}, fullWidth(100), image.Pt(70, 20), image.Pt(70, 20))
	if got := plain.at(image.Pt(10, 34)); got != 1 {
		t.Errorf("without ml-auto the wrapped box starts its line; hit %d", got)
	}
	auto := newWrapRig(Wrap{Justify: JustifyBetween, GapY: 4}, fullWidth(100), image.Pt(70, 20), image.Pt(70, 20))
	auto.auto = 1
	if got := auto.at(image.Pt(10, 34)); got != -1 {
		t.Errorf("ml-auto: start of the second line hit %d, want nothing", got)
	}
	if got := auto.at(image.Pt(90, 34)); got != 1 {
		t.Errorf("ml-auto: end of the second line hit %d, want the box", got)
	}
}

// FooterBar's min-h-11: its one line is centered in the height, where a
// zone's content-start keeps the line at the top.
func TestWrapMinimumHeightStretchesTheLinesUnlessContentStart(t *testing.T) {
	cs := layout.Constraints{Min: image.Pt(0, 44), Max: image.Pt(500, 1000)}
	stretch := newWrapRig(Wrap{}, cs, image.Pt(40, 20))
	if d, _ := stretch.frame(); d.Size != image.Pt(40, 44) {
		t.Errorf("got %v, want 40x44", d.Size)
	}
	if got := stretch.at(image.Pt(5, 5)); got != -1 {
		t.Errorf("stretched: top hit %d, want the line centered below it", got)
	}
	if got := stretch.at(image.Pt(5, 22)); got != 0 {
		t.Errorf("stretched: middle hit %d, want the box", got)
	}
	start := newWrapRig(Wrap{ContentStart: true}, cs, image.Pt(40, 20))
	if got := start.at(image.Pt(5, 5)); got != 0 {
		t.Errorf("content-start: top hit %d, want the box", got)
	}
}
