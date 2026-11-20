package main

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/gioterm"
)

// keyboardOf is a renderer that draws nothing and remembers whether the last
// snapshot it was handed has the keyboard.
type keyboardOf struct {
	fixedCells
	focused bool
}

func (k *keyboardOf) Draw(_ layout.Context, s gioterm.Snapshot) (int, error) {
	k.focused = s.Focused()
	return 0, nil
}

// Of the terminals on stage only the one that has the keyboard draws a full
// cursor; the others, and every one of them while the window is out of focus,
// draw an outline.
func TestOnlyTheTerminalWithTheKeyboardDrawsTheCursor(t *testing.T) {
	a, b := newMouseTestView(t, "a"), newMouseTestView(t, "b")
	ra, rb := &keyboardOf{}, &keyboardOf{}
	a.r, a.tag, b.r, b.tag = ra, new(int), rb, new(int)
	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	ts := &terminals{unzoomed: metric}
	size := image.Pt(80*mouseTestCellW+400, 24*mouseTestCellH+400)
	var router input.Router
	var ops op.Ops
	frames := func(takesA, takesB bool) {
		for range 3 {
			ops.Reset()
			gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: metric, Constraints: layout.Exact(size)}
			if err := ts.layout(gtx, a, takesA, backdropLayer{}); err != nil {
				t.Fatal(err)
			}
			if err := ts.layout(gtx, b, takesB, backdropLayer{}); err != nil {
				t.Fatal(err)
			}
			router.Frame(&ops)
		}
	}
	check := func(when string, wantA, wantB bool) {
		t.Helper()
		if ra.focused != wantA || rb.focused != wantB {
			t.Errorf("%s: terminals have the keyboard a=%v b=%v, want a=%v b=%v", when, ra.focused, rb.focused, wantA, wantB)
		}
	}
	frames(true, false)
	check("a in focus", true, false)
	frames(false, true)
	check("focus moved to b", false, true)
	router.Queue(key.FocusEvent{Focus: false})
	frames(false, true)
	check("window out of focus", false, false)
	router.Queue(key.FocusEvent{Focus: true})
	frames(false, true)
	check("window back in focus", false, true)
}
