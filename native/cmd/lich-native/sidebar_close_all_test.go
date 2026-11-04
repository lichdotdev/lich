package main

import (
	"slices"
	"testing"
)

func TestCloseAllAsksOnlyWhenATurnIsInFlight(t *testing.T) {
	u := &window{model: newModel(nil, func() {})}
	replay(t, u.model.status, []step{
		{0, statusEvent{ID: "b", State: "busy"}},
		{0, statusEvent{ID: "d", State: "done"}},
	})

	u.closeAll([]string{"d", "i"})
	if u.closeAllConfirm.open {
		t.Fatal("asked with no session mid-turn")
	}

	ids := []string{"b", "d", "i"}
	u.closeAll(ids)
	c := u.closeAllConfirm
	if !c.open || c.running != 1 || !slices.Equal(c.ids, ids) {
		t.Errorf("confirm = open %v, running %d, ids %v; want open, 1, %v", c.open, c.running, c.ids, ids)
	}
}
