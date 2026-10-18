package ui

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

// sortActivation is DRAG_THRESHOLD_PX of lib/use-sortable-list.ts: how far a
// press travels before it turns into a drag, so a plain click stays a click.
const sortActivation unit.Dp = 5

// Sortable is dnd-kit's DndContext with its pointer and keyboard sensors:
// items that are picked up and dropped over another item or over a zone. It
// moves nothing. Update reports "ID dropped over Over" and the caller, which
// owns the lists, applies it (moveFooterItem, arrayMove) and lays them out
// again, like any other caller-owned state of the kit.
//
// The pointer drag rides Gio's io/transfer: while a pointer is held down Gio
// tells only the handlers it was pressed on about Enter and Leave, unless a
// transfer is in flight, and then it tells every transfer target too, so
// items and zones are transfer targets (and sources, see hover).
//
// The keyboard sensor, without geometry Gio would expose to a widget, steps
// through the targets in layout order: Space or Enter on a focused item picks
// it up, the arrows move Over to the previous or next item or empty zone,
// Space or Enter drops, Escape cancels.
//
// List lays items out as a vertical list whose items step aside for the one
// in the air; see SortList and SortCarry.
type Sortable struct {
	// Disabled lays the items and zones out inert, dnd-kit's disabled.
	Disabled bool
	// Carry, shared by several Sortables, lets their items be dropped on its
	// targets. Their items then also hear each other pass, which nothing
	// reads: a Sortable's Over is only ever one of its own targets.
	Carry *SortCarry

	targets       map[string]*sortTarget
	order, laying []string

	pressed  string
	press    f32.Point
	delta    f32.Point
	active   string
	keyboard bool
	kbOver   string
	focused  string
	drop     SortDrop
	dropped  bool
	list     sortList
}

// SortDrop is dnd-kit's DragEndEvent: ID was dropped over Over. Carried means
// Over is a target of the Sortable's Carry rather than one of its own.
type SortDrop struct {
	ID, Over string
	Carried  bool
}

// sortTarget is an item's or a zone's event tag.
type sortTarget struct {
	id      string
	zone    bool
	empty   bool
	hovered bool
}

// Update handles the frame's input and reports a drop. Call it once a frame,
// before laying out any item or zone: it reads where the pointer is before
// what the button did, so a move and a release queued together drop where the
// move went.
func (s *Sortable) Update(gtx C) (SortDrop, bool) {
	if s.Carry != nil {
		s.Carry.hover(gtx)
		if s.active != "" && !s.keyboard && s.Carry.carrying == "" {
			s.end()
		}
	}
	s.order, s.laying = s.laying, s.order[:0]
	for id := range s.targets {
		// The item in the air keeps its target while it is not laid out,
		// for SortCarry.Hold to keep the drag's source alive.
		if !slices.Contains(s.order, id) && id != s.active {
			delete(s.targets, id)
		}
	}
	for _, id := range s.order {
		s.hover(gtx, s.targets[id])
	}
	for _, id := range s.order {
		if t := s.targets[id]; !t.zone {
			s.handle(gtx, t)
		}
	}
	if !s.dropped {
		return SortDrop{}, false
	}
	s.dropped = false
	return s.drop, true
}

// Dragging is the item in the air, "" when none is.
func (s *Sortable) Dragging() string { return s.active }

// Focused reports whether item id has the keyboard focus, for its ring.
func (s *Sortable) Focused(id string) bool { return s.focused == id }

// Over is the target the item in the air would land on now, "" for none: a
// Carry target under the pointer; in a List, the item it is over, itself
// included; else the item under the pointer other than the one lifted, else
// the zone under it. A zone that only rings while it is Over is dnd-kit's
// isOver.
func (s *Sortable) Over() string {
	over, _ := s.overTarget()
	return over
}

// overTarget is Over, and whether it is a Carry target.
func (s *Sortable) overTarget() (string, bool) {
	switch {
	case s.active == "":
		return "", false
	case s.keyboard:
		return s.kbOver, false
	case s.Carry != nil && s.Carry.Over() != "":
		return s.Carry.Over(), true
	}
	if a := slices.Index(s.list.ids, s.active); a >= 0 {
		return s.list.ids[s.list.over(s, a)], false
	}
	return s.hovered(), false
}

