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

	"github.com/lichdotdev/lich/native/backend"
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
	// Review's changes read once first: the fake backend answers at once, and
	// their first load landing between the two frames r.click compares reads
	// as a click that drew the old state.
	u.dock.tab = dockReview
	r.waitFor(t, "No uncommitted changes")
	u.dock.tab = dockCode
	r.settle(t)
	buttons := tabButtonsOf(r)
	if len(buttons) != dockBuiltinTabs {
		t.Fatalf("%d tab buttons at the dock's foot, want %d", len(buttons), dockBuiltinTabs)
	}
	for i, want := range []dockTab{dockReview, dockArtifacts, dockCode} {
		// The first Review frame starts a diff read, so its result may update the next frame.
		r.queueClick(buttons[int(want)-1])
		r.frame()
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

func TestDockReviewTabWaitsOverConflicts(t *testing.T) {
	d := &dock{artifacts: newArtifactsTab(new(artifactStore))}
	in := dockInput{isGit: true, git: backend.DiffStats{Files: 1, Conflicted: []string{"a.go"}}}
	review := d.tabSpecs(in)[1]
	if !review.mark || !review.waiting {
		t.Fatalf("Review over a conflict: mark %v, waiting %v; want both", review.mark, review.waiting)
	}
	in.git.Conflicted = nil
	if d.tabSpecs(in)[1].waiting {
		t.Error("Review without conflicts still waits")
	}
}
