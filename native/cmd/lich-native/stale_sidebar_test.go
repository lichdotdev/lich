package main

import (
	"image"
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// sidebarFrames is the open sidebar over three projects' sessions, a1
// waiting and b1 busy.
func sidebarFrames(t *testing.T) (*sidebar, *frameRig) {
	t.Helper()
	m := modelOn("a")
	report(t, m, "a1", statusWaiting)
	report(t, m, "b1", statusBusy)
	root := new(ui.Root)
	s := newSidebar(root)
	s.open = true
	r := newFrameRig(image.Pt(400, 700), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layout(gtx, th, m, "a1")
			return D{Size: gtx.Constraints.Max}
		})
	})
	return s, r
}

func TestSidebarFilterOpensInTheFrameAfterTheClick(t *testing.T) {
	s, r := sidebarFrames(t)
	r.settle(t)
	r.click(t, r.smallestClickIn(t, image.Rect(160, 0, 200, 50)))
	if !s.header.open {
		t.Fatal("the click missed the filter button")
	}
	if !r.router.Source().Focused(&s.header.query) {
		t.Error("the filter opened without its field focused")
	}
}

func TestSidebarPhaseChipFiltersInTheFrameThatTookTheClick(t *testing.T) {
	s, r := sidebarFrames(t)
	s.header.toggle()
	r.clickLabel(t, phaseChipLabel(phaseWaiting))
	if !s.header.phases[phaseWaiting] {
		t.Fatal("the click missed the chip")
	}
}

// withTerminal lays a session's terminal out right of the sidebar after it,
// taking the keyboard on every frame the sidebar does not hold it, as the
// window does. typing gives the terminal the keyboard first, as when you
// were typing in the session.
func withTerminal(s *sidebar, r *frameRig, typing *bool) (terminal *int) {
	sidebar, terminal := r.draw, new(int)
	r.draw = func(gtx C, th *ui.Theme) {
		sidebar(gtx, th)
		area := clip.Rect{Min: image.Pt(260, 0), Max: gtx.Constraints.Max}.Push(gtx.Ops)
		event.Op(gtx.Ops, terminal)
		area.Pop()
		for {
			if _, ok := gtx.Event(key.FocusFilter{Target: terminal}); !ok {
				break
			}
		}
		if *typing || !s.holdsKeys() {
			gtx.Execute(key.FocusCmd{Tag: terminal})
		}
	}
	return terminal
}

func TestSidebarFilterFieldKeepsTheKeyboardFromTheTerminal(t *testing.T) {
	s, r := sidebarFrames(t)
	s.header.toggle()
	typing := true
	withTerminal(s, r, &typing)
	r.settle(t)
	typing = false
	r.click(t, r.at(t, i18n.T("sidebar.sessionSidebar.filter")))
	r.settle(t)
	if !r.router.Source().Focused(&s.header.query) {
		t.Error("the terminal took the keyboard back from the filter field")
	}
}

// A press anywhere else in the sidebar leaves the field, as a page's click
// outside a field does, and the session takes the keyboard back.
func TestSidebarPressOutsideTheFilterFieldLeavesIt(t *testing.T) {
	s, r := sidebarFrames(t)
	s.header.toggle()
	typing := false
	terminal := withTerminal(s, r, &typing)
	r.settle(t)
	if !r.router.Source().Focused(&s.header.query) {
		t.Fatal("the filter opened without its field focused")
	}
	r.click(t, r.at(t, phaseChipLabel(phaseWaiting)))
	if !r.router.Source().Focused(terminal) {
		t.Error("the field kept the keyboard after a press on a phase chip")
	}
}
