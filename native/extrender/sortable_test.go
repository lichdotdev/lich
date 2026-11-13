package extrender

import (
	"slices"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/extension"
)

// card is an item 30 tall, so a slot's place does not hang on shaped text.
func card(k string) extension.Node {
	return extension.Box.New(extension.BoxProps{Height: 30}, label(k)).WithKey(k)
}

func sortable(key, group string, cards ...string) extension.Node {
	kids := make([]extension.Node, len(cards))
	for i, c := range cards {
		kids[i] = card(c)
	}
	return extension.SortableList.New(extension.SortableListProps{Group: group,
		OnReorder: extension.Handler("reorder:" + key), OnMove: extension.Handler("move:" + key)}, kids...).WithKey(key)
}

// board is two columns 150 wide, 20 apart: "todo" at x 0..150 and "done" at
// 170..320, the second one deeper in the tree and inside a scroll.
func board(todoGroup, doneGroup string) extension.Node {
	todo := extension.Box.New(extension.BoxProps{Width: 150}, sortable("todo", todoGroup, "a", "b", "c"))
	done := extension.Box.New(extension.BoxProps{Width: 150},
		extension.Box.New(extension.BoxProps{}, extension.Scroll.New(extension.ScrollProps{}, sortable("done", doneGroup, "x", "y"))))
	return extension.Box.New(extension.BoxProps{Direction: extension.DirectionRow, Gap: 20}, todo, done)
}

// drag presses at from, moves to to in steps with a frame after each, as a
// hand does, and lets go there.
func (g *rig) drag(from, to f32.Point) {
	g.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: from},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: from},
	)
	g.frame()
	const steps = 8
	for i := 1; i <= steps; i++ {
		at := from.Add(to.Sub(from).Mul(float32(i) / steps))
		g.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at})
		g.frame()
	}
	g.frame()
	g.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: to})
	g.frame()
}

func TestADragIntoAnotherListOfTheGroupSendsItsMove(t *testing.T) {
	cases := []struct {
		name     string
		from, to f32.Point
		h        extension.Handler
		want     extension.EventData
	}{
		{"onto the second item", f32.Pt(75, 15), f32.Pt(245, 40), "move:done", extension.EventData{Key: "a", Index: 1, From: "todo"}},
		{"under the last item", f32.Pt(75, 45), f32.Pt(245, 200), "move:done", extension.EventData{Key: "b", Index: 2, From: "todo"}},
		{"back to the first list's top", f32.Pt(245, 45), f32.Pt(75, 5), "move:todo", extension.EventData{Key: "y", Index: 0, From: "done"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t, board("board", "board"))
			g.drag(tc.from, tc.to)
			e := g.once(tc.h)
			if e.Key != tc.want.Key || e.Index != tc.want.Index || e.From != tc.want.From {
				t.Errorf("move = %+v, want key %q index %d from %q", e, tc.want.Key, tc.want.Index, tc.want.From)
			}
			if len(g.got) != 1 {
				t.Errorf("events = %+v, want only the move", g.got)
			}
		})
	}
}

func TestADragWithinAGroupedListStillReorders(t *testing.T) {
	g := newRig(t, board("board", "board"))
	g.drag(f32.Pt(75, 15), f32.Pt(75, 75))
	if e := g.once("reorder:todo"); !slices.Equal(e.Order, []string{"b", "c", "a"}) {
		t.Errorf("reorder = %+v, want b c a", e)
	}
	if len(g.got) != 1 {
		t.Errorf("events = %+v, want only the reorder", g.got)
	}
}

func TestListsOfDifferentGroupsDoNotExchange(t *testing.T) {
	for _, groups := range [][2]string{{"board", "other"}, {"board", ""}} {
		g := newRig(t, board(groups[0], groups[1]))
		g.drag(f32.Pt(75, 15), f32.Pt(245, 40))
		if len(g.got) != 0 {
			t.Errorf("groups %q: events = %+v, want none", groups, g.got)
		}
	}
}

func TestADropOutsideEveryListSendsNothing(t *testing.T) {
	g := newRig(t, board("board", "board"))
	g.drag(f32.Pt(75, 15), f32.Pt(360, 150))
	if len(g.got) != 0 {
		t.Errorf("events = %+v, want none", g.got)
	}
}

