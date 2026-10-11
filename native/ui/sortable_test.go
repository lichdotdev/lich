package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// sortRig is FooterLayoutEditor's shape: zone "left" at the top holding 30x20
// items 40px apart from (10, 10), and zone "right" 100px below it, empty.
type sortRig struct {
	router input.Router
	s      Sortable
	left   []string
	drops  []SortDrop
	lifted image.Point
}

var itemSize = image.Pt(30, 20)

func (r *sortRig) frame() {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Source: r.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(300, 200))}
	if d, ok := r.s.Update(gtx); ok {
		r.drops = append(r.drops, d)
	}
	r.zone(gtx, "left", image.Pt(0, 0), r.left)
	r.zone(gtx, "right", image.Pt(0, 100), nil)
	r.router.Frame(&ops)
}

func (r *sortRig) zone(gtx C, id string, at image.Point, items []string) {
	defer op.Offset(at).Push(gtx.Ops).Pop()
	r.s.Zone(gtx, id, func(gtx C) D {
		for i, it := range items {
			off := op.Offset(image.Pt(10+40*i, 10)).Push(gtx.Ops)
			r.s.Item(gtx, it, func(gtx C) D { return D{Size: itemSize} }, func(gtx C) D {
				r.lifted = gtx.Constraints.Min
				return D{Size: gtx.Constraints.Min}
			})
			off.Pop()
		}
		return D{Size: image.Pt(200, 50)}
	})
}

func newSortRig() *sortRig {
	r := &sortRig{left: []string{"a", "b"}}
	r.frame()
	return r
}

func pt(p image.Point) f32.Point { return f32.Pt(float32(p.X), float32(p.Y)) }

// press holds the primary button down at p.
func (r *sortRig) press(p image.Point) {
	r.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(p)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(p)},
	)
	r.frame()
}

func (r *sortRig) move(ps ...image.Point) {
	for _, p := range ps {
		r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(p)})
		r.frame()
	}
}

func (r *sortRig) release(p image.Point) {
	r.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pt(p)})
	r.frame()
}

// drag presses at the first point, moves through the others and releases at
// the last; the pointer moves twice at the end, as a hand does, so the target
// it lands on has heard it enter.
func (r *sortRig) drag(ps ...image.Point) {
	r.press(ps[0])
	last := ps[len(ps)-1]
	r.move(ps[1:]...)
	r.move(last.Add(image.Pt(1, 0)))
	r.release(last.Add(image.Pt(1, 0)))
}

func (r *sortRig) keys(names ...key.Name) {
	for _, n := range names {
		r.router.Queue(key.Event{Name: n, State: key.Press})
		r.frame()
	}
}

func (r *sortRig) want(t *testing.T, want ...SortDrop) {
	t.Helper()
	if len(r.drops) != len(want) || (len(want) == 1 && r.drops[0] != want[0]) {
		t.Errorf("drops %v, want %v", r.drops, want)
	}
}

var (
	onA       = image.Pt(15, 15)
	onB       = image.Pt(55, 15)
	leftSpace = image.Pt(150, 40)
	onRight   = image.Pt(100, 120)
	nowhere   = image.Pt(250, 180)
)

func TestSortableDropOverAnItemReportsTheItem(t *testing.T) {
	r := newSortRig()
	r.drag(onA, onA.Add(image.Pt(10, 0)), onB)
	r.want(t, SortDrop{ID: "a", Over: "b"})
}

func TestSortableDropOverAZoneReportsTheZone(t *testing.T) {
	r := newSortRig()
	r.drag(onA, onA.Add(image.Pt(10, 0)), onRight)
	r.want(t, SortDrop{ID: "a", Over: "right"})
	r = newSortRig()
	r.drag(onA, onA.Add(image.Pt(10, 0)), leftSpace)
	r.want(t, SortDrop{ID: "a", Over: "left"})
}

// The lifted item is not a target: back over its own placeholder, the zone
// under it is, so the drop appends, as dnd-kit drops the active id from hits.
func TestSortableDropOverItsOwnPlaceholderIsTheZone(t *testing.T) {
	r := newSortRig()
	r.drag(onA, onB, onA)
	r.want(t, SortDrop{ID: "a", Over: "left"})
}

func TestSortableDropOutsideEveryTargetDropsNothing(t *testing.T) {
	r := newSortRig()
	r.drag(onA, onA.Add(image.Pt(10, 0)), nowhere)
	r.want(t)
	if r.s.Dragging() != "" {
		t.Errorf("still dragging %q after the release", r.s.Dragging())
	}
}

