package main

import (
	"slices"
	"testing"
)

func TestRailGroupsSplitTheRowsByBlockAndHead(t *testing.T) {
	main := blockView{key: "/repo", kind: blockLoose, sessions: []sessionView{
		treeSession("first", "", "/repo"), treeSession("second", "", "/repo"),
	}}
	folder := blockView{key: folderKey("Later"), kind: blockFolder, sessions: []sessionView{treeSession("a", "", "/wt/a")}}
	groups := railGroups(railItems([]blockView{main, folder}))
	if len(groups) != 2 || groups[0].block != main.key || groups[1].block != folder.key {
		t.Fatalf("groups = %+v, want one a block", groups)
	}
	lead, rows := groups[0].split()
	if len(lead) != 0 || !slices.Equal(railShape(rows), []string{"card:first", "card:second"}) {
		t.Errorf("loose block = %v then %v, want its two cards with nothing leading them", railShape(lead), railShape(rows))
	}
	lead, rows = groups[1].split()
	if !slices.Equal(railShape(lead), []string{"gap", "head tray"}) || len(rows) != 1 {
		t.Errorf("folder = %v then %v, want its gap and head leading its card", railShape(lead), railShape(rows))
	}
}
