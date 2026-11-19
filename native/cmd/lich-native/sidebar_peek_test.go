package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// peekFrames is the collapsed sidebar on a wide window, with its peek drawn
// over the rest as the window draws it; acted is the last session a peek
// click activated. stage, when set, is laid out before the peek, under
// where it comes out.
func peekFrames(t *testing.T, stage func(C)) (s *sidebar, r *frameRig, acted *string) {
	t.Helper()
	m := modelOn("a")
	root := new(ui.Root)
	s = newSidebar(root)
	s.open = false
	acted = new(string)
	r = newFrameRig(image.Pt(800, 600), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layout(gtx, th, m, "a1")
			if stage != nil {
				stage(gtx)
			}
			if a := s.peekLayout(gtx, th, m, "a1", image.Point{}); a.activate != "" {
				*acted = a.activate
			}
			return D{Size: gtx.Constraints.Max}
		})
	})
	return s, r, acted
}

// railWidthPx is the rail's width in the rig, at a pixel a dp.
const railWidthPx = 50

// railTree is a point on the rail's tree, under its header row.
var railTree = f32.Pt(25, 90)

// rest leaves the pointer at p until the peek's delay has passed.
func rest(t *testing.T, r *frameRig, p f32.Point) {
	t.Helper()
	r.hover(p)
	r.now = r.now.Add(peekOpenDelay)
	r.settle(t)
}

func TestRailBringsTheSidebarOutOnceThePointerRestsOnItsTree(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	r.settle(t)
	r.hover(railTree)
	if s.peek.shown {
		t.Fatal("out before the pointer rested")
	}
	r.now = r.now.Add(peekOpenDelay)
	r.settle(t)
	if !s.peek.shown {
		t.Fatal("still in once the pointer rested on the rail's tree")
	}
	if s.open {
		t.Error("coming out opened the sidebar for good")
	}
}

func TestRailExpandButtonBringsNothingOut(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	r.settle(t)
	rest(t, r, f32.Pt(25, 25))
	if s.peek.shown {
		t.Fatal("the expand button brought the sidebar out over itself")
	}
}

func TestPeekGoesBackOnceThePointerLeaves(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	r.settle(t)
	rest(t, r, railTree)
	r.hover(f32.Pt(700, 300))
	r.now = r.now.Add(peekCloseDelay)
	r.settle(t)
	if s.peek.shown {
		t.Fatal("still out once the pointer left it")
	}
}

func TestPeekStaysOutWhileThePointerIsOnIt(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	r.settle(t)
	rest(t, r, railTree)
	r.hover(f32.Pt(200, 300))
	r.now = r.now.Add(peekCloseDelay)
	r.settle(t)
	if !s.peek.shown {
		t.Fatal("went back with the pointer on the panel")
	}
}

func TestPeekCollapseButtonKeepsTheSidebarOpen(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	r.settle(t)
	rest(t, r, railTree)
	w := int(s.widthRem * remDp)
	r.click(t, r.smallestClickIn(t, image.Rect(w-40, 0, w, 50)))
	if !s.open {
		t.Fatal("the button in the header did not keep the sidebar open")
	}
	if s.morph.active() {
		t.Error("kept open with a morph, though the panel already stands where it lands")
	}
}

func TestPeekSessionClickActivatesItAndSendsThePanelBack(t *testing.T) {
	s, r, acted := peekFrames(t, nil)
	r.settle(t)
	rest(t, r, railTree)
	r.click(t, r.at(t, "A two"))
	if *acted != "a2" {
		t.Fatalf("activated %q, want a2", *acted)
	}
	if s.peek.shown {
		t.Error("still out after a session was picked in it")
	}
}

func TestPeekKeepsItsClicksFromTheTerminalUnderIt(t *testing.T) {
	presses := new(int)
	// The terminal stands right of the rail, under where the panel comes out.
	terminal := func(gtx C) {
		area := clip.Rect{Min: image.Pt(railWidthPx, 0), Max: gtx.Constraints.Max}.Push(gtx.Ops)
		event.Op(gtx.Ops, presses)
		area.Pop()
		for {
			ev, ok := gtx.Event(pointer.Filter{Target: presses, Kinds: pointer.Press})
			if !ok {
				break
			}
			if _, ok := ev.(pointer.Event); ok {
				*presses++
			}
		}
	}
	s, r, _ := peekFrames(t, terminal)
	r.settle(t)
	rest(t, r, railTree)
	r.click(t, f32.Pt(150, 450))
	if *presses != 0 || !s.peek.shown {
		t.Fatalf("a click on the panel reached the terminal %d times; out %v", *presses, s.peek.shown)
	}
	r.click(t, f32.Pt(600, 450))
	if *presses != 1 {
		t.Fatalf("a click beside the panel reached the terminal %d times, want 1", *presses)
	}
}

func TestPeekTakesTheStagesRoomAndGivesItBack(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	r.settle(t)
	// The rig draws a pixel a dp.
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1}}
	rail, panel := s.boxWidth(gtx, false), s.boxWidth(gtx, true)
	rest(t, r, railTree)
	if got := s.stageReserve(gtx); got != panel {
		t.Fatalf("the stage leaves the sidebar out %dpx, want the open sidebar's %d", got, panel)
	}
	r.hover(f32.Pt(700, 300))
	r.now = r.now.Add(peekCloseDelay)
	r.frame()
	r.now = r.now.Add(frameStep)
	r.frame()
	reserve := s.stageReserve(gtx)
	if reserve <= rail || reserve >= panel {
		t.Fatalf("on its way back the stage leaves %dpx, want between the rail's %d and the panel's %d", reserve, rail, panel)
	}
	r.settle(t)
	if got := s.stageReserve(gtx); got != rail {
		t.Errorf("the stage leaves %dpx once the sidebar went back, want the rail's %d", got, rail)
	}
}

func TestSidebarOpenedWhileOutStaysWhereItIs(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	r.settle(t)
	rest(t, r, railTree)
	var act sidebarAction
	s.setOpen(layout.Context{}, true, &act)
	if !s.open || !act.saveOpen {
		t.Fatal("the sidebar out did not open")
	}
	if s.morph.active() {
		t.Error("opened with a morph from the rail, though it already stands where it opens")
	}
}
