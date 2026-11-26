package main

import (
	"image"
	"slices"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/ui"
)

func railShape(items []railItem) []string {
	var out []string
	for _, it := range items {
		s := [...]string{railCard: "card", railHead: "head", railGap: "gap", railEditor: "editor"}[it.kind]
		if it.sv != nil {
			s += ":" + it.sv.ID
		}
		if it.small {
			s += " small"
		}
		if it.onTray {
			s += " tray"
		}
		out = append(out, s)
	}
	return out
}

func TestRailDrawsTheListInOneColumn(t *testing.T) {
	main := blockView{key: rootBlockKey, kind: blockLoose, sessions: []sessionView{
		treeSession("mother", "", "/repo"), treeSession("child", "mother", "/repo"), treeSession("solo", "", "/repo"),
	}}
	folder := blockView{key: folderKey("Later"), kind: blockFolder, color: "pink", sessions: []sessionView{
		treeSession("a", "", "/wt/a"), treeSession("b", "", "/wt/b"),
	}}
	got := railShape(railItems([]blockView{main, folder}))
	want := []string{"card:mother", "card:child small", "card:solo", "gap", "head tray", "card:a tray", "card:b tray"}
	if !slices.Equal(got, want) {
		t.Errorf("rail = %v\nwant   %v", got, want)
	}
}

func TestRailLeavesAFoldedBlockItsHead(t *testing.T) {
	b := blockView{key: folderKey("Later"), kind: blockFolder, collapsed: true, sessions: []sessionView{treeSession("s", "", "/repo")}}
	if got := railShape(railItems([]blockView{b})); !slices.Equal(got, []string{"head tray"}) {
		t.Errorf("rail = %v, want the folder's head alone", got)
	}
}

func TestMorphNamesEveryNodeBothShapesDraw(t *testing.T) {
	main := blockView{key: "/repo", kind: blockLoose, sessions: []sessionView{treeSession("s", "", "/repo")}}
	folder := blockView{key: folderKey("Later"), kind: blockFolder, sessions: []sessionView{treeSession("a", "", "/wt/a")}}
	nodes := nodeGlyphs(&ui.Theme{Palette: ui.Dark}, []blockView{main, folder})
	for _, it := range railItems([]blockView{main, folder}) {
		if it.kind == railHead {
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
	return []blockView{{key: "/repo", kind: blockLoose, sessions: sessions}}
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
	if gone := railMoves(t, &r, now.Add(10*time.Second), railMotionBlocks("a")); len(gone) != 1 {
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

func TestTheRailBadgeKeepsTheCardsCornerAndDropsItAtRest(t *testing.T) {
	s := newSidebar(&ui.Root{})
	sv := treeSession("a", "", "/repo")
	sv.status = statusWaiting
	draw := func() string {
		r := newFrameRig(image.Pt(32, 32), func(gtx C, th *ui.Theme) { s.railBadge(gtx, th, &sv, th.Surface) })
		drawn, _ := r.frame()
		return drawn
	}
	waiting := draw()
	if s.corners["a"] == nil {
		t.Fatal("a waiting session's badge keeps no corner, want the one the card draws")
	}
	sv.status = statusDone
	sv.unread = false
	if draw() == waiting || s.corners["a"] != nil {
		t.Errorf("a read session's badge still draws or keeps its corner, want none")
	}
}

func TestTheRailServicesBadgeCountsOnlyAProjectWithServices(t *testing.T) {
	draw := func(tally servicesTally) string {
		r := newFrameRig(image.Pt(32, 32), func(gtx C, th *ui.Theme) {
			var click ui.Clickable
			click.Layout(gtx, func(gtx C) D {
				return ui.SquareFace(gtx, th.Surface, func(gtx C) D {
					return layout.Stack{}.Layout(gtx,
						layout.Stacked(func(gtx C) D { return icServices.Layout(gtx, ui.Space(4), th.MutedForeground) }),
						layout.Expanded(func(gtx C) D { return railServicesBadge(gtx, th, tally) }),
					)
				})
			})
		})
		drawn, _ := r.frame()
		return drawn
	}
	none, idle, up := draw(servicesTally{}), draw(servicesTally{total: 3}), draw(servicesTally{up: 2, total: 3})
	empty := draw(servicesTally{})
	if none != empty {
		t.Fatal("drawing nothing differs frame to frame")
	}
	if idle == none || up == idle {
		t.Errorf("badges for 0/3 and 2/3 look the same as no services or as each other, want a count each")
	}
}
