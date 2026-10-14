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

	"github.com/omartelo/lich/native/ui"
)

func TestSidebarHandleSitsInTheGutterClearOfTheBlockListsScrollbar(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	press := func(x float32) bool {
		s := newSidebar(new(ui.Root))
		var router input.Router
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(800, 400))}
		s.resize.Update(gtx, 300, 200, 500)
		s.resizeHandle(gtx, th, image.Pt(300, 400))
		router.Frame(&ops)
		router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, 100)})
		s.resize.Update(gtx, 300, 200, 500)
		return s.resize.Dragging()
	}
	if press(297) {
		t.Error("a press on the panel's own right edge, where the block list's scrollbar runs, grabbed the handle")
	}
	if !press(303) {
		t.Error("a press in the gutter past the panel missed the handle")
	}
}
