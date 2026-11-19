package main

import (
	"image"
	"testing"

	"gioui.org/io/input"

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
	u.ctx, u.client = t.Context(), client
	comments := new(commentStore)
	code := newCodeTab(client, r.invalidate, comments, func(ui.Toast) {}, func(string, string, int) {})
	review, err := newReviewPanel(client, &u.root, comments, "monospace", 12, r.invalidate, func(ui.Toast) {})
	if err != nil {
		t.Fatal(err)
	}
	u.dock = newDock(code, review, newArtifactsTab(new(artifactStore)), nil)
	u.dock.tab = tab
	u.footer = newFooter(u.ctx, client, &u.root, newFooterUsage(), newPlanQuota(quotaReader(client), r.invalidate), r.invalidate)
	u.footer.dock = u.dock
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

// The dock never closes: its header's right end is collapse and full screen,
// with no × after them.
func TestDockHeaderEndsOnCollapseAndFullScreen(t *testing.T) {
	_, r := stageFrames(t, dockReview)
	r.waitFor(t, "No uncommitted changes")
	actions := image.Rect(rigWindow.X-120, 0, rigWindow.X, 48)
	// A control with a tooltip is two nodes on the same bounds.
	controls := map[image.Rectangle]bool{}
	for _, node := range r.nodes() {
		if node.Desc.Gestures&input.ClickGesture != 0 && node.Desc.Bounds.In(actions) {
			controls[node.Desc.Bounds] = true
		}
	}
	if len(controls) != 2 {
		t.Errorf("%d controls at the header's right end, want collapse and full screen", len(controls))
	}
}

func TestDockFullScreenFillsTheRowInTheFrameAfterTheClick(t *testing.T) {
	u, r := stageFrames(t, dockReview)
	r.waitFor(t, "No uncommitted changes")
	r.click(t, r.smallestClickIn(t, image.Rect(rigWindow.X-36, 0, rigWindow.X, 48)))
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
