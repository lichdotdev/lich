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

// pageDockFrames is peekDockFrames with session s's page alone in the island,
// where the window would place its web view.
func pageDockFrames(t *testing.T) (*window, *frameRig, *browserPaneUI) {
	t.Helper()
	u, r := peekDockFrames(t)
	u.browserZoom = defaultZoom
	page := browserPaneOf("s")
	v := stageView{hasLayout: true, layout: layoutTab{ID: "l", Root: &paneNode{Session: page}}}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			row := gtx
			row.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X-gtx.Dp(gutter), gtx.Constraints.Max.Y))
			if err := u.stageRow(row, v); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return u, r, u.browserPane("s")
}

// pageRect is where placeNativePage puts the page's web view after the last
// frame: the page area's corner, at the size the pane laid the page out at.
func pageRect(t *testing.T, r *frameRig, p *browserPaneUI) image.Rectangle {
	t.Helper()
	at, ok := r.router.Source().Origin(&p.tag)
	if !ok {
		t.Fatal("no page area drawn")
	}
	corner := at.Round()
	return image.Rectangle{Min: corner, Max: corner.Add(image.Pt(p.view.W, p.view.H))}
}

func TestCollapsedDockOutFromTheEdgePushesThePageIn(t *testing.T) {
	u, r, p := pageDockFrames(t)
	r.settle(t)
	before := pageRect(t, r, p)
	r.hover(edgePoint)
	r.settle(t)
	if !u.dock.peek.shown {
		t.Fatal("still in with the pointer on the window's edge")
	}
	if len(u.dock.islands) == 0 {
		t.Fatal("the dock out stands over the island, not beside it")
	}
	// The rig draws a pixel a dp.
	push := u.dock.width(layout.Context{Metric: unit.Metric{PxPerDp: 1}}) + int(gutter)
	if got, want := u.dock.islands[0].x1, rigWindow.X-int(gutter)-push; got != want {
		t.Errorf("the island ends at %d, want %d: the dock out is drawn over it", got, want)
	}
	out := pageRect(t, r, p)
	if want := (image.Rectangle{Min: before.Min, Max: before.Max.Sub(image.Pt(push, 0))}); out != want {
		t.Errorf("page at %v with the dock out, want %v", out, want)
	}
	r.hover(f32.Pt(100, 300))
	r.now = r.now.Add(peekCloseDelay)
	r.settle(t)
	if u.dock.peek.shown {
		t.Fatal("still out once the pointer left it")
	}
	if back := pageRect(t, r, p); back != before {
		t.Errorf("page at %v once the dock went back, want %v", back, before)
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
