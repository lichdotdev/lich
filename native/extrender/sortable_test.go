package extrender

import (
	"slices"
	"testing"

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
