package main

import (
	"image"
	"testing"

	"github.com/lichdotdev/lich/native/ui"
)

// stageFrames is the stage row of a project with no session, its dock open
// on tab.
func stageFrames(t *testing.T, tab dockTab) (*window, *frameRig) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	u := &window{model: newModel(nil, r.invalidate), terms: &terminals{byID: map[string]*termView{}, invalidate: r.invalidate}}
	u.topbar = newTopbar(&u.root)
	u.sidebar = newSidebar(&u.root)
	_, client := newSidebarBackend(t)
	comments := new(commentStore)
	code, err := newCodeTab(client, r.invalidate, "monospace", 12, comments, func(ui.Toast) {})
	if err != nil {
		t.Fatal(err)
	}
	review, err := newReviewPanel(client, &u.root, comments, "monospace", 12, r.invalidate, func(ui.Toast) {})
	if err != nil {
		t.Fatal(err)
	}
	u.dock = newDock(code, review)
	u.dock.tab = tab
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			if err := u.stageRow(gtx, stageView{}); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return u, r
}

func TestDockCloseHidesItInTheFrameAfterTheClick(t *testing.T) {
	u, r := stageFrames(t, dockReview)
	r.settle(t)
	r.click(t, r.smallestClickIn(t, image.Rect(rigWindow.X-36, 0, rigWindow.X, 48)))
	if u.dock.tab != dockClosed {
		t.Fatalf("the click missed the ×: dock on %v", u.dock.tab)
	}
}

func TestDockFullScreenFillsTheRowInTheFrameAfterTheClick(t *testing.T) {
	u, r := stageFrames(t, dockReview)
	r.settle(t)
	r.click(t, r.smallestClickIn(t, image.Rect(rigWindow.X-64, 0, rigWindow.X-36, 48)))
	if !u.dock.fullscreen {
		t.Fatal("the click missed full screen")
	}
}

func TestSettingsCloseLeavesInTheFrameAfterTheClick(t *testing.T) {
	s, r := settingsFrames(t, sectionGeneral)
	r.settle(t)
	r.click(t, r.smallestClickIn(t, image.Rect(rigWindow.X-48, 0, rigWindow.X, 48)))
	if s.open {
		t.Fatal("the click missed the ×")
	}
}

func TestSettingsNavLightsOnlyThePickedSectionInTheFrameThatTookTheClick(t *testing.T) {
	s, r := settingsFrames(t, sectionGeneral)
	r.clickLabel(t, sectionByID(sectionHotkeys).label())
	if s.section != sectionHotkeys {
		t.Fatalf("on %q, want Hotkeys", s.section)
	}
}