// hovered is the item under the pointer other than the one lifted, else the
// zone under it.
func (s *Sortable) hovered() string {
	zone := ""
	for _, id := range s.order {
		t := s.targets[id]
		switch {
		case !t.hovered || id == s.active:
		case !t.zone:
			return id
		case zone == "":
			zone = id
		}
	}
	return zone
}

// Item lays out w as the item id, picked up by a press on it. While it is
// dragged by the pointer, lifted is drawn over everything, sized to the item
// and following the pointer: dnd-kit's DragOverlay. The item itself stays
// where it was for the caller to draw as the placeholder (Dragging() == id).
// In a List, lifted is nil: the list carries the item itself.
func (s *Sortable) Item(gtx C, id string, w, lifted layout.Widget) D {
	if s.Disabled {
		return w(gtx)
	}
	t := s.target(id, false)
	s.laying = append(s.laying, id)
	s.hover(gtx, t)
	s.handle(gtx, t)
	m := op.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	event.Op(gtx.Ops, t)
	if s.active == id && !s.keyboard {
		pointer.CursorGrabbing.Add(gtx.Ops)
	}
	call.Add(gtx.Ops)
	area.Pop()
	if s.active == id && !s.keyboard {
		if lifted != nil {
			s.lift(gtx, d, lifted)
		}
		grabbing(gtx)
	}
	return d
}

// Zone lays out w as the drop area id. A drop over it, rather than over one
// of its items, means "append". Only a zone with no item in it is a keyboard
// target: a populated one is reached through its items.
func (s *Sortable) Zone(gtx C, id string, w layout.Widget) D {
	if s.Disabled {
		return w(gtx)
	}
	t := s.target(id, true)
	at := len(s.laying)
	s.laying = append(s.laying, id)
	s.hover(gtx, t)
	m := op.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	t.empty = len(s.laying) == at+1
	area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	event.Op(gtx.Ops, t)
	call.Add(gtx.Ops)
	area.Pop()
	return d
}

func (s *Sortable) target(id string, zone bool) *sortTarget {
	if s.targets == nil {
		s.targets = map[string]*sortTarget{}
	}
	t, ok := s.targets[id]
	if !ok {
		t = &sortTarget{id: id}
		s.targets[id] = t
	}
	t.zone = zone
	return t
}

// mime keeps two Sortables on one screen from dropping into each other,
// unless they share a Carry, whose targets must hear their items pass.
func (s *Sortable) mime() string {
	if s.Carry != nil {
		return s.Carry.mime()
	}
	return fmt.Sprintf("application/x-lich-sortable-%p", s)
}

