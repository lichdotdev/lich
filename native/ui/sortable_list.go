package ui

import (
	"fmt"
	"image"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

const (
	// sortSettle is dnd-kit's defaultTransition for sortable items,
	// `transform 250ms ease`: siblings stepping aside, and an item settling
	// into its slot after the drop.
	sortSettle = 250 * time.Millisecond
	// sortHang is HANG_BELOW_POINTER_PX of file-drag-store.ts.
	sortHang unit.Dp = 8
	// sortCursorReach is how far the drag's grabbing cursor reaches around
	// the item, past the edge of any window.
	sortCursorReach = 1 << 20
)

// SortList is a SortableContext with verticalListSortingStrategy: List lays
// its items out top to bottom, Gap apart.
type SortList struct {
	Gap unit.Dp
	// Within holds the item in the air inside the list, use-sortable-list's
	// withinList. Without it the item goes wherever the pointer takes it
	// along the axis, out to a SortCarry target.
	Within bool
}

// sortList is the geometry List laid out last frame, which Over reads before
// this frame's layout, and the motion of every item.
type sortList struct {
	ids           []string
	tops, heights []int
	height        int
	gap, hang     int
	within        bool
	press         f32.Point
	slots         map[string]*sortSlot
}

// sortSlot is an item's offset from its slot, animated, and where it was
// drawn last frame, which a reorder animates it from.
type sortSlot struct {
	index int
	y     float32
	shift tween
}

// List lays ids out as a vertical sortable list: item(gtx, i) draws ids[i]
// and lays out, through s.Item with a nil lifted, the part a press picks it
// up by (all of it, or a block's header alone). While one is in the air it
// follows the pointer on the vertical axis only, drawn over everything, and
// the items between its slot and the one it is over step aside by its
// height, as dnd-kit's verticalListSortingStrategy; Over is that item,
// closestCenter. One List per Sortable.
func (s *Sortable) List(gtx C, l SortList, ids []string, item func(gtx C, i int) D) D {
	s.listPress(gtx)
	calls, width := s.measure(gtx, l, ids, item)
	a := slices.Index(ids, s.active)
	defer clip.Rect{Max: image.Pt(width, s.list.height)}.Push(gtx.Ops).Pop()
	if !s.Disabled {
		event.Op(gtx.Ops, &s.list)
	}
	for j := range ids {
		if j != a {
			s.place(gtx, j, s.offset(gtx, j, a), calls[j])
		}
	}
	if a >= 0 {
		s.lifted(gtx, a, s.offset(gtx, a, a), calls[a])
	}
	s.prune(ids)
	return D{Size: image.Pt(width, s.list.height)}
}

// listPress keeps where the last press landed in the list, which the hang
// below a SortCarry target is measured from.
func (s *Sortable) listPress(gtx C) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &s.list, Kinds: pointer.Press})
		if !ok {
			return
		}
		if e, ok := ev.(pointer.Event); ok {
			s.list.press = e.Position
		}
	}
}

// measure records every item at its slot and keeps the geometry.
func (s *Sortable) measure(gtx C, l SortList, ids []string, item func(gtx C, i int) D) ([]op.CallOp, int) {
	g := &s.list
	g.ids, g.tops, g.heights = slices.Clone(ids), g.tops[:0], g.heights[:0]
	g.gap, g.hang, g.within = gtx.Dp(l.Gap), gtx.Dp(sortHang), l.Within
	calls := make([]op.CallOp, len(ids))
	igtx := gtx
	igtx.Constraints.Min.Y = 0
	y, width := 0, gtx.Constraints.Min.X
	for i := range ids {
		if i > 0 {
			y += g.gap
		}
		m := op.Record(gtx.Ops)
		d := item(igtx, i)
		calls[i] = m.Stop()
		g.tops, g.heights = append(g.tops, y), append(g.heights, d.Size.Y)
		y += d.Size.Y
		width = max(width, d.Size.X)
	}
	g.height = y
	return calls, width
}

// offset is item j's distance from its slot this frame, with a the index
// of the item in the air (-1 for none). The item a pointer carries follows
// it; every other offset, the keyboard's item included, moves as a CSS
// transition, from where the item was drawn when its slot changed.
func (s *Sortable) offset(gtx C, j, a int) float32 {
	g := &s.list
	id := g.ids[j]
	if g.slots == nil {
		g.slots = map[string]*sortSlot{}
	}
	slot, seen := g.slots[id]
	if !seen {
		slot = &sortSlot{index: j}
		g.slots[id] = slot
	}
	var off float32
	if j == a && !s.keyboard {
		off = float32(g.dy(s, a))
		slot.shift.seed(off)
	} else {
		if seen && slot.index != j {
			slot.shift.seed(slot.y - float32(g.tops[j]))
		}
		off = slot.shift.at(gtx, float32(g.target(s, j, a)), sortSettle, ease)
	}
	slot.index, slot.y = j, float32(g.tops[j])+off
	return off
}