// Below the 5px activation distance a press stays a click: nothing lifts.
func TestSortableShortDragIsAClick(t *testing.T) {
	r := newSortRig()
	r.press(onA)
	r.move(onA.Add(image.Pt(3, 0)))
	if r.s.Dragging() != "" {
		t.Errorf("dragging %q after 3px, want nothing below 5px", r.s.Dragging())
	}
	r.release(onA.Add(image.Pt(3, 0)))
	r.want(t)
}

func TestSortableLiftedFollowsAtTheItemsSize(t *testing.T) {
	r := newSortRig()
	r.press(onA)
	r.move(onB)
	if r.s.Dragging() != "a" || r.lifted != itemSize {
		t.Errorf("dragging %q, lifted at %v; want a at %v", r.s.Dragging(), r.lifted, itemSize)
	}
	r.move(onB.Add(image.Pt(1, 0)))
	if r.s.Over() != "b" {
		t.Errorf("over %q, want b", r.s.Over())
	}
}

func TestSortableEscapeCancelsAPointerDrag(t *testing.T) {
	r := newSortRig()
	r.press(onA)
	r.move(onA.Add(image.Pt(10, 0)), onB, onB.Add(image.Pt(1, 0)))
	r.keys(key.NameEscape)
	r.release(onB)
	r.want(t)
}

// The keyboard steps through the items and the empty zones in layout order;
// the populated left zone is reached through its items.
func TestSortableKeyboardMovesThroughTargets(t *testing.T) {
	r := newSortRig()
	r.press(onA)
	r.release(onA)
	if !r.s.Focused("a") {
		t.Fatal("a press did not focus the item")
	}
	r.keys(key.NameSpace)
	if r.s.Dragging() != "a" || r.s.Over() != "a" {
		t.Fatalf("after Space: dragging %q over %q, want a over a", r.s.Dragging(), r.s.Over())
	}
	r.keys(key.NameRightArrow)
	if r.s.Over() != "b" {
		t.Errorf("after Right: over %q, want b", r.s.Over())
	}
	r.keys(key.NameRightArrow, key.NameRightArrow, key.NameSpace)
	r.want(t, SortDrop{ID: "a", Over: "right"})
}

func TestSortableKeyboardEscapeDropsNothing(t *testing.T) {
	r := newSortRig()
	r.press(onA)
	r.release(onA)
	r.keys(key.NameSpace, key.NameRightArrow, key.NameEscape, key.NameSpace)
	if r.s.Dragging() != "a" {
		t.Errorf("Space after Escape should pick a up again; dragging %q", r.s.Dragging())
	}
	r.want(t)
}

func TestSortableDisabledLiftsNothing(t *testing.T) {
	r := newSortRig()
	r.s.Disabled = true
	r.drag(onA, onA.Add(image.Pt(10, 0)), onB)
	r.want(t)
}

// The cursor is grab over an item, grabbing for the whole drag wherever the
// pointer goes (over the item, another one, a zone's gap, no target at all),
// and back to the default once the item is dropped.
func TestSortableCursorIsGrabbingForTheWholeDrag(t *testing.T) {
	r := newSortRig()
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(onA)})
	r.frame()
	if c := r.router.Cursor(); c != pointer.CursorGrab {
		t.Errorf("hovering a: cursor %v, want grab", c)
	}
	r.press(onA)
	for _, p := range []image.Point{onA.Add(image.Pt(10, 0)), onB, leftSpace, onRight, nowhere} {
		r.move(p, p.Add(image.Pt(1, 0)))
		if c := r.router.Cursor(); c != pointer.CursorGrabbing {
			t.Errorf("dragging at %v: cursor %v, want grabbing", p, c)
		}
	}
	r.release(nowhere)
	r.move(nowhere.Add(image.Pt(2, 0)))
	if c := r.router.Cursor(); c != pointer.CursorDefault {
		t.Errorf("after the drop: cursor %v, want default", c)
	}
}

func TestSortableGrabCanBeWithheldPerItem(t *testing.T) {
	r := newSortRig()
	r.s.Grab = func(id string) bool { return id != "a" }
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(onA)})
	r.frame()
	if c := r.router.Cursor(); c != pointer.CursorDefault {
		t.Errorf("hovering a with its grab withheld: cursor %v, want default", c)
	}
	r.move(onB)
	if c := r.router.Cursor(); c != pointer.CursorGrab {
		t.Errorf("hovering b: cursor %v, want grab", c)
	}
}
