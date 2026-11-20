package main

import (
	"image"
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