func (s *Sortable) lift(gtx C, d D, lifted layout.Widget) {
	m := op.Record(gtx.Ops)
	op.Offset(s.delta.Round()).Add(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(d.Size)
	lifted(lgtx)
	op.Defer(gtx.Ops, m.Stop())
}

// hover tracks whether the pointer is over t. Every target is also a
// transfer source, from before any press: Gio starts the transfer on the
// first move of a press, with the first source still under the pointer, and
// a move that already left the item still has its zone under it.
func (s *Sortable) hover(gtx C, t *sortTarget) {
	for {
		ev, ok := gtx.Event(
			pointer.Filter{Target: t, Kinds: pointer.Enter | pointer.Leave},
			transfer.TargetFilter{Target: t, Type: s.mime()},
			transfer.SourceFilter{Target: t, Type: s.mime()},
		)
		if !ok {
			return
		}
		switch e := ev.(type) {
		case pointer.Event:
			t.hovered = e.Kind == pointer.Enter
		case transfer.RequestEvent:
			// The drop is read from Over, not from the transfer; offering
			// nothing ends it, or Gio would hold the next drag's source.
			gtx.Execute(transfer.OfferCmd{Tag: t, Type: e.Type, Data: io.NopCloser(strings.NewReader(""))})
		}
	}
}

// handle reads an item's press, drag, focus and keys.
func (s *Sortable) handle(gtx C, t *sortTarget) {
	for {
		ev, ok := gtx.Event(s.filters(t)...)
		if !ok {
			return
		}
		switch e := ev.(type) {
		case pointer.Event:
			s.pointer(gtx, t, e)
		case key.FocusEvent:
			if e.Focus {
				s.focused = t.id
			} else if s.focused == t.id {
				s.focused = ""
			}
		case key.Event:
			s.key(t, e)
		}
	}
}

// filters asks for the arrows and Escape only while t is in the air, so a
// focused item does not take Escape from the dialog it sits in.
func (s *Sortable) filters(t *sortTarget) []event.Filter {
	fs := []event.Filter{
		pointer.Filter{Target: t, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel},
		key.FocusFilter{Target: t},
		key.Filter{Focus: t, Name: key.NameSpace},
		key.Filter{Focus: t, Name: key.NameReturn},
		key.Filter{Focus: t, Name: key.NameEnter},
	}
	if s.active != t.id {
		return fs
	}
	fs = append(fs, key.Filter{Focus: t, Name: key.NameEscape})
	for _, n := range []key.Name{key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow} {
		fs = append(fs, key.Filter{Focus: t, Name: n})
	}
	return fs
}

func (s *Sortable) pointer(gtx C, t *sortTarget, e pointer.Event) {
	switch e.Kind {
	case pointer.Press:
		if !e.Buttons.Contain(pointer.ButtonPrimary) || s.active != "" {
			return
		}
		s.pressed, s.press, s.delta = t.id, s.list.atRest(t.id, e.Position), f32.Point{}
		gtx.Execute(key.FocusCmd{Tag: t})
	case pointer.Drag:
		if s.pressed != t.id {
			return
		}
		s.delta = s.list.atRest(t.id, e.Position).Sub(s.press)
		travel := float32(gtx.Dp(sortActivation))
		if s.active == "" && s.delta.X*s.delta.X+s.delta.Y*s.delta.Y >= travel*travel {
			s.active, s.keyboard = t.id, false
			if s.Carry != nil {
				s.Carry.carrying, s.Carry.source = t.id, s
			}
		}
	case pointer.Release:
		if s.pressed == t.id && s.active == t.id {
			s.finish(s.overTarget())
		}
		s.pressed = ""
	case pointer.Cancel:
		if s.pressed == t.id {
			s.end()
		}
	}
}

func (s *Sortable) key(t *sortTarget, e key.Event) {
	if e.State != key.Press {
		return
	}
	switch e.Name {
	case key.NameSpace, key.NameReturn, key.NameEnter:
		if s.active == "" {
			s.active, s.keyboard, s.kbOver = t.id, true, t.id
		} else if s.keyboard {
			s.finish(s.kbOver, false)
		}
	case key.NameEscape:
		s.end()
	case key.NameLeftArrow, key.NameUpArrow:
		s.step(-1)
	case key.NameRightArrow, key.NameDownArrow:
		s.step(1)
	}
}

// step moves the keyboard's Over to the next target in layout order, items
// and empty zones, stopping at either end.
func (s *Sortable) step(dir int) {
	if !s.keyboard {
		return
	}
	var stops []string
	for _, id := range s.order {
		if t := s.targets[id]; !t.zone || t.empty {
			stops = append(stops, id)
		}
	}
	if i := slices.Index(stops, s.kbOver); i >= 0 && i+dir >= 0 && i+dir < len(stops) {
		s.kbOver = stops[i+dir]
	}
}

func (s *Sortable) finish(over string, carried bool) {
	if over != "" {
		s.drop, s.dropped = SortDrop{ID: s.active, Over: over, Carried: carried}, true
	}
	s.end()
}

func (s *Sortable) end() {
	if s.Carry != nil && s.active != "" && s.Carry.carrying == s.active {
		s.Carry.carrying, s.Carry.source = "", nil
	}
	s.active, s.keyboard, s.kbOver, s.pressed = "", false, "", ""
}
