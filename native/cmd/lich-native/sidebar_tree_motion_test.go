package main

import (
	"image"
	"slices"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func motionRows(sessions []sessionView, folded ...string) []sessionRow {
	return sessionRows(sessions, "", func(id string) bool { return slices.Contains(folded, id) })
}

func motionTree() []sessionView {
	return []sessionView{
		treeSession("mother", "", "/repo"),
		treeSession("a", "mother", "/repo"),
		treeSession("a1", "a", "/repo"),
		treeSession("b", "mother", "/repo"),
		treeSession("solo", "", "/repo"),
	}
}

func TestTreeMotionOpensTheRowsAMotherUnfolds(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions, "mother"))
	if len(m.runs) != 0 {
		t.Fatalf("runs = %d on the first frame, want none: nothing changed", len(m.runs))
	}
	if !m.update(now, motionRows(sessions)) || len(m.runs) != 1 {
		t.Fatalf("runs = %+v, want one opening", m.runs)
	}
	run := m.runs[0]
	if got := rowShape(run.rows); !slices.Equal(got, []string{"1:a", "2:a1.", "1:b."}) || run.closing {
		t.Errorf("run = %v closing %v, want the mother's three rows opening", got, run.closing)
	}
	first := run.frame(now, 0)
	if first.height != 0 || first.entry != (treeEntry{ring: 1, glyph: 1, body: 1}) || first.sweep.col != 0 {
		t.Errorf("first frame = %+v, want nothing drawn yet, swept down the mother's column", first)
	}
	last := run.frame(now.Add(run.length()), 2)
	if last.height != 1 || last.entry != (treeEntry{}) || last.sweep.rows < 1 {
		t.Errorf("last frame = %+v, want the row whole", last)
	}
}

func TestTreeMotionDrawsEachRingAfterTheLineReachesIt(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions, "mother"))
	m.update(now, motionRows(sessions))
	run := m.runs[0]
	mid := now.Add(run.length() / 3)
	upper, lower := run.frame(mid, 0), run.frame(mid, 2)
	if upper.entry.ring >= lower.entry.ring {
		t.Errorf("ring left = %v above and %v below, want the upper ring ahead", upper.entry.ring, lower.entry.ring)
	}
	if upper.sweep.rows <= lower.sweep.rows {
		t.Errorf("line = %v rows into the upper row and %v into the lower, want it further down the upper", upper.sweep.rows, lower.sweep.rows)
	}
}

func TestTreeMotionClosesAFoldedGroupUnderItsMother(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions))
	m.update(now, motionRows(sessions, "mother"))
	if len(m.runs) != 1 || !m.runs[0].closing || m.runs[0].after != "mother" {
		t.Fatalf("runs = %+v, want one closing under the mother", m.runs)
	}
	rows := m.place(now, motionRows(sessions, "mother"))
	want := []string{"0:mother", "1:a", "2:a1.", "1:b.", "0:solo."}
	if got := rowShape(rows); !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v: the group drawn under her while it leaves", got, want)
	}
	if f := rows[1].motion; f == nil || f.height != 1 || f.entry != (treeEntry{}) {
		t.Errorf("first leaving row = %+v, want it whole as the close starts", f)
	}
	end := m.runs[0].length()
	if f := m.runs[0].frame(now.Add(end), 0); f.height != 0 || f.entry != (treeEntry{ring: 1, glyph: 1, body: 1}) {
		t.Errorf("closed frame = %+v, want nothing left", f)
	}
	if m.update(now.Add(end), motionRows(sessions, "mother")) {
		t.Error("a run still moves once its close has played")
	}
}

func TestTreeMotionClosesAClosedSessionUnderTheRowAboveIt(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions))
	left := slices.DeleteFunc(slices.Clone(sessions), func(sv sessionView) bool { return sv.ID == "b" })
	if !m.update(now, motionRows(left)) || len(m.runs) != 1 || !m.runs[0].closing || m.runs[0].after != "a1" {
		t.Fatalf("runs = %+v, want b closing under a1", m.runs)
	}
	if got := rowShape(m.place(now, motionRows(left))); !slices.Equal(got, []string{"0:mother", "1:a", "2:a1.", "1:b.", "0:solo."}) {
		t.Errorf("rows = %v, want b drawn in its slot while it leaves", got)
	}
}