func TestADragWhoseListGoesAwayEndsWhenThePointerGoesUp(t *testing.T) {
	g := newRig(t, board("board", "board"))
	from := f32.Pt(75, 15)
	g.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: from},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: from},
	)
	g.frame()
	g.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(245, 40)})
	g.frame()
	carry := &stateOf[groupState](g.r, groupPrefix+"board").carry
	if carry.Carrying() != "a" {
		t.Fatalf("carrying %q, want a", carry.Carrying())
	}
	if d := g.r.drag; !d.down || d.at != f32.Pt(245, 40) || d.press != from {
		t.Fatalf("the surface follows the pointer as %+v, want pressed at %v and at (245,40)", d, from)
	}
	g.tree = extension.Box.New(extension.BoxProps{Width: 150}, sortable("done", "board", "x", "y"))
	g.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(75, 40)})
	g.frame()
	g.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(75, 40)})
	g.frame()
	g.frame()
	if carry.Carrying() != "" || len(g.got) != 0 {
		t.Errorf("carrying %q, events %+v; want the drag let go with nothing sent", carry.Carrying(), g.got)
	}
}

// hover presses at from and carries the item to to, a frame after each step,
// and holds it there.
func (g *rig) hover(from, to f32.Point) {
	g.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: from},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: from},
	)
	g.frame()
	// The first move stays on the item, as a hand's does: Gio starts the
	// transfer the targets hear from the source under that move.
	path := []f32.Point{from.Add(f32.Pt(6, 0))}
	const steps = 8
	for i := 1; i <= steps; i++ {
		path = append(path, from.Add(to.Sub(from).Mul(float32(i)/steps)))
	}
	for _, at := range path {
		g.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at})
		g.frame()
	}
	g.frame()
}

// landing is the onMove of the list drawn as the drop target, "" for none.
func (g *rig) landing() extension.Handler {
	t, _ := stateOf[groupState](g.r, groupPrefix+"board").landing()
	return t.onMove
}

