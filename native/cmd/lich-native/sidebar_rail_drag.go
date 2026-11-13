package main

import (
	"image"
	"math"
	"slices"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/ui"
)

// railSort is one block's drag in the rail: its rows sort as the open
// sidebar's cards do, and a card carried out lands on another block's head.
type railSort struct {
	cards ui.Sortable
	drag  dragLine
	slot  railItem // the card in the air as its slot links it
}

// railGroup is a block's run of rail rows: the gap before it, its head and
// its rows, laid out as one so its rows can sort.
type railGroup struct {
	block string
	items []railItem
}

// railGroups splits the rail's rows by the block each is in.
func railGroups(items []railItem) []railGroup {
	var out []railGroup
	for _, it := range items {
		if n := len(out); n > 0 && out[n-1].block == it.block {
			out[n-1].items = append(out[n-1].items, it)
			continue
		}
		out = append(out, railGroup{block: it.block, items: []railItem{it}})
	}
	return out
}

// split is the group's rows before its sortable ones (its gap and its head)
// and the rows that sort.
func (g railGroup) split() (lead, rows []railItem) {
	i := 0
	for i < len(g.items) && g.items[i].kind == railGap {
		i++
	}
	if i < len(g.items) && (g.items[i].kind == railHead || g.items[i].kind == railCheckout && g.items[i].key == g.block) {
		i++
	}
	return g.items[:i], g.items[i:]
}

func (r *sidebarRail) sort(block string) *railSort {
	if r.sorts == nil {
		r.sorts = map[string]*railSort{}
	}
	st, ok := r.sorts[block]
	if !ok {
		st = new(railSort)
		r.sorts[block] = st
	}
	return st
}

// group draws a block's rows: its head, then its rows sorting, the two a drop
// target as a whole while a card it takes is carried. A block a card would go
// to wears the open sidebar's lit tray.
func (r *sidebarRail) group(gtx C, th *ui.Theme, s *sidebar, g railGroup, b blockView, known bool, active string, act *sidebarAction) D {
	st := r.sort(g.block)
	ended := known && r.dropRail(gtx, s, st, b, act)
	lead, rows := g.split()
	gap := 0
	for gap < len(lead) && lead[gap].kind == railGap {
		gap++
	}
	lit := known && s.carry.Hovers(b.key)
	if lit {
		for _, list := range [][]railItem{lead, rows} {
			for i := range list {
				list[i].onTray = false
			}
		}
	}
	body := func(gtx C) D {
		children := make([]layout.FlexChild, 0, len(lead)-gap+1)
		for _, it := range lead[gap:] {
			children = append(children, layout.Rigid(func(gtx C) D { return r.item(gtx, th, s, it, active, act) }))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return r.rows(gtx, th, s, st, b, lead[gap:], rows, active, act) }))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	d := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			children := make([]layout.FlexChild, gap)
			for i := range gap {
				children[i] = layout.Rigid(func(gtx C) D { return r.item(gtx, th, s, lead[i], active, act) })
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
		layout.Rigid(func(gtx C) D {
			tray := body
			if lit {
				tray = func(gtx C) D { return dropTray(gtx, th, b.color, body) }
			}
			if takes, _ := s.headerDrop(b, r.view); known && takes {
				return s.carry.Target(gtx, b.key, tray)
			}
			return tray(gtx)
		}),
	)
	if ended {
		act.activate = ""
	}
	return d
}

