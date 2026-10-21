package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
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

// While the Pulls list slides in, the first tab stays merged the whole way
// and swaps from the pull request island to the list's once, instead of
// detaching while it fits wholly over neither.
func TestActiveTabStaysMergedWhileTheListSlides(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	const listW, gap, width = 320, 6, 1400
	list := color.NRGBA{R: 1, A: 255}
	view := color.NRGBA{R: 2, A: 255}
	bar := &topbar{tabW: 160}
	tabs := []projectTab{{ID: "a"}, {ID: "b"}}
	swaps, last := 0, view
	for slot := 1; slot < listW+gap; slot++ {
		spans := []islandSpan{{x0: slot - listW - gap, x1: slot - gap, color: list}, {x0: slot, x1: width, color: view}}
		s, ok := bar.mergeSpan(gtx, topbarFrame{tabs: tabs, active: "a", islands: spans, islandsSliding: true})
		if !ok {
			t.Fatalf("slot %d: the active tab detached mid-slide", slot)
		}
		if s.color != last {
			swaps, last = swaps+1, s.color
		}
	}
	if swaps != 1 || last != list {
		t.Errorf("the tab swapped islands %d times and landed on %v, want once onto the list", swaps, last)
	}
}

func TestSettingsButtonOpensOnGlobalAfterAProjectScope(t *testing.T) {
	u, _ := newGateWindow(t)
	u.applySettingsAction(topbarAction{projectSettings: "p"})
	if u.settings.scope != "p" {
		t.Fatalf("Project settings… opened on scope %q, want p", u.settings.scope)
	}
	u.applySettingsAction(topbarAction{settings: true})
	u.applySettingsAction(topbarAction{settings: true})
	if !u.settings.open || u.settings.scope != "" {
		t.Errorf("the settings button opened on scope %q (open %v), want Global", u.settings.scope, u.settings.open)
	}
}

// Past the window's own buttons, the strip leaves the active first tab's
// left fillet a radius of room after the brand, where it would otherwise be
// cut off at the strip's edge.
func TestStripStartLeavesTheFilletRoom(t *testing.T) {
	const island, radius = 44, 8
	if got := stripStart(island, 40, radius); got != island {
		t.Errorf("brand short of the island: start %d, want the island's edge %d", got, island)
	}
	if got := stripStart(island, 120, radius); got != 120+radius {
		t.Errorf("brand past the island: start %d, want %d", got, 120+radius)
	}
}

// The band's bare background is the window's title bar: past the last tab,
// and in the gaps around the brand and the buttons, a press drags the
// window; a tab keeps its own press.
func TestTopbarBareBandDragsTheWindow(t *testing.T) {
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(1400, 800))}
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Canvas}
	bar := newTopbar(&ui.Root{})
	f := topbarFrame{brandWidth: 200, stripX: 240, tabs: []projectTab{{ID: "a", Name: "a"}}, active: "a"}
	bar.layout(gtx, th, f)
	router.Frame(&ops)
	tab := bar.laid[0].rect
	middle := f32.Pt(0, float32(tab.Min.Y+tab.Dy()/2))
	for _, x := range []int{tab.Max.X + 200, gtx.Dp(gutter) / 2} {
		if a, ok := router.ActionAt(f32.Pt(float32(x), middle.Y)); !ok || a != system.ActionMove {
			t.Errorf("a press at x=%d does not drag the window", x)
		}
	}
	if _, ok := router.ActionAt(f32.Pt(float32(tab.Min.X+tab.Dx()/2), middle.Y)); ok {
		t.Error("a press on the tab drags the window")
	}
}
