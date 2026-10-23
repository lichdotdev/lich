package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func views(list ...string) []sessionView {
	out := make([]sessionView, 0, len(list))
	for _, id := range list {
		out = append(out, sessionView{Session: backend.Session{ID: id}})
	}
	return out
}

func viewIDs(list []sessionView) string {
	out := ""
	for i, v := range list {
		if i > 0 {
			out += ","
		}
		out += v.ID
	}
	return out
}

func TestDepartedNamesWhatLeftWithItsSlot(t *testing.T) {
	gone := departed(views("a", "b", "c"), views("a", "c"))
	if len(gone) != 1 || gone[0].session.ID != "b" || gone[0].index != 1 {
		t.Fatalf("departed = %+v", gone)
	}
	if gone := departed(views("a", "b"), views("b", "a", "c")); len(gone) != 0 {
		t.Fatalf("a grown or reordered list departed %+v", gone)
	}
}

func TestWithClosingPutsCardsBackInSlotOrder(t *testing.T) {
	a, c := views("a")[0], views("c")[0]
	if got := withClosing(views("b"), []closingCard{{c, 2}, {a, 0}}); viewIDs(got) != "a,b,c" {
		t.Fatalf("shown = %s", viewIDs(got))
	}
	if got := withClosing(views("a"), []closingCard{{c, 2}}); viewIDs(got) != "a,c" {
		t.Fatalf("past the end = %s", viewIDs(got))
	}
}

func TestHoldDepartedKeepsSeparateClosesInDrawnOrder(t *testing.T) {
	first, _ := holdDeparted(nil, views("a", "b", "c", "d"), views("a", "c", "d"))
	second, _ := holdDeparted(first, views("a", "c", "d"), views("a", "d"))
	if got := withClosing(views("a", "d"), second); viewIDs(got) != "a,b,c,d" {
		t.Fatalf("shown = %s", viewIDs(got))
	}
	if _, changed := holdDeparted(second, views("a"), views("a", "c")); changed {
		t.Fatal("a list that only grew changed what is held")
	}
}

// Back while its card was still going, then closed again: one entry, in the
// slot it holds now, so its exit is timed from the second close.
func TestHoldDepartedHoldsACardClosedTwiceOnce(t *testing.T) {
	held, _ := holdDeparted(nil, views("a", "b", "c"), views("a", "c"))
	again, changed := holdDeparted(held, views("c", "a", "b"), views("c", "a"))
	if !changed || len(again) != 1 || again[0].session.ID != "b" || again[0].index != 2 {
		t.Fatalf("held = %+v (changed %v)", again, changed)
	}
}

func TestClosingCardsPlaysTheExitThenDrops(t *testing.T) {
	var c closingCards
	t0 := time.Unix(1000, 0)
	c.shown(views("a", "b", "c"), t0)
	shown, closing := c.shown(views("a", "c"), t0)
	if viewIDs(shown) != "a,b,c" || !closing["b"] || len(closing) != 1 {
		t.Fatalf("closing frame: %s %v", viewIDs(shown), closing)
	}
	if got := c.expiry(); !got.Equal(t0.Add(exitDuration)) {
		t.Fatalf("expiry = %v", got)
	}
	shown, _ = c.shown(views("a", "c"), t0.Add(exitDuration-time.Millisecond))
	if viewIDs(shown) != "a,b,c" {
		t.Fatalf("mid-exit: %s", viewIDs(shown))
	}
	shown, closing = c.shown(views("a", "c"), t0.Add(exitDuration))
	if viewIDs(shown) != "a,c" || len(closing) != 0 || !c.expiry().IsZero() {
		t.Fatalf("after exit: %s %v", viewIDs(shown), closing)
	}
}

// A second close restarts the clock for every held card.
func TestClosingCardsRestartsTheClockOnAnotherClose(t *testing.T) {
	var c closingCards
	t0 := time.Unix(1000, 0)
	c.shown(views("a", "b", "c"), t0)
	c.shown(views("a", "c"), t0)
	later := t0.Add(exitDuration / 2)
	c.shown(views("a"), later)
	shown, _ := c.shown(views("a"), t0.Add(exitDuration))
	if viewIDs(shown) != "a,b,c" {
		t.Fatalf("both cards should still be going: %s", viewIDs(shown))
	}
}

func TestClosingCardsLetsAReturningSessionBeLive(t *testing.T) {
	var c closingCards
	t0 := time.Unix(1000, 0)
	c.shown(views("a", "b"), t0)
	c.shown(views("a"), t0)
	shown, closing := c.shown(views("a", "b"), t0)
	if viewIDs(shown) != "a,b" || closing["b"] {
		t.Fatalf("returned: %s %v", viewIDs(shown), closing)
	}
}

