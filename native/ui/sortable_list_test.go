package ui

import (
	"image"
	"slices"
	"testing"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

// listRig is the sidebar's shape: blocks A and B in an outer List, each a
// 100x20 header (its drag handle) over an inner List of 40x20 cards, the
// inner Sortables sharing a Carry whose target is a block header that does
// not hold the card in the air. Laid out from the top:
//
//	A 0-20, a1 20-40, a2 40-60, a3 60-80, B 80-100, b1 100-120
//
// A second mouse pointer, never pressed, probes where things are drawn: every
// card holds a probe area the pointer's Enter reaches.
type listRig struct {
	router  input.Router
	now     time.Time
	outer   Sortable
	inner   map[string]*Sortable
	carry   SortCarry
	blocks  []string
	cards   map[string][]string
	probes  map[string]*bool
	drops   []SortDrop
	hovered []string
}

const listRow = 20

func newListRig() *listRig {
	r := &listRig{
		inner:  map[string]*Sortable{"A": {}, "B": {}},
		blocks: []string{"A", "B"},
		cards:  map[string][]string{"A": {"a1", "a2", "a3"}, "B": {"b1"}},
		probes: map[string]*bool{},
	}
	for _, s := range r.inner {
		s.Carry = &r.carry
	}
	for _, id := range []string{"a1", "a2", "a3", "b1"} {
		r.probes[id] = new(bool)
	}
	r.frame()
	return r
}

func (r *listRig) frame() {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Now: r.now, Source: r.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(300, 300)}}
	r.readProbes(gtx)
	r.collect(r.outer.Update(gtx))
	r.outer.List(gtx, SortList{Within: true}, r.blocks, func(gtx C, i int) D { return r.block(gtx, r.blocks[i]) })
	r.router.Frame(&ops)
}

func (r *listRig) collect(d SortDrop, ok bool) {
	if ok {
		r.drops = append(r.drops, d)
	}
}

func (r *listRig) block(gtx C, id string) D {
	in := r.inner[id]
	r.collect(in.Update(gtx))
	header := func(gtx C) D {
		return r.outer.Item(gtx, id, func(gtx C) D { return D{Size: image.Pt(100, listRow)} }, nil)
	}
	carried := r.carry.Carrying()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if carried != "" && !slices.Contains(r.cards[id], carried) {
				return r.carry.Target(gtx, id, header)
			}
			return header(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return in.List(gtx, SortList{}, r.cards[id], func(gtx C, i int) D { return r.card(gtx, in, r.cards[id][i]) })
		}),
	)
}

func (r *listRig) card(gtx C, s *Sortable, id string) D {
	return s.Item(gtx, id, func(gtx C) D {
		size := image.Pt(40, listRow)
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, r.probes[id])
		return D{Size: size}
	}, nil)
}

func (r *listRig) readProbes(gtx C) {
	for _, tag := range r.probes {
		for {
			ev, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Enter | pointer.Leave})
			if !ok {
				break
			}
			if e, ok := ev.(pointer.Event); ok && e.PointerID == probePointer {
				*tag = e.Kind == pointer.Enter
			}
		}
	}
}

const probePointer pointer.ID = 2

// probe moves the probing pointer to p, once things have settled, and
// returns the cards drawn under it.
func (r *listRig) probe(p image.Point) []string {
	r.wait(time.Second)
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, PointerID: probePointer, Position: pt(p)})
	r.frame()
	var under []string
	for _, id := range []string{"a1", "a2", "a3", "b1"} {
		if *r.probes[id] {
			under = append(under, id)
		}
	}
	return under
}

func (r *listRig) wait(d time.Duration) {
	r.now = r.now.Add(d)
	r.frame()
}

func (r *listRig) press(p image.Point) {
	r.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(p)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(p)},
	)
	r.frame()
}

func (r *listRig) move(ps ...image.Point) {
	for _, p := range ps {
		r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(p)})
		r.frame()
	}
}

func (r *listRig) release(p image.Point) {
	r.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pt(p)})
	r.frame()
}

func (r *listRig) keys(names ...key.Name) {
	for _, n := range names {
		r.router.Queue(key.Event{Name: n, State: key.Press})
		r.frame()
	}
}

func (r *listRig) want(t *testing.T, want ...SortDrop) {
	t.Helper()
	if !slices.Equal(r.drops, want) {
		t.Errorf("drops %v, want %v", r.drops, want)
	}
}

var (
	onA1     = image.Pt(20, 30)
	onHeadA  = image.Pt(50, 10)
	onHeadB  = image.Pt(70, 90)
	a1Lifted = onA1.Add(image.Pt(0, 6))
)

func TestSortListDropsOverTheClosestCenter(t *testing.T) {
	r := newListRig()
	r.press(onA1)
	r.move(a1Lifted, image.Pt(20, 52))
	if got := r.inner["A"].Over(); got != "a2" {
		t.Errorf("over %q 22px down, want a2", got)
	}
	r.release(image.Pt(20, 52))
	r.want(t, SortDrop{ID: "a1", Over: "a2"})
}

