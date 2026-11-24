package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// collapseButton is the dock header's collapse, left of full screen.
var collapseButton = image.Rect(rigWindow.X-72, 0, rigWindow.X-36, 48)

func TestDockCollapseTakesItOffTheStageInTheFrameAfterTheClick(t *testing.T) {
	u, r := stageFrames(t, dockReview)
	r.waitFor(t, "No uncommitted changes")
	r.click(t, r.smallestClickIn(t, collapseButton))
	if !u.dock.collapsed {
		t.Fatal("the click missed collapse")
	}
	r.hover(f32.Pt(100, 300))
	r.settle(t)
	if u.dock.slide.shown != 0 || u.dock.visible() {
		t.Fatalf("still on the stage once collapsed: %v of it out", u.dock.slide.shown)
	}
	if u.dock.tab != dockReview {
		t.Errorf("collapsing closed the dock: on %v", u.dock.tab)
	}
}

// peekDockFrames is the stage row with the dock collapsed on Review, the
// window's right gutter left free beside it as the window leaves it.
func peekDockFrames(t *testing.T) (*window, *frameRig) {
	t.Helper()
	u, r := stageFrames(t, dockReview)
	u.dock.collapsed, u.dock.peek.tab = true, dockReview
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			row := gtx
			row.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X-gtx.Dp(gutter), gtx.Constraints.Max.Y))
			if err := u.stageRow(row, stageView{}); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return u, r
}

// edgePoint is a point in the window's right gutter, where a pointer thrown
// to the side lands.
var edgePoint = f32.Pt(float32(rigWindow.X-2), 300)

func TestCollapsedDockComesOutFromTheWindowEdge(t *testing.T) {
	u, r := peekDockFrames(t)
	r.settle(t)
	if u.dock.visible() {
		t.Fatal("out before the pointer came to the edge")
	}
	r.hover(edgePoint)
	if !u.dock.peek.shown {
		t.Fatal("still in the moment the pointer touched the window's edge")
	}
	r.waitFor(t, "No uncommitted changes")
	r.hover(f32.Pt(100, 300))
	r.now = r.now.Add(peekCloseDelay)
	r.settle(t)
	if u.dock.peek.shown {
		t.Fatal("still out once the pointer left it")
	}
}

func TestDockExpandFromThePeekPutsItBackOnTheStage(t *testing.T) {
	u, r := peekDockFrames(t)
	r.settle(t)
	r.hover(edgePoint)
	r.now = r.now.Add(peekOpenDelay)
	r.waitFor(t, "No uncommitted changes")
	r.click(t, r.smallestClickIn(t, image.Rect(rigWindow.X-72-6, 0, rigWindow.X-36-6, 48)))
	if u.dock.collapsed {
		t.Fatal("the click missed expand")
	}
	if u.dock.slide.shown != 1 {
		t.Errorf("slid in from the edge, though it already stands where it lands: share %v", u.dock.slide.shown)
	}
}

func TestCollapsedDockStaysAtTheEdgeWhenTheWindowOpens(t *testing.T) {
	u, r := stageFrames(t, dockClosed)
	u.dock.collapsed = true
	u.dock.restore(dockReview)
	r.settle(t)
	if u.dock.visible() {
		t.Error("the stored tab brought the collapsed dock out at start")
	}
}

func TestBalloonBringsTheCollapsedDockOutAndNeverClosesIt(t *testing.T) {
	d := &dock{collapsed: true}
	d.show(dockReview)
	if !d.showing(dockReview) {
		t.Fatal("the balloon's Review did not bring the collapsed dock out")
	}
	d.show(dockCode)
	if !d.showing(dockCode) {
		t.Fatal("Code did not switch the dock that is out")
	}
	d.show(dockCode)
	if !d.showing(dockCode) {
		t.Fatalf("Code again left the dock on %v", d.tab)
	}
}

func TestBalloonBringsOutTheCollapsedDockItsTabIsHiddenIn(t *testing.T) {
	d := &dock{collapsed: true, tab: dockReview}
	d.show(dockReview)
	if d.tab != dockReview || !d.visible() {
		t.Fatalf("Review on the hidden Review tab closed it: on %v, visible %v", d.tab, d.visible())
	}
}

func TestDockCollapsedIsStored(t *testing.T) {
	_, client := newSidebarBackend(t)
	if got, err := loadDockCollapsed(t.Context(), client); err != nil || got {
		t.Fatalf("unset reads %v, %v; want docked", got, err)
	}
	if err := saveDockCollapsed(t.Context(), client, true); err != nil {
		t.Fatal(err)
	}
	if got, err := loadDockCollapsed(t.Context(), client); err != nil || !got {
		t.Fatalf("stored collapsed reads %v, %v", got, err)
	}
}

func TestDockShownFromElsewhereComesOutOnTheTabItIsHiddenIn(t *testing.T) {
	u, r := peekDockFrames(t)
	r.settle(t)
	u.dock.show(dockReview)
	r.waitFor(t, "No uncommitted changes")
	if !u.dock.peek.shown {
		t.Fatal("a path opened in the hidden Code tab left the collapsed dock in")
	}
}

func TestDockComesOutForAPointerThatOnlyCrossesTheEdge(t *testing.T) {
	u, r := peekDockFrames(t)
	r.settle(t)
	// Both moves land in one frame, as a flick past the edge does.
	r.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: edgePoint},
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(900, 300)},
	)
	r.frame()
	r.frame()
	if !u.dock.peek.shown {
		t.Fatal("a pointer that crossed the edge brought nothing out")
	}
}

func TestDockStaysOutWhileThePointerIsOutOfTheWindowPastTheEdge(t *testing.T) {
	u, r := peekDockFrames(t)
	r.settle(t)
	r.hover(edgePoint)
	r.router.Queue(pointer.Event{Kind: pointer.Leave, Source: pointer.Mouse, Position: edgePoint})
	r.frame()
	r.now = r.now.Add(time.Minute)
	r.settle(t)
	if !u.dock.peek.shown {
		t.Fatal("went back while the pointer was out of the window past the edge")
	}
	r.hover(f32.Pt(100, 300))
	r.now = r.now.Add(peekCloseDelay)
	r.settle(t)
	if u.dock.peek.shown {
		t.Fatal("still out once the pointer came back onto the stage")
	}
}

func TestDockCollapsedUnderThePointerStaysIn(t *testing.T) {
	u, r := stageFrames(t, dockReview)
	r.waitFor(t, "No uncommitted changes")
	r.click(t, r.smallestClickIn(t, collapseButton))
	r.now = r.now.Add(time.Minute)
	r.settle(t)
	if u.dock.visible() {
		t.Fatal("came straight back out under the pointer that collapsed it")
	}
}

func TestReviewPeekTakesSixtyPercentOfTheStage(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	cases := []struct {
		name  string
		tab   dockTab
		stage int
		want  int
	}{
		{"review on a wide stage", dockReview, 1500, 900},
		{"review wide enough to split on a smaller stage", dockReview, 1336, 820},
		{"review within a stage narrower than a split", dockReview, 700, 700},
		{"review capped at the dock's widest", dockReview, 3000, dockMaxRem * remDp},
		{"review never under the docked width", dockReview, 400, dockDefaultRem * remDp},
		{"another tab keeps the dock's width", dockCode, 1500, dockDefaultRem * remDp},
	}
	for _, c := range cases {
		d := &dock{widthRem: dockDefaultRem, minRem: dockMinRem, tab: c.tab}
		if got := d.peekWidth(gtx, c.stage); got != c.want {
			t.Errorf("%s: peekWidth = %d, want %d", c.name, got, c.want)
		}
	}
}
