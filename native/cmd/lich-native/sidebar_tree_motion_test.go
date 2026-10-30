package main

import (
	"slices"
	"testing"
	"time"
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

func openIDs(sessions []sessionView) map[string]bool {
	open := map[string]bool{}
	for _, sv := range sessions {
		open[sv.ID] = true
	}
	return open
}

func TestTreeMotionOpensTheRowsAMotherUnfolds(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions, "mother"), openIDs(sessions))
	if len(m.runs) != 0 {
		t.Fatalf("runs = %d on the first frame, want none: nothing changed", len(m.runs))
	}
	if !m.update(now, motionRows(sessions), openIDs(sessions)) || len(m.runs) != 1 {
		t.Fatalf("runs = %+v, want one opening", m.runs)
	}
	run := m.runs[0]
	if got := rowShape(run.rows); !slices.Equal(got, []string{"1:a", "2:a1.", "1:b."}) || run.closing {
		t.Errorf("run = %v closing %v, want the mother's three rows opening", got, run.closing)
	}
	first := run.frame(now, 0)
	if first.height != 0 || first.entry != (treeEntry{ring: 1, body: 1}) || first.sweep.col != 0 {
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
	m.update(now, motionRows(sessions, "mother"), openIDs(sessions))
	m.update(now, motionRows(sessions), openIDs(sessions))
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
	m.update(now, motionRows(sessions), openIDs(sessions))
	m.update(now, motionRows(sessions, "mother"), openIDs(sessions))
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
	if f := m.runs[0].frame(now.Add(end), 0); f.height != 0 || f.entry != (treeEntry{ring: 1, body: 1}) {
		t.Errorf("closed frame = %+v, want nothing left", f)
	}
	if m.update(now.Add(end), motionRows(sessions, "mother"), openIDs(sessions)) {
		t.Error("a run still moves once its close has played")
	}
}

func TestTreeMotionLeavesAClosedSessionToItsOwnExit(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions), openIDs(sessions))
	left := slices.DeleteFunc(slices.Clone(sessions), func(sv sessionView) bool { return sv.ID == "b" })
	if m.update(now, motionRows(left), openIDs(left)) {
		t.Errorf("runs = %+v, want none: a closed session plays closingCards' exit", m.runs)
	}
}

func TestTreeMotionOpensAChildBornUnderItsMother(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions), openIDs(sessions))
	born := append(slices.Clone(sessions), treeSession("c", "mother", "/repo"))
	m.update(now, motionRows(born), openIDs(born))
	if len(m.runs) != 1 || !slices.Equal(rowShape(m.runs[0].rows), []string{"1:c."}) {
		t.Errorf("runs = %+v, want the newborn alone opening", m.runs)
	}
}

func TestTreeMotionLeavesANewRootToPresence(t *testing.T) {
	sessions, now := motionTree(), time.Now()
	var m treeMotion
	m.update(now, motionRows(sessions), openIDs(sessions))
	more := append(slices.Clone(sessions), treeSession("root", "", "/repo"))
	if m.update(now, motionRows(more), openIDs(more)) {
		t.Errorf("runs = %+v, want none: a root hangs from no line", m.runs)
	}
}
