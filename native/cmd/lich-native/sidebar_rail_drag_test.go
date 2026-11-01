package main

import (
	"slices"
	"testing"
)

func TestRailGroupsSplitTheRowsByBlockAndHead(t *testing.T) {
	main := blockView{key: "/repo", kind: blockCheckout, under: "/repo", sessions: []sessionView{
		treeSession("first", "", "/repo"), treeSession("second", "", "/repo"),
	}}
	folder := blockView{key: folderKey("Later"), kind: blockFolder, byCheckout: true, sessions: []sessionView{treeSession("a", "", "/wt/a")}}
	groups := railGroups(railItems([]blockView{main, folder}))
	if len(groups) != 2 || groups[0].block != main.key || groups[1].block != folder.key {
		t.Fatalf("groups = %+v, want one a block", groups)
	}
	lead, rows := groups[0].split()
	if !slices.Equal(railShape(lead), []string{"checkout v"}) || !slices.Equal(railShape(rows), []string{"card:first ^ v", "card:second ^"}) {
		t.Errorf("checkout block = %v then %v, want its node leading its two cards", railShape(lead), railShape(rows))
	}
	lead, rows = groups[1].split()
	if !slices.Equal(railShape(lead), []string{"gap", "head tray"}) || len(rows) != 2 {
		t.Errorf("folder = %v then %v, want its gap and head leading its checkout and card", railShape(lead), railShape(rows))
	}
}

// Carried below the second card, the first one's slot is the tree's last
// node: the line runs down to it and stops.
func TestRailDragLinksTheRowsWhereTheyStand(t *testing.T) {
	main := blockView{key: "/repo", kind: blockCheckout, under: "/repo", sessions: []sessionView{
		treeSession("first", "", "/repo"), treeSession("second", "", "/repo"),
	}}
	lead, rows := railGroups(railItems([]blockView{main}))[0].split()
	byID := railDragLines(lead, rows, "first", "second")
	if got := railShape([]railItem{byID["second"], byID["first"]}); !slices.Equal(got, []string{"card:second ^ v", "card:first ^"}) {
		t.Errorf("rows in the air = %v, want second linked on to first, first ending the line", got)
	}
}