func TestTreeMotionOpensAChildBornUnderItsMother(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions))
	born := append(slices.Clone(sessions), treeSession("c", "mother", "/repo"))
	m.update(now, motionRows(born))
	if len(m.runs) != 1 || !slices.Equal(rowShape(m.runs[0].rows), []string{"1:c."}) {
		t.Errorf("runs = %+v, want the newborn alone opening", m.runs)
	}
}

func TestTreeMotionOpensANewRoot(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions))
	more := append([]sessionView{treeSession("root", "", "/repo")}, sessions...)
	if !m.update(now, motionRows(more)) || len(m.runs) != 1 || m.runs[0].after != "" {
		t.Fatalf("runs = %+v, want the new root opening at the top", m.runs)
	}
	if f := m.runs[0].frame(now, 0); f.sweep.col != -1 {
		t.Errorf("root sweep = %+v, want no column: a root hangs from no line", f.sweep)
	}
}

func TestTreeMotionOpensTheRowsOfABlockThatArrives(t *testing.T) {
	var m treeMotion
	m.arrive(time.Time{})
	if !m.update(time.Now(), motionRows(motionTree())) || len(m.runs[0].rows) != 5 {
		t.Errorf("runs = %+v, want every row of a block new to a drawn list opening", m.runs)
	}
}

func TestTreeMotionPutsANewRowsNameInBeforeItsRing(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions))
	born := append(slices.Clone(sessions), treeSession("c", "mother", "/repo"))
	m.update(now, motionRows(born))
	f := m.runs[0].frame(now.Add(treeOpen(1)), 0)
	if f.height != 1 || f.entry.body != 0 || f.entry.ring == 0 || f.entry.glyph == 0 {
		t.Errorf("frame once the row is open = %+v, want its name in and its ring and glyph still coming", f)
	}
}

func TestBlockPresenceHoldsALeavingCheckoutForItsTreeMotion(t *testing.T) {
	checkout := func(key string) blockView {
		return blockView{key: key, kind: blockCheckout, under: key, sessions: []sessionView{{Session: backend.Session{ID: key + "-s"}, path: key}}}
	}
	var p blockPresence
	now := time.Unix(100, 0)
	p.shown(now, "p", []blockView{checkout("/a"), checkout("/b")})
	shown := p.shown(now, "p", []blockView{checkout("/a")})
	if len(shown) != 2 || shown[1].key != "/b" || shown[1].move == nil || !shown[1].move.leaving {
		t.Fatalf("shown = %+v, want /b held in its slot, leaving by the tree's motion", shown)
	}
	end := shown[1].move.end()
	if got := p.shown(end, "p", []blockView{checkout("/a")}); len(got) != 1 {
		t.Errorf("shown at the motion's end = %d blocks, want /b gone", len(got))
	}
	arrived := p.shown(end, "p", []blockView{checkout("/a"), checkout("/c")})
	if arrived[1].move == nil || arrived[1].move.leaving {
		t.Errorf("a new checkout = %+v, want it coming in by the tree's motion", arrived[1])
	}
	folder := blockView{key: folderKey("Later"), kind: blockFolder, byCheckout: true, sessions: []sessionView{{Session: backend.Session{ID: "f"}, path: "/f"}}}
	withFolder := p.shown(end, "p", []blockView{checkout("/a"), checkout("/c"), folder})
	if withFolder[2].move == nil || withFolder[2].move.leaving {
		t.Errorf("a new folder = %+v, want it coming in by the tree's motion", withFolder[2])
	}
}