func TestTheListAnItemIsCarriedOverIsTheDropTarget(t *testing.T) {
	cases := []struct {
		name     string
		from, to f32.Point
		want     extension.Handler
	}{
		{"over another list of the group", f32.Pt(75, 15), f32.Pt(245, 40), "move:done"},
		{"under its last item", f32.Pt(75, 15), f32.Pt(245, 200), "move:done"},
		{"back over the other list", f32.Pt(245, 45), f32.Pt(75, 5), "move:todo"},
		{"within its own list", f32.Pt(75, 15), f32.Pt(75, 75), ""},
		{"over no list", f32.Pt(75, 15), f32.Pt(360, 150), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t, board("board", "board"))
			g.hover(tc.from, tc.to)
			if got := g.landing(); got != tc.want {
				t.Errorf("drop target = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTheDropTargetGoesWithTheItem(t *testing.T) {
	from, over := f32.Pt(75, 15), f32.Pt(245, 40)
	ends := map[string][]pointer.Event{
		"dropped":              {{Kind: pointer.Release, Source: pointer.Mouse, Position: over}},
		"cancelled":            {{Kind: pointer.Cancel, Source: pointer.Mouse}},
		"carried off the list": {{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(360, 150)}},
	}
	for name, evs := range ends {
		t.Run(name, func(t *testing.T) {
			g := newRig(t, board("board", "board"))
			g.hover(from, over)
			if got := g.landing(); got != "move:done" {
				t.Fatalf("drop target = %q, want move:done", got)
			}
			for _, e := range evs {
				g.router.Queue(e)
			}
			g.frame()
			g.frame()
			if got := g.landing(); got != "" {
				t.Errorf("drop target = %q after the item was %s, want none", got, name)
			}
		})
	}
}

func TestOnlyTheDropTargetIsLit(t *testing.T) {
	g := newRig(t, board("board", "board"))
	rest := g.shot()
	g.hover(f32.Pt(75, 15), f32.Pt(245, 40))
	lit := g.shot()
	// Under the last item of each column, where no card and no carried item is.
	if a, b := rest.RGBAAt(245, 150), lit.RGBAAt(245, 150); a == b {
		t.Errorf("the list under the item is drawn as at rest, %v", a)
	}
	if a, b := rest.RGBAAt(75, 150), lit.RGBAAt(75, 150); a != b {
		t.Errorf("the list the item left is lit: %v at rest, %v now", a, b)
	}
}

// TestACardStandsOutOfTheLitList keeps a filled card readable on the tray: a
// board's cards are commonly filled with "accent", the tray's own hue.
func TestACardStandsOutOfTheLitList(t *testing.T) {
	filled := func(k string) extension.Node {
		return extension.Box.New(extension.BoxProps{Height: 30, Background: "accent"}, label(k)).WithKey(k)
	}
	column := func(key string, cards ...string) extension.Node {
		kids := make([]extension.Node, len(cards))
		for i, c := range cards {
			kids[i] = filled(c)
		}
		return extension.Box.New(extension.BoxProps{Width: 150}, extension.SortableList.New(extension.SortableListProps{
			Group: "board", OnMove: extension.Handler("move:" + key)}, kids...).WithKey(key))
	}
	g := newRig(t, extension.Box.New(extension.BoxProps{Direction: extension.DirectionRow, Gap: 20},
		column("todo", "a", "b"), column("done", "x", "y")))
	g.hover(f32.Pt(75, 15), f32.Pt(245, 40))
	lit := g.shot()
	// The right end of the target's first card, past its label, and the tray under its last card.
	if card, tray := lit.RGBAAt(310, 15), lit.RGBAAt(245, 150); card == tray {
		t.Errorf("a card in the lit list is the tray's color, %v", card)
	}
}

// TestALitListInAScrollEndsWithItsViewport keeps the tray's bottom edge on
// screen: a scroll gives its children no end to fill up to.
func TestALitListInAScrollEndsWithItsViewport(t *testing.T) {
	tree := func() extension.Node {
		todo := extension.Box.New(extension.BoxProps{Width: 150}, sortable("todo", "board", "a", "b", "c"))
		done := extension.Box.New(extension.BoxProps{Width: 150, Height: 200},
			extension.Scroll.New(extension.ScrollProps{}, sortable("done", "board", "x", "y")))
		return extension.Box.New(extension.BoxProps{Direction: extension.DirectionRow, Gap: 20}, todo, done)
	}
	g := newRig(t, tree())
	g.hover(f32.Pt(75, 15), f32.Pt(245, 40))
	lit := g.shot()
	// The viewport's last row, and the tray's fill well above it.
	if edge, fill := lit.RGBAAt(245, 199), lit.RGBAAt(245, 150); edge == fill {
		t.Errorf("the tray has no bottom edge at the end of the scroll: %v there as in its fill", edge)
	}
	g = newRig(t, tree())
	g.drag(f32.Pt(75, 15), f32.Pt(245, 190))
	if e := g.once("move:done"); e.Index != 2 {
		t.Errorf("move = %+v, want the end of the list for a drop at the viewport's end", e)
	}
}

// pressable is a card that opens on a press, as a board's card does.
func pressable(k string) extension.Node {
	return extension.NavRow.New(extension.NavRowProps{Height: 30, OnPress: extension.Handler("open:" + k)}, label(k)).WithKey(k)
}

func pressableBoard() extension.Node {
	column := func(key string, cards ...string) extension.Node {
		kids := make([]extension.Node, len(cards))
		for i, c := range cards {
			kids[i] = pressable(c)
		}
		return extension.Box.New(extension.BoxProps{Width: 150}, extension.SortableList.New(extension.SortableListProps{Group: "board",
			OnReorder: extension.Handler("reorder:" + key), OnMove: extension.Handler("move:" + key)}, kids...).WithKey(key))
	}
	return extension.Box.New(extension.BoxProps{Direction: extension.DirectionRow, Gap: 20}, column("todo", "a", "b", "c"), column("done", "x", "y"))
}

func TestADragIsNotAClickOnTheCard(t *testing.T) {
	cases := []struct {
		name     string
		from, to f32.Point
		want     extension.Handler
	}{
		{"into another list", f32.Pt(75, 15), f32.Pt(245, 40), "move:done"},
		{"within the list", f32.Pt(75, 15), f32.Pt(75, 75), "reorder:todo"},
		{"over its own slot", f32.Pt(75, 15), f32.Pt(75, 25), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t, pressableBoard())
			g.drag(tc.from, tc.to)
			if got := g.fired("open:a"); len(got) != 0 {
				t.Errorf("the dragged card opened: %+v", got)
			}
			if tc.want != "" {
				g.once(tc.want)
			}
		})
	}
}

func TestAClickOnACardStillPressesIt(t *testing.T) {
	g := newRig(t, pressableBoard())
	g.click(pointer.ButtonPrimary, 75, 15)
	g.once("open:a")
	if len(g.got) != 1 {
		t.Errorf("events = %+v, want only the press", g.got)
	}
}

// filledBoard is two columns of filled cards, "todo" at x 0..150 and "done" at
// 170..320, so a probe of a pixel tells whether a card is there.
func filledBoard(todo, done []string) extension.Node {
	column := func(key string, cards []string) extension.Node {
		kids := make([]extension.Node, len(cards))
		for i, c := range cards {
			kids[i] = extension.Box.New(extension.BoxProps{Height: 30, Background: "accent"}, label(c)).WithKey(c)
		}
		return extension.Box.New(extension.BoxProps{Width: 150}, extension.SortableList.New(extension.SortableListProps{
			Group: "board", OnMove: extension.Handler("move:" + key)}, kids...).WithKey(key))
	}
	return extension.Box.New(extension.BoxProps{Direction: extension.DirectionRow, Gap: 20}, column("todo", todo), column("done", done))
}

// A card a render puts in another list of the group slides there from where it
// was, as it does inside one list; one the user dropped there does not.
func TestACardAnotherRenderMovesSlidesToItsList(t *testing.T) {
	// The slot "a" takes at the end of "done", clear of the cards that step aside.
	probe := func(g *rig) [4]uint8 { c := g.shot().RGBAAt(310, 75); return [4]uint8{c.R, c.G, c.B, c.A} }
	moved := func() extension.Node { return filledBoard([]string{"b", "c"}, []string{"x", "y", "a"}) }
	rest := func() extension.Node { return filledBoard([]string{"a", "b", "c"}, []string{"x", "y"}) }

	g := newRig(t, moved())
	settled := probe(g)
	g = newRig(t, rest())
	// The router knows where a list sits only from the second frame.
	g.wait(time.Millisecond)
	g.tree = moved()
	g.wait(time.Millisecond)
	if got := probe(g); got == settled {
		t.Errorf("the card is at its new slot in the frame its list changed, %v: it should still be near where it was", got)
	}
	g.wait(time.Second)
	if got := probe(g); got != settled {
		t.Errorf("the card is %v a second on, want it settled in its slot (%v)", got, settled)
	}
}

func TestACardTheUserDroppedDoesNotSlide(t *testing.T) {
	g := newRig(t, filledBoard([]string{"a", "b", "c"}, []string{"x", "y"}))
	g.drag(f32.Pt(75, 15), f32.Pt(245, 200))
	g.once("move:done")
	settled := newRig(t, filledBoard([]string{"b", "c"}, []string{"x", "y", "a"}))
	want := settled.shot().RGBAAt(310, 75)
	g.tree = filledBoard([]string{"b", "c"}, []string{"x", "y", "a"})
	g.wait(time.Millisecond)
	if got := g.shot().RGBAAt(310, 75); got != want {
		t.Errorf("the dropped card is %v in the frame the extension applied the move, want it in its slot at once (%v)", got, want)
	}
}

func TestACardOnlyRenderedBeforeDoesNotSlide(t *testing.T) {
	// "a" leaves the board for a render and comes back in another list: it is new there.
	g := newRig(t, filledBoard([]string{"a", "b", "c"}, []string{"x", "y"}))
	g.tree = filledBoard([]string{"b", "c"}, []string{"x", "y"})
	g.wait(time.Millisecond)
	g.tree = filledBoard([]string{"b", "c"}, []string{"x", "y", "a"})
	g.wait(time.Millisecond)
	settled := newRig(t, filledBoard([]string{"b", "c"}, []string{"x", "y", "a"})).shot().RGBAAt(310, 75)
	if got := g.shot().RGBAAt(310, 75); got != settled {
		t.Errorf("a card that was not on the board last frame slides in, %v want %v", got, settled)
	}
}
