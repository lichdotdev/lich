package main

import (
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/unit"
)

func TestTabWidthShrinksEvenlyToItsFloor(t *testing.T) {
	cases := []struct{ room, count, want int }{
		{1000, 3, 196}, // room to spare: the fixed width
		{800, 5, 160},  // shared evenly
		{800, 8, 118},  // the floor; the strip scrolls past it
		{800, 0, 196},
	}
	for _, c := range cases {
		if got := tabWidth(c.room, c.count, 196, 118); got != c.want {
			t.Errorf("tabWidth(%d, %d) = %d, want %d", c.room, c.count, got, c.want)
		}
	}
}

func TestBrandColumnFollowsTheSidebar(t *testing.T) {
	const rail, panel = 32, 240
	cases := []struct {
		name           string
		open, morphing bool
		p              float32
		width          int
		alpha          float32
	}{
		{"open", true, false, 0, panel, 1},
		{"collapsed", false, false, 0, rail, 0},
		{"expanding, a quarter in", true, true, 0.25, 84, 0.25},
		{"collapsing, a quarter in", false, true, 0.25, 188, 0.75},
		{"collapsing, landed", false, true, 1, rail, 0},
	}
	for _, c := range cases {
		w, a := brandColumn(rail, panel, c.open, c.morphing, c.p)
		if w != c.width || a != c.alpha {
			t.Errorf("%s: width %d alpha %v, want %d %v", c.name, w, a, c.width, c.alpha)
		}
	}
}

// The active tab grows out of the island it sits wholly over, fillets
// included, and out of neither when it sits over the gap between two.
func TestActiveTabMergesOnlyIntoTheIslandUnderIt(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	r := gtx.Dp(tabRadius)
	list := islandSpan{x0: 0, x1: 300, color: color.NRGBA{R: 1, A: 255}}
	view := islandSpan{x0: 306, x1: 1000, color: color.NRGBA{R: 2, A: 255}}
	tabs := []projectTab{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	bar := &topbar{tabW: 100}
	cases := []struct {
		active string
		want   islandSpan
		merged bool
		square bool
	}{
		{"a", list, true, true},
		{"b", list, true, false},
		{"c", islandSpan{}, false, false}, // 200..300 plus its right fillet runs past the list
		{"d", islandSpan{}, false, false}, // 300..400 straddles the gap
	}
	for _, c := range cases {
		f := topbarFrame{tabs: tabs, active: c.active, islands: []islandSpan{list, view}}
		got, ok := bar.mergeSpan(gtx, f)
		if ok != c.merged || got != c.want {
			t.Errorf("tab %s: merged into %+v (%v), want %+v (%v)", c.active, got, ok, c.want, c.merged)
		}
		if _, sq := bar.islandSquare(gtx, f); sq != c.square {
			t.Errorf("tab %s: square %v, want %v", c.active, sq, c.square)
		}
	}
	over := topbarFrame{tabs: tabs, active: "d", islands: []islandSpan{list, {x0: 300 - r, x1: 1000}}}
	if s, ok := bar.mergeSpan(gtx, over); !ok || s.x0 != 300-r {
		t.Errorf("a tab wholly over the second island did not merge into it: %+v, %v", s, ok)
	}
	if _, ok := bar.mergeSpan(gtx, topbarFrame{tabs: tabs, active: "d"}); !ok {
		t.Error("with one island the active tab did not merge")
	}
}
