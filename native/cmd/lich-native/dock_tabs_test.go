package main

import (
	"image"
	"slices"
	"sort"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/native/i18n"
)

// tabButtonsOf are the centres of the pill's buttons, left to right: the
// clickables in the dock's bottom strip, each button once: the semantic tree
// lists it twice, under the tooltip's trigger and its own.
func tabButtonsOf(r *frameRig) []f32.Point {
	strip := image.Rect(rigWindow.X/2, rigWindow.Y-80, rigWindow.X, rigWindow.Y)
	var found []image.Rectangle
	for _, n := range r.nodes() {
		if b := n.Desc.Bounds; n.Desc.Gestures&input.ClickGesture != 0 && b.In(strip) && !slices.Contains(found, b) {
			found = append(found, b)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Min.X < found[j].Min.X })
	out := make([]f32.Point, len(found))
	for i, b := range found {
		out[i] = centre(b)
	}
	return out
}

func TestDockTabButtonOpensItsTabInTheFrameThatTookTheClick(t *testing.T) {
	u, r := stageFrames(t, dockCode)
	r.settle(t)
	buttons := tabButtonsOf(r)
	if len(buttons) != dockBuiltinTabs {
		t.Fatalf("%d tab buttons at the dock's foot, want %d", len(buttons), dockBuiltinTabs)
	}
	for i, want := range []dockTab{dockReview, dockArtifacts, dockCode} {
		r.click(t, buttons[int(want)-1])
		if u.dock.tab != want {
			t.Fatalf("click %d: dock on %v, want %v", i, u.dock.tab, want)
		}
		if got := u.dock.selectedTab(nil); got != int(want)-1 {
			t.Errorf("click %d: pill lights tab %d, want %d", i, got, int(want)-1)
		}
	}
}

func TestDockTabButtonNamesItsTabInATooltip(t *testing.T) {
	_, r := stageFrames(t, dockCode)
	r.settle(t)
	review := tabButtonsOf(r)[1]
	name := i18n.T("dock.rightDock.tabReview")
	if r.shows(name) {
		t.Fatalf("%q drawn before the pointer came", name)
	}
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: review})
	r.frame()
	r.now = r.now.Add(time.Second)
	r.frame()
	r.settle(t)
	if !r.shows(name) {
		t.Errorf("no tooltip %q over the Review button:\n%s", name, r.drawn())
	}
}