func (s *Sortable) place(gtx C, j int, off float32, call op.CallOp) {
	defer op.Affine(f32.Affine2D{}.Offset(f32.Pt(0, float32(s.list.tops[j])+off))).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// lifted draws the item in the air over everything, unclipped, and lets the
// pointer through it to what is under it (pointer-events-none), so a
// SortCarry target under the item still hears the pointer arrive. The
// grabbing cursor comes with the item's own Item.
func (s *Sortable) lifted(gtx C, a int, off float32, call op.CallOp) {
	m := op.Record(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	s.place(gtx, a, off, call)
	pass.Pop()
	op.Defer(gtx.Ops, m.Stop())
}

// grabbing shows CursorGrabbing wherever the pointer goes for the rest of
// the drag. It hears nothing, so what is under it still does.
func grabbing(gtx C) {
	m := op.Record(gtx.Ops)
	reach := image.Rect(-sortCursorReach, -sortCursorReach, sortCursorReach, sortCursorReach)
	area := clip.Rect(reach).Push(gtx.Ops)
	pointer.CursorGrabbing.Add(gtx.Ops)
	area.Pop()
	op.Defer(gtx.Ops, m.Stop())
}

func (s *Sortable) prune(ids []string) {
	for id := range s.list.slots {
		if !slices.Contains(ids, id) {
			delete(s.list.slots, id)
		}
	}
}

// atRest takes a position in item id's own area back to where the area sits
// at rest. A List draws its items, handles included, where they are moving
// to, and Gio reports a position in the area as it was last drawn: read as
// it is, the drag would lose what it already moved the item by.
func (g *sortList) atRest(id string, p f32.Point) f32.Point {
	slot, ok := g.slots[id]
	if !ok || slot.index >= len(g.tops) {
		return p
	}
	return p.Add(f32.Pt(0, slot.y-float32(g.tops[slot.index])))
}

// dy is where the pointer has taken item a: its drag on the vertical axis
// (restrictToVerticalAxis), held inside the list when within, and hung just
// under the pointer while a SortCarry target is under it (hangBelowTarget).
func (g *sortList) dy(s *Sortable, a int) int {
	dy := int(s.delta.Y + 0.5)
	if g.within {
		dy = min(max(dy, -g.tops[a]), g.height-g.tops[a]-g.heights[a])
	}
	if s.Carry != nil && s.Carry.Over() != "" {
		dy = int(g.press.Y+s.delta.Y+0.5) + g.hang - g.tops[a]
	}
	return dy
}

// over is the index of the item a lands on: the keyboard's, or the one whose
// center is closest to a's where the pointer took it (closestCenter). Over
// a SortCarry target it is a itself, so the list makes no room for a drop
// that files the item instead (fileAwareCollision).
func (g *sortList) over(s *Sortable, a int) int {
	if s.keyboard {
		if o := slices.Index(g.ids, s.kbOver); o >= 0 {
			return o
		}
		return a
	}
	if s.Carry != nil && s.Carry.Over() != "" {
		return a
	}
	center := g.tops[a] + g.heights[a]/2 + g.dy(s, a)
	best, dist := a, -1
	for j := range g.ids {
		d := g.tops[j] + g.heights[j]/2 - center
		d = max(d, -d)
		if dist < 0 || d < dist {
			best, dist = j, d
		}
	}
	return best
}

// target is verticalListSortingStrategy's transform for item j: aside by
// a's height and the gap when it sits between a's slot and the one a is
// over, and for the keyboard's a, the slot it is over.
func (g *sortList) target(s *Sortable, j, a int) int {
	if a < 0 {
		return 0
	}
	o := g.over(s, a)
	step := g.heights[a] + g.gap
	switch {
	case j == a && a < o:
		return g.tops[o] + g.heights[o] - g.tops[a] - g.heights[a]
	case j == a:
		return g.tops[o] - g.tops[a]
	case j > a && j <= o:
		return -step
	case j < a && j >= o:
		return step
	}
	return 0
}

// SortCarry is file-drag-store.ts: it lets an item of any Sortable that
// shares it be carried out of its own list and dropped on a target that
// belongs to none of them, the header of another block a card is filed into.
// The drop comes from the item's Sortable, with Carried set. A keyboard drag
// moves no pointer and never reaches a target: the card's menu is that path.
type SortCarry struct {
	targets  map[string]*carryTarget
	carrying string
	latest   time.Time
}

// carryTarget is a target's event tag; laid is the frame it was last laid
// out in, so one that stopped taking the item stops being under it.
type carryTarget struct {
	hovered bool
	laid    time.Time
}

// Carrying is the item in the air from any Sortable sharing c, "" when none
// is, for a target to decide whether it takes it.
func (c *SortCarry) Carrying() string { return c.carrying }

// Over is the target under the pointer while an item is carried, "" for
// none: the header that would draw as the drop target.
func (c *SortCarry) Over() string {
	if c.carrying == "" {
		return ""
	}
	for id, t := range c.targets {
		if t.hovered && t.laid.Equal(c.latest) {
			return id
		}
	}
	return ""
}

// Target lays out w as the drop target id. Lay out only a target that takes
// Carrying(), as a header wears FILE_TARGET_ATTRIBUTE only while it does.
func (c *SortCarry) Target(gtx C, id string, w layout.Widget) D {
	if c.targets == nil {
		c.targets = map[string]*carryTarget{}
	}
	t, ok := c.targets[id]
	if !ok {
		t = &carryTarget{}
		c.targets[id] = t
	}
	t.laid, c.latest = gtx.Now, gtx.Now
	c.hover(gtx)
	d := w(gtx)
	defer clip.Rect{Max: d.Size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, t)
	return d
}

// hover reads every target's Enter and Leave. The Sortables sharing c read
// them before their own release, so a move and a release queued together
// drop where the move went.
func (c *SortCarry) hover(gtx C) {
	for _, t := range c.targets {
		for {
			ev, ok := gtx.Event(
				pointer.Filter{Target: t, Kinds: pointer.Enter | pointer.Leave},
				transfer.TargetFilter{Target: t, Type: c.mime()},
			)
			if !ok {
				break
			}
			if e, ok := ev.(pointer.Event); ok {
				t.hovered = e.Kind == pointer.Enter
			}
		}
	}
}

func (c *SortCarry) mime() string { return fmt.Sprintf("application/x-lich-sortable-%p", c) }