func TestTreeMotionKeepsTheLineToALeavingLastChild(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions))
	left := slices.DeleteFunc(slices.Clone(sessions), func(sv sessionView) bool { return sv.ID == "b" })
	m.update(now, motionRows(left))
	rows := m.place(now, motionRows(left))
	a, a1 := rows[1], rows[2]
	if a.key != "a" || a.tail {
		t.Errorf("a = %+v, want it no tail while b leaves under it: its line runs on to b", a)
	}
	if !slices.Equal(a1.lanes, []bool{true}) {
		t.Errorf("a1 lanes = %v, want the mother's line running past it to b", a1.lanes)
	}
}

func TestRelinkMatchesTheTreesOwnLinks(t *testing.T) {
	check := func(name string, rows []sessionRow) {
		want := slices.Clone(rows)
		got := relink(slices.Clone(rows))
		for i := range want {
			if got[i].tail != want[i].tail || !slices.Equal(got[i].lanes, want[i].lanes) {
				t.Errorf("%s row %s: relinked tail %v lanes %v, want %v %v", name, want[i].key, got[i].tail, got[i].lanes, want[i].tail, want[i].lanes)
			}
		}
	}
	check("tree", motionRows(motionTree()))
	check("checkout", sessionRows([]sessionView{
		treeSession("mother", "", "/repo/wt/a"), treeSession("tests", "mother", "/repo/wt/a"), treeSession("flake", "mother", "/repo/wt/b"), treeSession("solo", "", "/repo/wt/a"),
	}, "/repo/wt/a", func(string) bool { return false }))
	check("folder", blockRows(blockView{key: "f", byCheckout: true}, []sessionView{
		treeSession("a", "", "/wt/a"), treeSession("a-child", "a", "/wt/c"), treeSession("a2", "", "/wt/a"), treeSession("b", "", "/wt/b"),
	}))
}

func TestTreeMotionClosesAFoldedBlocksRowsFromItsTop(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	open := blockView{sessions: sessions}
	folded := blockView{sessions: sessions, collapsed: true}
	var m treeMotion
	m.update(now, foldedRows(open, motionRows(sessions)))
	m.update(now, foldedRows(folded, motionRows(sessions)))
	if len(m.runs) != 1 || !m.runs[0].closing || m.runs[0].after != "" || len(m.runs[0].rows) != 5 {
		t.Fatalf("runs = %+v, want every row closing from the block's top", m.runs)
	}
	if !m.hasRows() {
		t.Error("hasRows = false while the rows leave, want the folder's tray held")
	}
	rows := m.place(now, foldedRows(folded, motionRows(sessions)))
	if len(rows) != 5 || rows[0].motion == nil || !rows[0].motion.closing {
		t.Fatalf("rows = %v, want the five drawn while they leave", rowShape(rows))
	}
	end := now.Add(m.runs[0].length())
	if m.update(end, nil) || m.hasRows() || len(m.place(end, nil)) != 0 {
		t.Error("a folded block still draws rows once its close has played")
	}
	if !m.update(end, foldedRows(open, motionRows(sessions))) || m.runs[0].closing {
		t.Errorf("runs = %+v, want the rows opening as the block unfolds", m.runs)
	}
}

// The selected card's fill came in whole left of its ring while the rest of
// it faded in: a lit stub under the row above.
func TestAnEnteringCardsFillComesInWholeAcrossItsRing(t *testing.T) {
	const width, height = 200, 32
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	s := newSidebar(&ui.Root{})
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	motion := rowMotion{height: 1, entry: treeEntry{ring: 1, glyph: 1, body: 1}}
	r := sessionRow{motion: &motion}
	sv := sessionView{Session: backend.Session{ID: "s1"}, place: r.place(true)}
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(width, height))}
	paint.Fill(ops, th.Sidebar)
	moveRow(gtx, r, func(gtx C) D {
		d, _ := s.card(gtx, th, &sv, true, nil)
		return d
	})
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	if left, right := img.RGBAAt(3, height/2), img.RGBAAt(width/2, height/2); left != right {
		t.Errorf("fill left of the ring %v, past it %v: want one fill while the body is still out", left, right)
	}
}
