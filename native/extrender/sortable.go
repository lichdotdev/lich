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
	list   string
	onMove extension.Handler
	index  int
}

// landing is the slot the item in the air would drop into if let go now.
func (g *groupState) landing() (moveTarget, bool) {
	t, ok := g.targets[g.carry.Over()]
	return t, ok
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
	if s.Moved() && (r.drag.down || r.drag.released) {
		r.drag.sorted = true
	}
	// A grouped item goes sideways to the other lists, so it may leave.
	list := ui.SortList{Gap: unit.Dp(p.Gap), Horizontal: p.Horizontal, Within: p.Within, Leaves: p.Leaves || g != nil}
	inAir := ""
	if g != nil && r.drag.down {
		inAir = s.Dragging()
	}
	items := listItems{calls: make([]op.CallOp, len(keys)), sizes: make([]image.Point, len(keys))}
	rec := op.Record(gtx.Ops)
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
	drawn := rec.Stop()
	if g == nil {
		drawn.Add(gtx.Ops)
		return d
	}
	takes := g.carry.Carrying() != "" && !slices.Contains(keys, g.carry.Carrying())
	if t, ok := g.landing(); takes && ok && t.list == id {
		lightRoom(gtx, th, r.dropRoom(gtx, p, d.Size))
	}
	drawn.Add(gtx.Ops)
	tops := items.tops(gtx.Dp(list.Gap), p.Horizontal)
	if a := slices.Index(keys, inAir); a >= 0 {
		r.drawInAir(gtx, along(tops[a], p.Horizontal), items.calls[a])
	} else if takes {
		r.layTargets(gtx, g, id, p, items, tops, d.Size)
	}
	return d
}

// dropRoom is the room a list takes a drop over: its breadth, by all the
// length it is given. A scroll gives its children no end, so there the room
// ends with the scroll's viewport.
// shortcut: the room is measured from the top of the scroll's child, so a
// list set lower inside that child overruns the viewport by as much; give
// the list its own offset when a board needs a header inside the scroll.
func (r *Renderer) dropRoom(gtx C, p extension.SortableListProps, size image.Point) image.Point {
	end := length(gtx.Constraints.Max, p.Horizontal)
	if room := length(r.scrollRoom, p.Horizontal); room > 0 {
		end = min(end, room)
	}
	return along(end, p.Horizontal).Add(along(length(size, !p.Horizontal), !p.Horizontal))
}

// dropTrayAlpha is the sidebar's folder tray tint (folderTrayAlpha): a card
// filled with the accent, as boards commonly are, stays distinct on it.
const dropTrayAlpha = 0.45

// lightRoom draws room as the sidebar draws the folder a carried card would
// be filed into.
func lightRoom(gtx C, th *ui.Theme, room image.Point) {
	ui.DropTray(gtx, th.Over(ui.Alpha(th.Accent, dropTrayAlpha)), th.MutedForeground, func(C) D { return D{Size: room} })
}

// dropped hands a drop to the extension: a reorder to this list, a move to
// the list the item was carried to.
func (r *Renderer) dropped(p extension.SortableListProps, from string, g *groupState, keys []string, d ui.SortDrop) {
	if !d.Carried {
		if order, moved := reorder(keys, d); moved {
			r.emit(p.OnReorder, extension.EventData{Order: order})
		}
		return
	}
	if t, ok := g.targets[d.Over]; ok {
		r.emit(t.onMove, extension.EventData{Key: d.ID, Index: t.index, From: from})
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
	room := r.dropRoom(gtx, p, size)
	end, cross := length(room, p.Horizontal), length(room, !p.Horizontal)
	from := 0
	for i := 0; i <= len(tops); i++ {
		to := end
		if i < len(tops) {
			to = tops[i] + length(it.sizes[i], p.Horizontal)/2
		}
		to = max(to, from)
		tid := id + "#" + strconv.Itoa(i)
		g.targets[tid] = moveTarget{list: id, onMove: p.OnMove, index: i}
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
	// sorted is a press, drawn this frame, that became a drag of a sortable
	// item, held until the frame after the pointer goes up.
	sorted bool
}

func (r *Renderer) readPointer(gtx C) {
	d := &r.drag
	d.released, d.sorted = false, false
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
