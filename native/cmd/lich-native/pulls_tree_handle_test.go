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

func TestTreeHandleSitsPastTheRuleClearOfTheTreesScrollbar(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	press := func(x float32) bool {
		var v pullView
		v.tree.laidOut = 200
		var router input.Router
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(800, 400))}
		v.tree.resize.Update(gtx, 200, 100, 600)
		v.treeHandle(gtx, th)
		router.Frame(&ops)
		router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, 100)})
		v.tree.resize.Update(gtx, 200, 100, 600)
		return v.tree.resize.Dragging()
	}
	if press(197) {
		t.Error("a press on the tree's own right edge, where its scrollbar runs, grabbed the handle")
	}
	if !press(204) {
		t.Error("a press just past the rule missed the handle")
	}
}

func TestTreeFileClickBringsItsCardToTheTop(t *testing.T) {
	p := &reviewPanel{order: []string{"a.go", "b.go", "c.go"}}
	p.scrollToFile("c.go")
	if p.list.Position.First != 2 || p.list.Position.Offset != 0 {
		t.Errorf("position = %+v, want c.go's card first", p.list.Position)
	}
	p.split = true
	p.scrollToFile("b.go")
	if p.list.Position.First != 2 {
		t.Errorf("squeezed split position = %+v, want past the notice row", p.list.Position)
	}
}

func TestViewedOnACardScrolledIntoBringsTheNextFileUnderIt(t *testing.T) {
	p := &reviewPanel{order: []string{"a.go", "b.go", "c.go"}, pr: &prFiles{viewed: map[string]string{}}}
	b := &fileCard{open: true}
	b.git.path = "b.go"
	p.list.Position.First, p.list.Position.Offset = 1, 900
	p.markViewed(b)
	if b.open || p.list.Position.First != 1 || p.list.Position.Offset != 0 {
		t.Errorf("open %v, position = %+v, want b.go folded at the top", b.open, p.list.Position)
	}

	c := &fileCard{open: true}
	c.git.path = "c.go"
	p.list.Position.First, p.list.Position.Offset = 0, 40
	p.markViewed(c)
	if p.list.Position.First != 0 || p.list.Position.Offset != 40 {
		t.Errorf("position = %+v, want a card in full view to leave the scroll alone", p.list.Position)
	}
}

func TestFoldingACardScrolledIntoBringsTheNextFileUnderIt(t *testing.T) {
	p := &reviewPanel{order: []string{"a.go", "b.go"}}
	b := &fileCard{open: true}
	b.git.path = "b.go"
	p.list.Position.First, p.list.Position.Offset = 1, 900
	p.fold(b)
	if b.open || p.list.Position.First != 1 || p.list.Position.Offset != 0 {
		t.Errorf("open %v, position = %+v, want b.go folded at the top", b.open, p.list.Position)
	}
}
