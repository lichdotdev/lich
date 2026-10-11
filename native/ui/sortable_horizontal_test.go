package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// rowRig is a horizontal List of three 40x20 tabs, t1 0-40, t2 40-80 and
// t3 80-120: the project tab strip's shape.
type rowRig struct {
	router input.Router
	s      Sortable
	ids    []string
	size   image.Point
	drops  []SortDrop
}

func (r *rowRig) frame() {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Source: r.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(300, 300)}}
	if d, ok := r.s.Update(gtx); ok {
		r.drops = append(r.drops, d)
	}
	r.size = r.s.List(gtx, SortList{Horizontal: true, Within: true}, r.ids, func(gtx C, i int) D {
		return r.s.Item(gtx, r.ids[i], func(gtx C) D { return D{Size: image.Pt(40, listRow)} }, nil)
	}).Size
	r.router.Frame(&ops)
}

func (r *rowRig) queue(kind pointer.Kind, p image.Point) {
	ev := pointer.Event{Kind: kind, Source: pointer.Mouse, Position: pt(p)}
	if kind != pointer.Release {
		ev.Buttons = pointer.ButtonPrimary
	}
	r.router.Queue(ev)
	r.frame()
}

// Dragged 52px right, and 20px down that the axis ignores, t1's center sits
// at 72, closest to t2's 60.
func TestSortListHorizontalDropsAlongTheRow(t *testing.T) {
	r := &rowRig{ids: []string{"t1", "t2", "t3"}}
	r.frame()
	if r.size != image.Pt(120, listRow) {
		t.Fatalf("row is %v, want 120x20", r.size)
	}
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(image.Pt(20, 10))})
	r.queue(pointer.Press, image.Pt(20, 10))
	r.queue(pointer.Move, image.Pt(26, 10))
	r.queue(pointer.Move, image.Pt(72, 30))
	if got := r.s.Over(); got != "t2" {
		t.Errorf("over %q, want t2", got)
	}
	r.queue(pointer.Release, image.Pt(72, 30))
	if len(r.drops) != 1 || r.drops[0] != (SortDrop{ID: "t1", Over: "t2"}) {
		t.Errorf("drops %v, want t1 over t2", r.drops)
	}
}
