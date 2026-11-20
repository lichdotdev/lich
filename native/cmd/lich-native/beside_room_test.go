package main

import (
	"context"
	"image"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// besideFrames draws the layout the model has on screen over a stage of
// size, through the real panes, so a pane added beside one is measured
// against where they were drawn.
func besideFrames(t *testing.T, size image.Point, lay func(m *model)) (*window, *frameRig, *model) {
	t.Helper()
	u, r, _ := chatFramesIn(t, api.ViewTerminal, size, busySession())
	m, _ := newLayoutsModel(t)
	m.invalidate = r.invalidate
	lay(m)
	u.model = m
	u.sidebar, u.dock, u.git, u.extPanel, u.topbar = &sidebar{}, &dock{}, &gitPanel{}, &extensionPanel{}, &topbar{}
	u.palette, u.worktree = &commandPalette{}, &worktreeDialog{}
	u.terms = &terminals{byID: map[string]*termView{}, invalidate: r.invalidate, client: u.client}
	u.gate = &spawnGate{probe: &fakeProbe{live: true}, invalidate: r.invalidate, deciding: "another"}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			u.readDragPointer(gtx)
			lt, ok := m.shownLayoutTab()
			if !ok {
				t.Fatal("no layout on screen")
			}
			v := stageView{hasLayout: true, layout: lt, panes: m.viewsOf(lt.sessions(), gtx.Now)}
			if err := u.layoutPanes(gtx, v); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	r.settle(t)
	// The split is measured against the frame before the one it acts in.
	r.frame()
	return u, r, m
}

// refused reports whether the toasts, drawn after an action, include the one
// that refuses a pane. The stage is not drawn again: a page pane would open a
// native page.
func refused(u *window, r *frameRig) bool {
	r.draw = func(gtx C, th *ui.Theme) { u.layoutToasts(gtx) }
	r.frame()
	return r.shows(i18n.T("layouts.split.noRoom"))
}

// Two panes side by side, each too narrow to halve: the browser opens under
// its session's terminal instead of being refused.
func TestABrowserWithNoRoomAtTheRightOpensUnderItsSession(t *testing.T) {
	u, r, m := besideFrames(t, image.Pt(1200, 800), func(m *model) {
		m.splitBeside(context.Background(), "orch", "wa", edgeRight, false)
	})
	u.openBrowser("orch")
	if refused(u, r) {
		t.Fatal("opening the browser was refused")
	}
	if shape, _ := pageScreen(t, m); shape != "row(column(orch,browser:orch),wa)" {
		t.Fatalf("on screen %s, want the page under orch's terminal", shape)
	}
}

// Three panes in a row on a short stage: orch's pane holds the page neither
// beside nor under it as drawn, but the other two give way, so the page
// goes at its right and the dividers make the room.
func TestABrowserOpensWhereTheLayoutCanGiveWay(t *testing.T) {
	u, r, m := besideFrames(t, image.Pt(2100, 340), func(m *model) {
		ctx := context.Background()
		m.splitBeside(ctx, "orch", "wa", edgeRight, false)
		m.splitBeside(ctx, "wa", "wb", edgeRight, false)
		m.setRatio(m.shownLayout, nil, 0.25)
	})
	d := &u.layoutStage.pointer
	if d.fits("orch", edgeRight) || d.fits("orch", edgeBottom) {
		t.Fatalf("orch drawn at %v holds a page as is; the case needs a layout that has to give way", d.last["orch"])
	}
	u.openBrowser("orch")
	if refused(u, r) {
		t.Fatal("opening the browser was refused")
	}
	if shape, _ := pageScreen(t, m); shape != "row(row(orch,browser:orch),row(wa,wb))" {
		t.Fatalf("on screen %s, want the page at orch's right", shape)
	}
}

// Past what any divider can give, the browser is still refused.
func TestABrowserWithNoRoomAnywhereIsRefused(t *testing.T) {
	u, r, m := besideFrames(t, image.Pt(1200, 340), func(m *model) {
		m.splitBeside(context.Background(), "orch", "wa", edgeRight, false)
	})
	u.openBrowser("orch")
	if !refused(u, r) {
		t.Fatal("no toast refused the pane")
	}
	if m.pageShown("orch") {
		t.Fatal("the page went on screen with no room for it")
	}
}

// Delegates that fit in no pane as drawn still go in where the layout's
// dividers can give way: as many as the stage holds, beside the slot before
// under it.
func TestDelegatesFitWhereTheLayoutCanGiveWay(t *testing.T) {
	d := &dragPointer{paneW: 480, paneH: 180, gap: 6, stage: image.Pt(2100, 340)}
	tab := layoutTab{Root: &paneNode{Axis: splitRow, Ratio: 0.25, First: &paneNode{Session: "orch"},
		Second: &paneNode{Axis: splitRow, Ratio: 0.5, First: &paneNode{Session: "wa"}, Second: &paneNode{Session: "wb"}}}}
	ids, under := d.delegatesFitting(tab, "orch", false, []string{"d1", "d2"})
	if len(ids) != 1 || ids[0] != "d1" || under {
		t.Errorf("fitting %v under=%v, want d1 alone at orch's right: two stacked are taller than the stage", ids, under)
	}
	d.stage = image.Pt(1500, 340)
	if ids, _ := d.delegatesFitting(tab, "orch", false, []string{"d1"}); len(ids) != 0 {
		t.Errorf("fitting %v on a stage too narrow for four panes and too short for two rows", ids)
	}
}
