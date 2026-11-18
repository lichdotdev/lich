package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/ui"
)

// collapseButton is the dock header's collapse, left of full screen.
var collapseButton = image.Rect(rigWindow.X-92, 0, rigWindow.X-64, 48)

func TestDockCollapseTakesItOffTheStageInTheFrameAfterTheClick(t *testing.T) {
	u, r := stageFrames(t, dockReview)
	r.waitFor(t, "No uncommitted changes")
	r.click(t, r.smallestClickIn(t, collapseButton))
	if !u.dock.collapsed {
		t.Fatal("the click missed collapse")
	}
	r.hover(f32.Pt(100, 300))
	r.settle(t)
	if u.dock.islands != nil || u.dock.visible() {
		t.Fatalf("still on the stage once collapsed: islands %v", u.dock.islands)
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

// windowEdge is a point in the window's right gutter, where a pointer thrown
// to the side lands.
var windowEdge = f32.Pt(float32(rigWindow.X-2), 300)

func TestCollapsedDockComesOutFromTheWindowEdge(t *testing.T) {
	u, r := peekDockFrames(t)
	r.settle(t)
	if u.dock.visible() {
		t.Fatal("out before the pointer came to the edge")
	}
	r.hover(windowEdge)
	r.now = r.now.Add(peekOpenDelay)
	r.waitFor(t, "No uncommitted changes")
	if !u.dock.peek.shown {
		t.Fatal("still in once the pointer rested on the window's edge")
	}
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
	r.hover(windowEdge)
	r.now = r.now.Add(peekOpenDelay)
	r.waitFor(t, "No uncommitted changes")
	r.click(t, r.smallestClickIn(t, image.Rect(rigWindow.X-92-6, 0, rigWindow.X-64-6, 48)))
	if u.dock.collapsed {
		t.Fatal("the click missed expand")
	}
	if u.dock.slide.shown != 1 {
		t.Errorf("slid in from the edge, though it already stands where it lands: share %v", u.dock.slide.shown)
	}
}

func TestFooterBringsTheCollapsedDockOutAndClosesIt(t *testing.T) {
	d := &dock{collapsed: true}
	d.toggle(dockReview)
	if !d.showing(dockReview) {
		t.Fatal("the footer's Review did not bring the collapsed dock out")
	}
	d.toggle(dockCode)
	if !d.showing(dockCode) {
		t.Fatal("Code from the footer did not switch the dock that is out")
	}
	d.toggle(dockCode)
	if d.tab != dockClosed {
		t.Fatalf("Code again left the dock on %v", d.tab)
	}
}

func TestFooterBringsOutTheCollapsedDockItsTabIsHiddenIn(t *testing.T) {
	d := &dock{collapsed: true, tab: dockReview}
	d.toggle(dockReview)
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
