package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// clickFrames lays a tree out through a frameRig, waits for its first frame
// to show rows, clicks the top row and reports the rows the frame that took
// the click drew, and those of the frame after it.
func clickFrames(t *testing.T, list *widget.List, draw func(C, *ui.Theme)) (took, next int) {
	t.Helper()
	r := newFrameRig(image.Pt(200, 300), draw)
	deadline := time.Now().Add(2 * time.Second)
	for r.frame(); list.List.Position.Count == 0; r.frame() {
		if time.Now().After(deadline) {
			t.Fatal("the tree never drew a row")
		}
		time.Sleep(time.Millisecond)
	}
	var rows []int
	r.draw = func(gtx C, th *ui.Theme) {
		draw(gtx, th)
		rows = append(rows, list.List.Position.Count)
	}
	r.click(t, f32.Pt(20, 8))
	return rows[len(rows)-2], rows[len(rows)-1]
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
