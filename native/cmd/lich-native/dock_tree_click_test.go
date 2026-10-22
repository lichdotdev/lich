package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// clickFrames lays a tree out through draw, waits for its first frame to show
// rows, clicks the top row and reports the rows the frame that took the click
// drew, and those of the frame after it. Gio draws one frame per event, so
// the two differ only when the click landed after the rows were read.
func clickFrames(t *testing.T, list *widget.List, draw func(C, *ui.Theme)) (took, next int) {
	t.Helper()
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
	var router input.Router
	var ops op.Ops
	frame := func() int {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(200, 300)), Source: router.Source(), Now: time.Now()}
		draw(gtx, th)
		router.Frame(&ops)
		return list.List.Position.Count
	}
	deadline := time.Now().Add(2 * time.Second)
	for frame() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the tree never drew a row")
		}
		time.Sleep(time.Millisecond)
	}
	at := f32.Pt(20, 8)
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	took = frame()
	return took, frame()
}

func TestCodeTreeOpensAFolderInTheFrameThatTookItsClick(t *testing.T) {
	c := &codeTab{toggled: map[string]bool{}}
	c.rows.list.Axis = layout.Vertical
	c.tree.data, c.tree.has, c.tree.settled = &treeData{listing: fileListing{Files: []string{"src/a.go", "src/b.go", "c.go"}}}, true, true
	took, next := clickFrames(t, &c.rows.list, func(gtx C, th *ui.Theme) { c.treeBody(gtx, th) })
	if took != 4 {
		t.Errorf("the frame that took the click drew %d rows, want 4 with src open; the next frame drew %d", took, next)
	}
}

func TestPullsTreeFoldsAFolderInTheFrameThatTookItsClick(t *testing.T) {
	v := &pullView{files: &reviewPanel{order: []string{"src/a.go", "src/b.go", "c.go"}}, tree: filesTree{closed: map[string]bool{}}}
	v.tree.rows.list.Axis = layout.Vertical
	took, next := clickFrames(t, &v.tree.rows.list, func(gtx C, th *ui.Theme) { v.treeLayout(gtx, th) })
	if took != 2 {
		t.Errorf("the frame that took the click drew %d rows, want 2 with src folded; the next frame drew %d", took, next)
	}
}
