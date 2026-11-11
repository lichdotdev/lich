package extrender

import (
	"image"
	"slices"
	"strconv"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

// groupPrefix keeps a group's state apart from the nodes': a node's id is
// "" or starts with "/".
const groupPrefix = "group:"

// groupState is what the lists of one group share: the carry their items
// travel on, and the list and slot each of its targets drops into.
type groupState struct {
	carry   ui.SortCarry
	targets map[string]moveTarget
}

type moveTarget struct {
	onMove extension.Handler
	index  int
}

// listItems is what a list's items drew this frame, for the one in the air
// to be drawn again over everything and for the slots a drop lands in.
type listItems struct {
	calls []op.CallOp
	sizes []image.Point
}

// group is the state of the group name, nil for a list in none.
func (r *Renderer) group(name string) *groupState {
	if name == "" {
		return nil
	}
	g := stateOf[groupState](r, groupPrefix+name)
	if g.targets == nil {
		g.targets = map[string]moveTarget{}
	}
	return g
}

func drawSortableList(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.SortableListProps](n)
	s := stateOf[ui.Sortable](r, id)
	keys := make([]string, len(n.Children))
	for i, c := range n.Children {
		keys[i] = childKey(c, i)
	}
	g := r.group(p.Group)
	s.Carry = nil
	if g != nil {
		s.Carry = &g.carry
	}
	if d, ok := s.Update(gtx); ok {
		r.dropped(p, n.Key, g, keys, d)
	}
	// A grouped item goes sideways to the other lists, so it may leave.
	list := ui.SortList{Gap: unit.Dp(p.Gap), Horizontal: p.Horizontal, Within: p.Within, Leaves: p.Leaves || g != nil}
	inAir := ""
	if g != nil && r.drag.down {
		inAir = s.Dragging()
	}
	items := listItems{calls: make([]op.CallOp, len(keys)), sizes: make([]image.Point, len(keys))}
	d := s.List(gtx, list, keys, func(gtx C, i int) D {
		m := op.Record(gtx.Ops)
		cd := r.child(th, n.Children[i], id, i)(gtx)
		items.calls[i], items.sizes[i] = m.Stop(), cd.Size
		return s.Item(gtx, keys[i], func(gtx C) D {
			if keys[i] != inAir {
				items.calls[i].Add(gtx.Ops)
			}
			return cd
		}, nil)
	})
	if g == nil {
		return d
	}
	tops := items.tops(gtx.Dp(list.Gap), p.Horizontal)
	if a := slices.Index(keys, inAir); a >= 0 {
		r.drawInAir(gtx, along(tops[a], p.Horizontal), items.calls[a])
	} else if c := g.carry.Carrying(); c != "" && !slices.Contains(keys, c) {
		r.layTargets(gtx, g, id, p, items, tops, d.Size)
	}
	return d
}

// dropped hands a drop to the extension: a reorder to this list, a move to
// the list the item was carried to.
func (r *Renderer) dropped(p extension.SortableListProps, from string, g *groupState, keys []string, d ui.SortDrop) {
	if !d.Carried {
		if order, moved := reorder(keys, d); moved {
			r.fire(p.OnReorder, extension.EventData{Order: order})
		}
		return
	}
	if t, ok := g.targets[d.Over]; ok {
		r.fire(t.onMove, extension.EventData{Key: d.ID, Index: t.index, From: from})
	}
}

// reorder is keys with the dropped one moved to the slot of the one it was
// dropped over, as dnd-kit's arrayMove; a drop on itself or off the list
// moves nothing.
func reorder(keys []string, d ui.SortDrop) ([]string, bool) {
	from, to := slices.Index(keys, d.ID), slices.Index(keys, d.Over)
	if from < 0 || to < 0 || from == to {
		return nil, false
	}
	out := slices.Delete(slices.Clone(keys), from, from+1)
	return slices.Insert(out, to, d.ID), true
}

// tops is where each item starts along the list's axis, at rest.
func (it listItems) tops(gap int, horizontal bool) []int {
	tops := make([]int, len(it.sizes))
	y := 0
	for i, sz := range it.sizes {
		tops[i] = y
		y += length(sz, horizontal) + gap
	}
	return tops
}

// drawInAir draws the item the pointer carries over everything, from its
// slot at rest by as far as the pointer went, and lets the pointer through
// it to the list under it. The list keeps its slot empty meanwhile.
func (r *Renderer) drawInAir(gtx C, slot image.Point, call op.CallOp) {
	m := op.Record(gtx.Ops)
	op.Offset(slot.Add(r.drag.at.Sub(r.drag.press).Round())).Add(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	call.Add(gtx.Ops)
	pass.Pop()
	op.Defer(gtx.Ops, m.Stop())
}

// layTargets lays the slots an item carried from another list of the group
// lands in. Slot i runs from the middle of item i-1 to the middle of item i,
// and the last one on to the end of the room the list is given, so an empty
// list or the space under the last item still takes a drop.
func (r *Renderer) layTargets(gtx C, g *groupState, id string, p extension.SortableListProps, it listItems, tops []int, size image.Point) {
	end := length(gtx.Constraints.Max, p.Horizontal)
	cross := length(size, !p.Horizontal)
	from := 0
	for i := 0; i <= len(tops); i++ {
		to := end
		if i < len(tops) {
			to = tops[i] + length(it.sizes[i], p.Horizontal)/2
		}
		to = max(to, from)
		tid := id + "#" + strconv.Itoa(i)
		g.targets[tid] = moveTarget{onMove: p.OnMove, index: i}
		slot := along(from, p.Horizontal)
		box := along(to-from, p.Horizontal).Add(along(cross, !p.Horizontal))
		st := op.Offset(slot).Push(gtx.Ops)
		g.carry.Target(gtx, tid, func(C) D { return D{Size: box} })
		st.Pop()
		from = to
	}
}

// length is p's component along a list's axis.
func length(p image.Point, horizontal bool) int {
	if horizontal {
		return p.X
	}
	return p.Y
}

// along is the point v along a list's axis.
func along(v int, horizontal bool) image.Point {
	if horizontal {
		return image.Pt(v, 0)
	}
	return image.Pt(0, v)
}

// dragPointer follows the pointer over the whole surface, which a list
// reads to draw the item it carries out of its own box.
type dragPointer struct {
	press, at f32.Point
	down      bool
	// released is the pointer let go this frame.
	released bool
}

func (r *Renderer) readPointer(gtx C) {
	d := &r.drag
	d.released = false
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: d, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			d.press, d.at, d.down = e.Position, e.Position, true
		case pointer.Drag:
			d.at = e.Position
		default:
			d.down, d.released = false, true
		}
	}
}

// holdGroups keeps every group with an item in the air, and the drag's
// source, alive through a frame whose tree no longer draws that list: Gio
// forgets a handler nobody asked about in a frame, and a drag whose source
// it forgot brings the window down.
func (r *Renderer) holdGroups(gtx C) {
	for id, v := range r.state {
		if g, ok := v.(*groupState); ok && g.carry.Carrying() != "" {
			r.seen[id] = true
			g.carry.Hold(gtx)
		}
	}
}

// releaseStranded lets go of a drag whose list was not drawn to hear the
// pointer go up.
func (r *Renderer) releaseStranded(gtx C) {
	if !r.drag.released {
		return
	}
	for _, v := range r.state {
		if g, ok := v.(*groupState); ok && g.carry.Carrying() != "" {
			g.carry.Release()
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}
