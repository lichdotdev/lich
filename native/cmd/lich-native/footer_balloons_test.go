package main

import (
	"image"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// pxGtx is a frame at one pixel per dp.
func pxGtx() C {
	return layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(rigWindow)}
}

func TestTheTerminalGivesTheBalloonsABandOnlyWhereTheyFloat(t *testing.T) {
	u := &window{settings: new(settingsScreen)}
	agent := sessionView{ID: "a", Kind: "claude"}
	shell := sessionView{ID: "s", Kind: "shell"}
	cases := []struct {
		name string
		v    stageView
		want int
	}{
		{"an agent's terminal", stageView{active: agent, hasActive: true}, 52},
		{"a plain terminal", stageView{active: shell, hasActive: true}, 0},
		{"a layout with an agent", stageView{hasLayout: true, panes: map[string]sessionView{"s": shell, "a": agent}}, 42},
		{"a layout of plain terminals", stageView{hasLayout: true, panes: map[string]sessionView{"s": shell}}, 0},
		{"a service's console", stageView{active: agent, hasActive: true, hasConsole: true}, 0},
		{"Home", stageView{home: true}, 0},
		{"a project with no session", stageView{}, 0},
	}
	for _, c := range cases {
		if got := u.ptyBand(pxGtx(), c.v); got != c.want {
			t.Errorf("%s: band %d, want %d", c.name, got, c.want)
		}
	}
	if !u.balloonsShown(stageView{}) {
		t.Error("a project with no session lost its balloons, the branch's way to the Git panel")
	}
	if u.balloonsShown(stageView{active: shell, hasActive: true}) {
		t.Error("a plain terminal carries balloons")
	}
}

func TestTheGitPanelTakesTheBalloonsBand(t *testing.T) {
	_, client := newSidebarBackend(t)
	u := &window{settings: new(settingsScreen)}
	u.git = newGitPanel(t.Context(), client, &u.root, func() {}, func(ui.Toast) {}, "monospace", 12)
	u.git.open = true
	agent := sessionView{ID: "a", Kind: "claude"}
	v := stageView{active: agent, hasActive: true, in: busySession()}
	if got := u.ptyBand(pxGtx(), v); got != 0 {
		t.Errorf("band %d under an open Git panel, want 0", got)
	}
	if u.balloonsShown(v) {
		t.Error("the balloons float over the Git panel")
	}
}

// balloonFrames draws the balloons over a stage of rigWindow's size.
func balloonFrames(t *testing.T, in footerInput) (*footer, *frameRig) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	f := newFooterRig(t, nil).f
	f.dock = new(dock)
	r.draw = func(gtx C, th *ui.Theme) {
		f.frame(gtx, in)
		f.layoutBalloons(gtx, th, gtx.Dp(balloonInset))
	}
	return f, r
}

func TestBalloonsStandInTheStagesBottomCornersOnOneLine(t *testing.T) {
	_, r := balloonFrames(t, busySession())
	r.settle(t)
	left, right := r.at(t, "PR #7"), r.at(t, "main")
	top := float32(rigWindow.Y - 52)
	for name, p := range map[string]float32{"left": left.Y, "right": right.Y} {
		if p < top || p > float32(rigWindow.Y-8) {
			t.Errorf("the %s balloon's item is at y=%v, want it in the band from %v", name, p, top)
		}
	}
	if left.X > float32(rigWindow.X/2) || right.X < float32(rigWindow.X/2) {
		t.Errorf("left item at x=%v, right at x=%v: want them at either corner", left.X, right.X)
	}
}

func TestANarrowStageLeavesOutWhatDoesNotFitRatherThanWrap(t *testing.T) {
	in := busySession()
	r := newFrameRig(image.Pt(360, 200), nil)
	f := newFooterRig(t, nil).f
	f.dock = new(dock)
	r.draw = func(gtx C, th *ui.Theme) {
		f.frame(gtx, in)
		f.layoutBalloons(gtx, th, gtx.Dp(balloonInset))
	}
	drawn := r.settle(t)
	if !strings.Contains(drawn, "main") {
		t.Errorf("the right balloon lost its first item:\n%s", drawn)
	}
	for _, n := range r.nodes() {
		if b := n.Desc.Bounds; n.Desc.Label != "" && b.Min.Y < 200-52 {
			t.Errorf("%q drawn at %v, over the band", n.Desc.Label, b)
		}
	}
}

func TestChangesBalloonOpensTheReviewTabInTheFrameThatTookTheClick(t *testing.T) {
	u, r := stageFrames(t, dockReview)
	in := busySession()
	in.sessionID = ""
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			u.footer.frame(gtx, in)
			if err := u.stageRow(gtx, stageView{in: in}); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	// The checkout under test is read before the tab goes: the click then
	// finds Review's diff on hand, not a read that lands after the frame.
	r.waitFor(t, "No uncommitted changes")
	u.dock.tab = dockCode
	r.settle(t)
	if !r.shows(i18n.T("dock.filesPanel.empty")) {
		t.Fatalf("the Code tab is not on screen:\n%s", r.drawn())
	}
	took, _ := r.clickLabel(t, "2")
	if u.dock.tab != dockReview {
		t.Fatalf("the click missed Review changes: dock on %v", u.dock.tab)
	}
	if strings.Contains(took, i18n.T("dock.filesPanel.empty")) {
		t.Errorf("the frame that took the click still showed the Code tab:\n%s", took)
	}
}