// rows lays out a block's rows as a sortable list: each card picks up, the
// rows are linked where the drag shows them, and the slot the card in the
// air would land in draws its node dashed.
func (r *sidebarRail) rows(gtx C, th *ui.Theme, s *sidebar, st *railSort, b blockView, lead, rows []railItem, active string, act *sidebarAction) D {
	dragging := st.cards.Dragging()
	st.drag.update(gtx.Now, dragging)
	byID := railDragLines(lead, rows, dragging, st.cards.Over())
	if st.drag.moving(gtx.Now) {
		gtx.Execute(op.InvalidateCmd{})
	}
	ids := make([]string, len(rows))
	for i, it := range rows {
		ids[i] = it.id
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	d := st.cards.List(gtx, ui.SortList{Leaves: st.cards.Carry != nil, Snap: treeSnap(b, ids)}, ids, func(gtx C, i int) D {
		it := byID[ids[i]]
		if st.drag.loose(it.id, gtx.Now) {
			// The card in the air leaves its lines and its tray to its slot.
			it.up, it.down, it.onTray = false, false, false
		}
		it.cut = st.drag.cut(it.id, gtx.Now)
		if it.kind != railCard || it.motion != nil {
			return r.item(gtx, th, s, it, active, act)
		}
		return st.cards.Item(gtx, it.sv.ID, func(gtx C) D { return r.item(gtx, th, s, it, active, act) }, nil)
	})
	if at, length, ok := st.cards.Landing(); ok && dragging != "" {
		st.drag.hold(sessionRow{key: dragging}, at, length)
		st.slot = byID[dragging]
	}
	if st.drag.showsSlot(gtx.Now) {
		r.slot(gtx, th, st)
	}
	return d
}

// railDragLines is the block's rows by id, linked in the order a drag in
// the air shows them: the card moved to the slot it is over.
func railDragLines(lead, rows []railItem, dragging, over string) map[string]railItem {
	byID := make(map[string]railItem, len(rows))
	for _, it := range rows {
		byID[it.id] = it
	}
	from := slices.IndexFunc(rows, func(it railItem) bool { return it.id == dragging })
	to := slices.IndexFunc(rows, func(it railItem) bool { return it.id == over })
	if from < 0 || to < 0 || from == to {
		return byID
	}
	moved := slices.Delete(slices.Clone(rows), from, from+1)
	moved = slices.Insert(moved, to, rows[from])
	tree := append(slices.Clone(lead), moved...)
	for i := range tree {
		tree[i].up, tree[i].down = false, false
	}
	railLines(tree)
	for _, it := range tree[len(lead):] {
		byID[it.id] = it
	}
	return byID
}

// slot draws where the card in the air would land: the line through it, and
// its node dashed.
func (r *sidebarRail) slot(gtx C, th *ui.Theme, st *railSort) {
	d := &st.drag
	y := d.glide.y(gtx.Now, float32(d.at))
	defer op.Offset(image.Pt(0, int(math.Round(float64(y))))).Push(gtx.Ops).Pop()
	size := image.Pt(gtx.Constraints.Max.X, d.length)
	it := st.slot
	half := gtx.Dp(sessionTreeRingR)
	if it.small {
		half = gtx.Dp(railChildR)
	}
	it.motion = nil
	if !d.landed.IsZero() {
		// The slot lets go of its line as the card settles into it, and the
		// card's own draws back (dragLine.cut).
		it.cut = treeEase(clamp01(float32(gtx.Now.Sub(d.landed)) / float32(dragSettle)))
	}
	if it.onTray {
		railTray(gtx, th, it, size)
	}
	railLine(gtx, th, it, size, half)
	_, dashes := d.slot(gtx.Now)
	if dashes == 0 {
		return
	}
	defer paint.PushOpacity(gtx.Ops, dashes).Pop()
	c := th.Over(ui.Alpha(th.MutedForeground, treeLinkAlpha))
	center := f32.Pt(float32(size.X)/2, float32(size.Y)/2)
	if it.small {
		// A child's slot at a child's size, as its node.
		k := float32(gtx.Dp(railChildR)) / float32(gtx.Dp(sessionTreeRingR))
		defer op.Affine(f32.Affine2D{}.Scale(center, f32.Pt(k, k))).Push(gtx.Ops).Pop()
	}
	ui.DashedCircle(gtx, center, sessionTreeRingR, sessionTreeStroke, dragDash, c)
}

// dropRail reads the block's drag in the rail as dropCards reads the open
// sidebar's. It reports a drag that ended this frame.
func (r *sidebarRail) dropRail(gtx C, s *sidebar, st *railSort, b blockView, act *sidebarAction) bool {
	// Nothing drags under a filter, as in the open sidebar (dropCards).
	st.cards.Disabled = s.header.filter().narrows()
	st.cards.Carry = &s.carry
	if b.kind == blockPinned {
		st.cards.Carry = nil
	}
	before := st.cards.Dragging()
	d, ok := st.cards.Update(gtx)
	ended := before != "" && st.cards.Dragging() == ""
	if !ok {
		return ended
	}
	if d.Carried {
		st.drag = dragLine{}
		if i := slices.IndexFunc(r.view.blocks, func(t blockView) bool { return t.key == d.Over }); i >= 0 {
			act.file = fileRequest{ids: []string{d.ID}, folder: r.view.blocks[i].folder}
		}
		return ended
	}
	if d.ID != d.Over {
		act.cardOrder = blockReorder{key: b.key, ids: arrayMove(blockIDs(b), d.ID, d.Over)}
	}
	return ended
}
