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

	"github.com/lichdotdev/lich/native/ui"
)

func TestSidebarHandleSitsInTheGapPastThePanel(t *testing.T) {
	g := layout.Context{Metric: unit.Metric{PxPerDp: 1}}.Dp(gutter)
	press := func(x float32) bool {
		s := newSidebar(new(ui.Root))
		var router input.Router
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(800, 400))}
		s.resize.Update(gtx, 300, 200, 500)
		s.resizeHandle(gtx, image.Pt(300, 400))
		router.Frame(&ops)
		router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, 100)})
		s.resize.Update(gtx, 300, 200, 500)
		return s.resize.Dragging()
	}
	if !press(301) || !press(float32(300+g-1)) {
		t.Error("a press in the gap between the panel and the stage missed the handle")
	}
	if press(297) {
		t.Error("a press on the panel's own edge, where the block list's scrollbar runs, grabbed the handle")
	}
	if press(float32(300 + g + 1)) {
		t.Error("a press on the stage past the gap grabbed the handle")
	}
}
