package main

import (
	"github.com/lichdotdev/lich/native/backend"
	"slices"
	"testing"
)

func TestCardSelection(t *testing.T) {
	var sel cardSelection
	sel.toggle("a")
	sel.toggle("b")
	sel.toggle("c")
	sel.toggle("b")
	if !sel.has("a") || sel.has("b") || !sel.has("c") {
		t.Fatalf("after toggling a, b, c, b: %v", sel.ids)
	}
	var act sidebarAction
	if sel.menu("b", &act) != nil {
		t.Error("a card outside the selection opens the selection's menu")
	}
	menu := sel.menu("a", &act)
	if want := []string{"Close all sessions"}; !slices.Equal(labels(menu), want) {
		t.Fatalf("selection menu %v, want %v", labels(menu), want)
	}
	entry(t, menu, "Close all sessions").OnClick()
	if !slices.Equal(act.closeAll, []string{"a", "c"}) {
		t.Errorf("Close all sessions closed %v", act.closeAll)
	}
	if sel.has("a") || sel.has("c") {
		t.Error("the selection outlives the close it asked for")
	}
	sel.toggle("a")
	if sel.menu("a", &act) != nil {
		t.Error("a selection of one opens the selection's menu")
	}
	sel.clear()
	if sel.has("a") {
		t.Error("clear left a card selected")
	}
}

func TestCardSelectionDropsClosedSessions(t *testing.T) {
	var sel cardSelection
	sel.toggle("a")
	sel.toggle("b")
	sel.keep([]backend.Session{{ID: "a"}, {ID: "c"}})
	if sel.has("b") || !sel.has("a") {
		t.Fatalf("after b closed elsewhere: %v", sel.ids)
	}
	var act sidebarAction
	if sel.menu("a", &act) != nil {
		t.Error("the one card left still opens the selection's menu")
	}
}
