package main

import (
	"image"
	"testing"

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
