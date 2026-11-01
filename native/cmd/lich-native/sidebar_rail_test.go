package main

import (
	"slices"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func railShape(items []railItem) []string {
	var out []string
	for _, it := range items {
		s := [...]string{railCard: "card", railCheckout: "checkout", railHead: "head", railGap: "gap"}[it.kind]
		if it.sv != nil {
			s += ":" + it.sv.ID
		}
		if it.small {
			s += " small"
		}
		if it.up {
			s += " ^"
		}
		if it.down {
			s += " v"
		}
		if it.onTray {
			s += " tray"
		}
		out = append(out, s)
	}
	return out
}

func TestRailDrawsTheTreeInOneColumn(t *testing.T) {
	main := blockView{key: "/repo", kind: blockCheckout, under: "/repo", sessions: []sessionView{
		treeSession("mother", "", "/repo"), treeSession("child", "mother", "/repo"), treeSession("solo", "", "/repo"),
	}}
	folder := blockView{key: folderKey("Later"), kind: blockFolder, byCheckout: true, color: "pink", sessions: []sessionView{
		treeSession("a", "", "/wt/a"), treeSession("b", "", "/wt/b"),
	}}
	got := railShape(railItems([]blockView{main, folder}))
	want := []string{
		"checkout v", "card:mother ^ v", "card:child small ^ v", "card:solo ^",
		"gap", "head tray", "checkout v tray", "card:a ^ tray", "checkout v tray", "card:b ^ tray",
	}
	if !slices.Equal(got, want) {
		t.Errorf("rail = %v\nwant   %v", got, want)
	}
}

func TestRailLeavesAFoldedBlockItsHead(t *testing.T) {
	b := blockView{key: "/repo", kind: blockCheckout, under: "/repo", folded: true, sessions: []sessionView{treeSession("s", "", "/repo")}}
	if got := railShape(railItems([]blockView{b})); !slices.Equal(got, []string{"checkout"}) {
		t.Errorf("rail = %v, want the checkout's node alone", got)
	}
}

func TestMorphNamesEveryNodeBothShapesDraw(t *testing.T) {
	main := blockView{key: "/repo", kind: blockCheckout, under: "/repo", sessions: []sessionView{treeSession("s", "", "/repo")}}
	folder := blockView{key: folderKey("Later"), kind: blockFolder, byCheckout: true, sessions: []sessionView{treeSession("a", "", "/wt/a")}}
	nodes := nodeGlyphs(&ui.Theme{Palette: ui.Dark}, []blockView{main, folder})
	for _, it := range railItems([]blockView{main, folder}) {
		if it.kind == railCheckout || it.kind == railHead {
			if _, ok := nodes[glyphNode+it.key]; !ok {
				t.Errorf("the rail's node %q has no flying glyph", it.key)
			}
		}
	}
}

func railMotionBlocks(ids ...string) []blockView {
	var sessions []sessionView
	for _, id := range ids {
		sessions = append(sessions, treeSession(id, "", "/repo"))
	}
	return []blockView{{key: "/repo", kind: blockCheckout, under: "/repo", sessions: sessions}}
}

func railMoves(t *testing.T, r *sidebarRail, now time.Time, blocks []blockView) map[string]*rowMotion {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Now: now}
	moving := map[string]*rowMotion{}
	for _, it := range r.move(gtx, railItems(blocks)) {
		moving[it.id] = it.motion
	}
	return moving
}

func TestRailOpensAndClosesRowsByTheLine(t *testing.T) {
	var r sidebarRail
	now := time.Unix(100, 0)
	if got := railMoves(t, &r, now, railMotionBlocks("a")); got["a"] != nil {
		t.Fatalf("first frame: a moves (%+v), want the rail as it is", got["a"])
	}
	opened := railMoves(t, &r, now, railMotionBlocks("a", "b"))
	if m := opened["b"]; m == nil || m.closing {
		t.Fatalf("a new session: motion %+v, want it opening", m)
	}
	closing := railMoves(t, &r, now.Add(5*time.Second), railMotionBlocks("a"))
	if m, ok := closing["b"]; !ok || m == nil || !m.closing {
		t.Fatalf("a closed session: rows %v, want b drawn while it closes", closing)
	}
	if gone := railMoves(t, &r, now.Add(10*time.Second), railMotionBlocks("a")); len(gone) != 2 {
		t.Errorf("after its close: rows %v, want b gone", gone)
	}
}

func TestRailForgetsWhatChangedWhileTheSidebarWasOpen(t *testing.T) {
	var r sidebarRail
	now := time.Unix(100, 0)
	railMoves(t, &r, now, railMotionBlocks("a"))
	r.forget()
	if got := railMoves(t, &r, now, railMotionBlocks("a", "b")); got["b"] != nil {
		t.Errorf("b opened while the sidebar was open moves in the rail (%+v), want it there at rest", got["b"])
	}
}

func TestRailMenusMatchTheOpenSidebarsLessRename(t *testing.T) {
	s := newSidebar(&ui.Root{})
	blocks := []blockView{{key: folderKey("Later"), kind: blockFolder, folder: "Later", title: "Later", byCheckout: true, sessions: []sessionView{
		treeSession("mother", "", "/wt/a"), treeSession("child", "mother", "/wt/a"),
	}}}
	s.rail.view = sidebarView{blocks: blocks, folders: []string{"Later"}}
	var act sidebarAction
	labels := func(id string) []string {
		var out []string
		for _, it := range railItems(blocks) {
			if it.id == id {
				for _, e := range s.rail.entries(&ui.Theme{Palette: ui.Dark}, s, it, &act) {
					out = append(out, e.Label)
				}
			}
		}
		return out
	}
	mother := labels("mother")
	if slices.Contains(mother, i18n.T("sidebar.sessionCard.rename")) || !slices.Contains(mother, i18n.T("sidebar.sessionGroupHeader.collapse", i18n.Args{"name": ""})) {
		t.Errorf("mother's menu = %v, want no rename and her fold", mother)
	}
	head := labels("head:" + folderKey("Later"))
	if !slices.Contains(head, i18n.T("sidebar.checkoutRow.newSessionHere")) || slices.Contains(head, i18n.T("sidebar.sessionGroupHeader.renameFolder")) {
		t.Errorf("folder's menu = %v, want a new session and no rename", head)
	}
}
