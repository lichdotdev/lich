package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/gioterm"
)

// fixedCells is a renderer whose cells are mouseTestCellW×mouseTestCellH
// and that draws nothing.
type fixedCells struct{}

func (fixedCells) Measure(layout.Context) error { return nil }
func (fixedCells) Cell() (int, int)             { return mouseTestCellW, mouseTestCellH }
func (fixedCells) Draw(layout.Context, gioterm.Snapshot) (int, error) {
	return 0, nil
}
func (fixedCells) SetSize(unit.Sp) {}
func (fixedCells) Release()        {}

// A pointer over a cell of the laid-out terminal selects that cell, wherever
// the padding puts the grid in the window.
func TestDragSelectsTheCellsUnderThePointer(t *testing.T) {
	v := newMouseTestView(t, "hello world")
	v.r, v.tag = fixedCells{}, new(int)
	metric := unit.Metric{PxPerDp: 1.5, PxPerSp: 1.5}
	ts := &terminals{unzoomed: metric}
	gtx := layout.Context{Metric: metric}
	pad := image.Pt(gtx.Dp(terminalPadding.Left), gtx.Dp(terminalPadding.Top))
	size := image.Pt(80*mouseTestCellW+pad.X+gtx.Dp(terminalPadding.Right), 24*mouseTestCellH+pad.Y+gtx.Dp(terminalPadding.Bottom))
	var router input.Router
	var ops op.Ops
	frame := func() {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: metric, Constraints: layout.Exact(size)}
		if err := ts.layout(gtx, v, false); err != nil {
			t.Fatal(err)
		}
		router.Frame(&ops)
	}
	onWindow := func(p f32.Point) f32.Point { return p.Add(f32.Pt(float32(pad.X), float32(pad.Y))) }
	frame()
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: onWindow(from(1, 0))},
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: onWindow(at(3, 0))},
	)
	frame()
	if got := v.vt.SelectionText(); got != "ell" {
		t.Fatalf("dragging over cells 1..3 selected %q, want %q", got, "ell")
	}
}