func TestExitDurationIsPinned(t *testing.T) {
	// Pinned, not derived: the card's exit animation is timed from it.
	if exitDuration != 180*time.Millisecond {
		t.Fatalf("exitDuration = %v, want 180ms", exitDuration)
	}
}

func shownKeys(shown []blockView) string {
	return strings.Join(blockKeys(sidebarView{blocks: shown}), ",")
}

func TestBlockPresenceHoldsALeavingBlockAsItLastWas(t *testing.T) {
	var p blockPresence
	t0 := time.Unix(100, 0)
	a, b := blockView{key: "a"}, blockView{key: "b", sessions: views("s1")}
	p.shown(t0, "p", []blockView{a, b})
	shown := p.shown(t0, "p", []blockView{a})
	if shownKeys(shown) != "a,b" || viewIDs(shown[1].sessions) != "s1" || !p.motion.Leaving("b") {
		t.Fatalf("closing frame: %s, leaving b %v", shownKeys(shown), p.motion.Leaving("b"))
	}
	if shown = p.shown(t0.Add(ui.PresenceExit), "p", []blockView{a}); shownKeys(shown) != "a" {
		t.Fatalf("after the exit: %s, want a", shownKeys(shown))
	}
}

func TestBlockPresenceDropsTheLastProjectsBlocks(t *testing.T) {
	var p blockPresence
	t0 := time.Unix(100, 0)
	p.shown(t0, "p", []blockView{{key: "a"}})
	if shown := p.shown(t0, "q", []blockView{{key: "c"}}); shownKeys(shown) != "c" {
		t.Fatalf("after switching projects: %s, want c alone", shownKeys(shown))
	}
}

// sidebarHeights lays the block list out at each step after the switch from
// before to after, and returns its height at each.
func sidebarHeights(before, after sidebarView) []int {
	s := newSidebar(&ui.Root{})
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	var r input.Router
	t0 := time.Unix(100, 0)
	frame := func(v sidebarView, at time.Duration) int {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Source: r.Source(), Now: t0.Add(at), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(300, 4000)}}
		var act sidebarAction
		s.blockList(gtx, th, v, "", &act)
		r.Frame(&ops)
		return s.list.Position.Length
	}
	frame(before, 0)
	var heights []int
	for at := time.Duration(0); at <= 2*ui.PresenceExit; at += ui.PresenceExit / 4 {
		heights = append(heights, frame(after, at))
	}
	return heights
}

func blocksOf(project string, blocks [][]string) sidebarView {
	v := sidebarView{project: project, showHeader: true}
	for i, ids := range blocks {
		b := blockView{key: string(rune('A' + i)), kind: blockCheckout}
		for _, id := range ids {
			b.sessions = append(b.sessions, sessionView{Session: backend.Session{ID: id}, projectID: project})
		}
		v.blocks = append(v.blocks, b)
	}
	return v
}

func moves(heights []int) bool {
	first, last := heights[0], heights[len(heights)-1]
	for _, h := range heights[1 : len(heights)-1] {
		if h != first && h != last {
			return true
		}
	}
	return false
}

func TestABlockGrowsInAndShrinksOutWithItsCard(t *testing.T) {
	in := sidebarHeights(blocksOf("p", [][]string{{"a"}}), blocksOf("p", [][]string{{"a"}, {"b"}}))
	if in[0] >= in[len(in)-1] || !moves(in) {
		t.Errorf("a new block: heights %v, want growing from the list without it", in)
	}
	out := sidebarHeights(blocksOf("p", [][]string{{"a"}, {"b"}}), blocksOf("p", [][]string{{"a"}}))
	if out[0] <= out[len(out)-1] || !moves(out) {
		t.Errorf("a closed block: heights %v, want shrinking to the list without it", out)
	}
	if other := sidebarHeights(blocksOf("p", [][]string{{"a"}}), blocksOf("q", [][]string{{"a"}, {"b"}})); moves(other) {
		t.Errorf("another project's list: heights %v, want drawn whole at once", other)
	}
}

func TestABlocksHeaderUnfoldsWhenTheListSplits(t *testing.T) {
	alone := blocksOf("p", [][]string{{"a"}})
	alone.showHeader = false
	split := blocksOf("p", [][]string{{"a"}})
	if in := sidebarHeights(alone, split); in[0] >= in[len(in)-1] || !moves(in) {
		t.Errorf("headers shown: heights %v, want the header unfolding", in)
	}
	if out := sidebarHeights(split, alone); out[0] <= out[len(out)-1] || !moves(out) {
		t.Errorf("headers hidden: heights %v, want the header folding away", out)
	}
}
