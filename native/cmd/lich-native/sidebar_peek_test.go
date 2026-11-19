package main

import (
	"image"
	"testing"
	"time"

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
// click activated. stage, when set, is laid out under the peek, as the
// window lays the stage out before it.
func peekFrames(t *testing.T, stage func(C)) (s *sidebar, r *frameRig, acted *string) {
	t.Helper()
	m := modelOn("a")
	root := new(ui.Root)
	s = newSidebar(root)
	s.open = false
	acted = new(string)
	r = newFrameRig(image.Pt(800, 600), func(gtx C, th *ui.Theme) {
		// As the window does: the morph advances before the frame, and
		// reads its capture frame once it is drawn.
		s.morph.advance(gtx)
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
		s.morph.capture(gtx.Ops)
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

// morphRun steps frameStep at a time from a pointer move to p until the
// sidebar's morph lands, and reports its box's width on each frame it flew
// and the stage's room on every frame.
func morphRun(t *testing.T, s *sidebar, r *frameRig, p f32.Point) (widths, room []int) {
	t.Helper()
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1}}
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p})
	for range 100 {
		r.frame()
		room = append(room, s.stageReserve(gtx))
		if s.morph.landed {
			return widths, room
		}
		if s.morph.phase == morphFlying && !s.morph.capturing {
			widths = append(widths, s.morphWidth(gtx))
		}
		r.now = r.now.Add(frameStep)
	}
	t.Fatal("the morph never landed")
	return nil, nil
}

// The pointer on the rail runs the expand button's morph, over the stage,
// which keeps the rail's room; leaving runs the collapse back.
func TestPeekRunsTheExpandMorphOverTheStage(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	r.settle(t)
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1}}
	rail, panel := s.boxWidth(gtx, false), s.boxWidth(gtx, true)
	for _, way := range []struct {
		name     string
		at       f32.Point
		from, to int
	}{{"out", railTree, rail, panel}, {"back", f32.Pt(700, 300), panel, rail}} {
		widths, room := morphRun(t, s, r, way.at)
		if !s.peek.morph {
			t.Fatalf("%s: the morph that ran is not the peek's", way.name)
		}
		if len(widths) < 5 {
			t.Fatalf("%s: the morph flew %d frames of 180ms, want one a frame", way.name, len(widths))
		}
		for i, w := range widths {
			if want := lerpInt(way.from, way.to, morphProgress(time.Duration(i)*frameStep)); w != want {
				t.Errorf("%s, frame %d: the sidebar %dpx wide, want the expand's %d", way.name, i, w, want)
			}
		}
		for i, x := range room {
			if x != rail {
				t.Fatalf("%s, frame %d: the stage leaves %dpx, want the rail's %d throughout", way.name, i, x, rail)
			}
		}
	}
}

// Kept open from the peek, the sidebar takes its room from the stage.
func TestPeekPinnedTakesTheStagesRoom(t *testing.T) {
	s, r, _ := peekFrames(t, nil)
	r.settle(t)
	rest(t, r, railTree)
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1}}
	if got, rail := s.stageReserve(gtx), s.boxWidth(gtx, false); got != rail {
		t.Fatalf("out from the rail, the stage leaves %dpx, want the rail's %d", got, rail)
	}
	s.setOpen(gtx, true, new(sidebarAction))
	r.settle(t)
	if got, panel := s.stageReserve(gtx), s.boxWidth(gtx, true); got != panel {
		t.Errorf("kept open, the stage leaves %dpx, want the panel's %d", got, panel)
	}
}