// Over a2, a2 steps up into a1's slot while a1 rides 22px down.
func TestSortListSiblingsStepAside(t *testing.T) {
	r := newListRig()
	r.press(onA1)
	r.move(a1Lifted, image.Pt(20, 52))
	if under := r.probe(image.Pt(5, 25)); !slices.Equal(under, []string{"a2"}) {
		t.Errorf("under a1's slot: %v, want a2 stepped into it", under)
	}
	if under := r.probe(image.Pt(5, 61)); !slices.Contains(under, "a1") {
		t.Errorf("under 42-62: %v, want the lifted a1", under)
	}
}

func TestSortListLiftedStaysOnTheVerticalAxis(t *testing.T) {
	r := newListRig()
	r.press(onA1)
	r.move(a1Lifted, image.Pt(80, 36))
	if under := r.probe(image.Pt(5, 28)); !slices.Contains(under, "a1") {
		t.Errorf("under (5, 28): %v, want a1 still at x 0-40", under)
	}
}

// Dragged 490px down, block A stops where its bottom meets the list's.
func TestSortListWithinHoldsTheItemInTheList(t *testing.T) {
	r := newListRig()
	r.press(onHeadA)
	r.move(onHeadA.Add(image.Pt(0, 6)), image.Pt(50, 500))
	if under := r.probe(image.Pt(5, 110)); !slices.Equal(under, []string{"a3"}) {
		t.Errorf("under (5, 110): %v, want a3 of A held at 40-120", under)
	}
	r.release(image.Pt(50, 500))
	r.want(t, SortDrop{ID: "A", Over: "B"})
}

// A press on a card drags the card alone, one on the header the block alone.
func TestSortListNestedListsDragByTheirOwnHandles(t *testing.T) {
	r := newListRig()
	r.press(onA1)
	r.move(a1Lifted)
	if r.outer.Dragging() != "" || r.inner["A"].Dragging() != "a1" {
		t.Errorf("card press: outer %q, inner %q; want only a1", r.outer.Dragging(), r.inner["A"].Dragging())
	}
	r.release(a1Lifted)
	r.press(onHeadA)
	r.move(onHeadA.Add(image.Pt(0, 6)))
	if r.outer.Dragging() != "A" || r.inner["A"].Dragging() != "" {
		t.Errorf("header press: outer %q, inner %q; want only A", r.outer.Dragging(), r.inner["A"].Dragging())
	}
}

// Carried onto B's header, a1 is over B, the list makes no room for it, it
// hangs 8px under the pointer, and the drop is the carry's.
func TestSortListCarriesAnItemOntoATarget(t *testing.T) {
	r := newListRig()
	r.press(onA1)
	r.move(a1Lifted, onHeadB, onHeadB.Add(image.Pt(1, 0)))
	if r.carry.Carrying() != "a1" || r.carry.Over() != "B" || r.inner["A"].Over() != "B" {
		t.Errorf("carrying %q over %q, list over %q; want a1 over B", r.carry.Carrying(), r.carry.Over(), r.inner["A"].Over())
	}
	if under := r.probe(image.Pt(5, 112)); !slices.Contains(under, "a1") {
		t.Errorf("under (5, 112): %v, want a1 hung at 98-118", under)
	}
	if under := r.probe(image.Pt(5, 45)); !slices.Equal(under, []string{"a2"}) {
		t.Errorf("under (5, 45): %v, want a2 in its own slot", under)
	}
	r.release(onHeadB)
	r.want(t, SortDrop{ID: "a1", Over: "B", Carried: true})
	if r.carry.Carrying() != "" {
		t.Errorf("still carrying %q after the drop", r.carry.Carrying())
	}
}

func TestSortListKeyboardStepsAndMakesRoom(t *testing.T) {
	r := newListRig()
	r.press(onA1)
	r.release(onA1)
	r.keys(key.NameSpace, key.NameDownArrow)
	if under := r.probe(image.Pt(5, 25)); !slices.Equal(under, []string{"a2"}) {
		t.Errorf("under a1's slot: %v, want a2 stepped into it", under)
	}
	r.keys(key.NameSpace)
	r.want(t, SortDrop{ID: "a1", Over: "a2"})
}

// After the drop the caller reorders, and a1 ends in its new slot: the
// offset it was let go at is measured from the slot it moved to.
func TestSortListSettlesIntoTheNewSlot(t *testing.T) {
	r := newListRig()
	r.press(onA1)
	r.move(a1Lifted, image.Pt(20, 52))
	r.release(image.Pt(20, 52))
	r.cards["A"] = []string{"a2", "a1", "a3"}
	r.frame()
	if under := r.probe(image.Pt(5, 45)); !slices.Equal(under, []string{"a1"}) {
		t.Errorf("settled: under (5, 45) %v, want a1 in slot 2", under)
	}
}
