package main

import (
	"image"
	"testing"

	"gioui.org/f32"

	"github.com/lichdotdev/lich/native/ui"
)

// peekFrames is the collapsed sidebar on a wide window, with its peek drawn
// over the rest as the window draws it; acted is the last session a peek
// click activated.
func peekFrames(t *testing.T) (s *sidebar, r *frameRig, acted *string) {
	t.Helper()
	m := modelOn("a")
	root := new(ui.Root)
	s = newSidebar(root)
	s.open = false
	acted = new(string)
	r = newFrameRig(image.Pt(800, 600), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layout(gtx, th, m, "a1")
			if a := s.peekLayout(gtx, th, m, "a1", image.Point{}); a.activate != "" {
				*acted = a.activate
			}
			return D{Size: gtx.Constraints.Max}
		})
	})
	return s, r, acted
}

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
	s, r, _ := peekFrames(t)
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
	s, r, _ := peekFrames(t)
	r.settle(t)
	rest(t, r, f32.Pt(25, 25))
	if s.peek.shown {
		t.Fatal("the expand button brought the sidebar out over itself")
	}
}

func TestPeekGoesBackOnceThePointerLeaves(t *testing.T) {
	s, r, _ := peekFrames(t)
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
	s, r, _ := peekFrames(t)
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
	s, r, _ := peekFrames(t)
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
