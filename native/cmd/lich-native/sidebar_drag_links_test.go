package main

import (
	"slices"
	"testing"
	"time"
)

// Dragging the first of two sessions under a checkout below the second drew
// the tree in its old order: the row now on top ended the checkout's line,
// and the one carried below it hung off the end of nothing.
func TestADragLinksTheRowsWhereTheyStand(t *testing.T) {
	rows := sessionRows([]sessionView{
		treeSession("shell", "", "/repo/wt/a"), treeSession("side", "", "/repo/wt/a"),
	}, "/repo/wt/a", func(string) bool { return false })
	got := dragRows(rows, "shell", "side")
	if shape := rowShape(got); !slices.Equal(shape, []string{"1:side", "1:shell."}) {
		t.Errorf("rows = %v, want side then shell, shell ending the line", shape)
	}
	if !slices.Equal(rowShape(rows), []string{"1:shell", "1:side."}) {
		t.Errorf("rows = %v, want the block's own rows left as they were", rowShape(rows))
	}
}

func TestADragOverNothingInItsListMovesNoRow(t *testing.T) {
	rows := motionRows(motionTree())
	if got := dragRows(rows, "solo", "a folder header"); !slices.Equal(rowShape(got), rowShape(rows)) {
		t.Errorf("rows = %v, want them as they stand", rowShape(got))
	}
}

func TestACarriedCardLetsGoOfItsLineAndJoinsWhereItLands(t *testing.T) {
	var d dragLine
	now := time.Unix(100, 0)
	d.update(now, "shell")
	if got := d.cut("shell", now); got != 1 {
		t.Errorf("cut while in the air = %v, want 1: the card draws no line", got)
	}
	if got := d.cut("side", now); got != 0 {
		t.Errorf("another row's cut = %v, want 0", got)
	}
	letGo, slot := d.slot(now)
	if letGo != 0 || slot != 0 {
		t.Errorf("slot at the lift = %v %v, want the line whole and no dashes yet", letGo, slot)
	}
	letGo, slot = d.slot(now.Add(dragLetGo + dragSlotIn))
	if letGo != 1 || slot != 1 {
		t.Errorf("slot once let go = %v %v, want the line gone and the dashes in", letGo, slot)
	}
	landed := now.Add(time.Second)
	d.update(landed, "")
	if got := d.cut("shell", landed.Add(dragSettle)); got != 1 {
		t.Errorf("cut while the card settles = %v, want 1", got)
	}
	if got := d.cut("shell", landed.Add(dragSettle+dragJoin/2)); got <= 0 || got >= 1 {
		t.Errorf("cut midway through the join = %v, want the line part drawn", got)
	}
	end := landed.Add(dragSettle + dragJoin)
	if got := d.cut("shell", end); got != 0 || d.moving(end) {
		t.Errorf("cut at the join's end = %v moving %v, want the line whole and still", got, d.moving(end))
	}
}

func snapBlock() blockView {
	return blockView{key: "/repo", kind: blockCheckout, under: "/repo", sessions: []sessionView{
		treeSession("mother", "", "/repo"), treeSession("child", "mother", "/repo"), treeSession("solo", "", "/repo"),
	}}
}

// Dragged between a mother and her child, a session showed the child under
// it while in the air, then landed after her: the tree keeps a child under
// its mother whatever the stored order says. The slot snaps to one the tree
// keeps.
func TestADragOnlyLandsWhereTheTreeKeepsIt(t *testing.T) {
	b := snapBlock()
	keys := treeKeys(blockRows(b, b.sessions))
	snap := treeSnap(b, keys)
	if got := snap(2, 1); got != 2 {
		t.Errorf("solo over the child snaps to %d, want 2: between mother and child is no slot", got)
	}
	if got := snap(2, 0); got != 0 {
		t.Errorf("solo over the mother snaps to %d, want 0: above her is a slot", got)
	}
	if got := snap(1, 2); got != 1 {
		t.Errorf("the child over solo snaps to %d, want 1: she stays under her mother", got)
	}
}

// A mother in the air folds over her children, so they go with her: the list
// moves her alone, and she lands past solo.
func TestAMotherInTheAirTakesHerChildren(t *testing.T) {
	b := carriedFold(snapBlock(), "mother")
	rows := blockRows(b, b.sessions)
	if got := rowShape(rows); !slices.Equal(got, []string{"1:mother", "1:solo."}) {
		t.Fatalf("rows = %v, want the mother folded over her child", got)
	}
	if got := treeSnap(b, treeKeys(rows))(0, 1); got != 1 {
		t.Errorf("the mother over solo snaps to %d, want 1", got)
	}
	own := snapBlock()
	own.motherFolds = map[string]bool{}
	carriedFold(own, "mother")
	if len(own.motherFolds) != 0 {
		t.Errorf("the block's own folds = %v, want them untouched", own.motherFolds)
	}
}

func treeKeys(rows []sessionRow) []string {
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = r.key
	}
	return keys
}
