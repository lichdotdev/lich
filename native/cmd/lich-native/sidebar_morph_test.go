package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/lichdotdev/lich/native/ui"
)

const testFrame = 16 * time.Millisecond

func TestMorphProgressEasesFromZeroToOne(t *testing.T) {
	// The curve solves by bisection, so its start is zero to within float noise.
	if p := morphProgress(0); p > 1e-6 {
		t.Fatalf("progress at 0 = %v, want 0", p)
	}
	if p := morphProgress(sidebarMorphDuration); p != 1 {
		t.Fatalf("progress at the duration = %v, want 1", p)
	}
	// cubic-bezier(0.2, 0, 0, 1) is past three quarters by half time.
	if p := morphProgress(sidebarMorphDuration / 2); p < 0.75 || p >= 1 {
		t.Fatalf("progress at half time = %v, want in [0.75, 1)", p)
	}
	last := float32(0)
	for d := time.Duration(0); d <= sidebarMorphDuration; d += time.Millisecond {
		p := morphProgress(d)
		if p < last {
			t.Fatalf("progress went back at %v: %v after %v", d, p, last)
		}
		last = p
	}
}

func TestFlyBoxEasesTheCornerAndTakesTheTargetSize(t *testing.T) {
	from := image.Rect(10, 100, 26, 116)
	to := image.Rect(30, 40, 52, 62)
	if got := flyBox(from, to, 0); got.Min != from.Min || got.Size() != to.Size() {
		t.Fatalf("flyBox at 0 = %v, want corner %v at size %v", got, from.Min, to.Size())
	}
	if got := flyBox(from, to, 1); got != to {
		t.Fatalf("flyBox at 1 = %v, want %v", got, to)
	}
	if got := flyBox(from, to, 0.5); got.Min != image.Pt(20, 70) {
		t.Fatalf("flyBox at 0.5 corner = %v, want (20,70)", got.Min)
	}
}

func TestMorphCaptureReadsGlyphBoxesInWindowCoordinates(t *testing.T) {
	m := &sidebarMorph{phase: morphCapture, capturing: true}
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Constraints: layout.Exact(image.Pt(100, 100))}
	glyph := func(gtx C) D { return D{Size: image.Pt(16, 16)} }
	at := func(side morphSide, corner image.Point, name string) {
		defer op.Offset(corner).Push(ops).Pop()
		m.side = side
		m.glyph(gtx, name, glyph)
	}
	at(sideFrom, image.Pt(30, 40), glyphNew)
	at(sideTo, image.Pt(8, 10), glyphNew)
	at(sideTo, image.Pt(8, 50), glyphSession+"s1")
	m.capture(ops)
	if m.phase != morphFlying {
		t.Fatalf("phase after capture = %v, want flying", m.phase)
	}
	want := map[morphSide]map[string]image.Rectangle{
		sideFrom: {glyphNew: image.Rect(30, 40, 46, 56)},
		sideTo:   {glyphNew: image.Rect(8, 10, 24, 26), glyphSession + "s1": image.Rect(8, 50, 24, 66)},
	}
	for side, boxes := range want {
		if len(m.boxes[side]) != len(boxes) {
			t.Fatalf("side %d boxes = %v, want %v", side, m.boxes[side], boxes)
		}
		for name, box := range boxes {
			if m.boxes[side][name] != box {
				t.Fatalf("side %d %s = %v, want %v", side, name, m.boxes[side][name], box)
			}
		}
	}
}

func TestMorphCaptureSkipsTheFrameThatBeganIt(t *testing.T) {
	m := &sidebarMorph{}
	gtx := layout.Context{Ops: new(op.Ops)}
	m.advance(gtx)
	m.begin(gtx)
	m.capture(gtx.Ops)
	if m.phase != morphCapture {
		t.Fatalf("phase = %v, want capture: the click's frame drew no probes", m.phase)
	}
}

func TestMorphGlyphStepsAsideWhileItFlies(t *testing.T) {
	m := &sidebarMorph{phase: morphFlying}
	drawn := false
	d := m.glyph(layout.Context{Ops: new(op.Ops)}, glyphNew, func(gtx C) D {
		drawn = true
		return D{Size: image.Pt(16, 16)}
	})
	if d.Size != image.Pt(16, 16) {
		t.Fatalf("glyph size in flight = %v, want its own room", d.Size)
	}
	// The widget runs to measure itself; only its ops are dropped.
	if !drawn {
		t.Fatal("glyph widget did not run")
	}
}

// The stage's width is what its terminal is fit to, and every new width is a
// resize sent to the PTY: through a whole morph either way it takes the old
// width and the new one and nothing between.
func TestSidebarMorphResizesTheStageOnce(t *testing.T) {
	const window = 1400
	for _, opening := range []bool{true, false} {
		s := newSidebar(new(ui.Root))
		s.open = !opening
		gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1}, Now: time.Unix(0, 0)}
		var widths []int
		frame := func() {
			s.morph.advance(gtx)
			widths = append(widths, window-s.stageReserve(gtx))
			s.morph.capture(gtx.Ops)
			gtx.Now = gtx.Now.Add(testFrame)
		}
		frame()
		before := widths[0]
		s.setOpen(gtx, opening, new(sidebarAction))
		for range 2 + int(sidebarMorphDuration/testFrame) + 2 {
			frame()
		}
		after := window - s.boxWidth(gtx, opening)
		if s.morph.active() {
			t.Fatalf("opening=%v: morph still running after its duration", opening)
		}
		if got := widths[len(widths)-1]; got != after {
			t.Fatalf("opening=%v: final stage width = %d, want %d", opening, got, after)
		}
		changes := 0
		for i, w := range widths {
			if w != before && w != after {
				t.Fatalf("opening=%v: frame %d stage width %d is neither %d nor %d", opening, i, w, before, after)
			}
			if i > 0 && w != widths[i-1] {
				changes++
			}
		}
		if changes != 1 {
			t.Fatalf("opening=%v: stage width changed %d times (%v), want once", opening, changes, widths)
		}
	}
}
