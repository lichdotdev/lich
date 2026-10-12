package main

import (
	"slices"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
)

func TestUndoPutsTheReopenedRowBackAtItsSlotAndOnScreen(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = []lichclient.Project{{ID: "p", Path: "/repo", Sessions: []lichclient.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}}}}
	m.activeProject, m.active = "p", "b"

	closed, index, _ := m.slotOf("b")
	if _, active, _ := m.closeSession("b"); active != "c" {
		t.Fatalf("close moved the screen to %q, want c", active)
	}
	reopened := closed
	reopened.ID, reopened.Path = "b2", "/repo"
	ids, ok := m.restoreSession("p", reopened, index)
	if !ok || !slices.Equal(ids, []string{"a", "b2", "c"}) {
		t.Fatalf("order %v (ok %v), want b2 back in b's slot", ids, ok)
	}
	if m.active != "b2" || m.projects[0].ActiveSessionID != "b2" {
		t.Fatalf("active %q / %q, want the restored card", m.active, m.projects[0].ActiveSessionID)
	}
	if m.projects[0].Sessions[1].Path != "" {
		t.Fatalf("path %q, want the project's own checkout as the model spells it", m.projects[0].Sessions[1].Path)
	}
}

func TestUndoPastTheEndAppendsAndABackgroundProjectKeepsTheScreen(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = []lichclient.Project{
		{ID: "p", Sessions: []lichclient.Session{{ID: "a"}}},
		{ID: "q", Sessions: []lichclient.Session{{ID: "x"}}},
	}
	m.activeProject, m.active = "p", "a"
	ids, ok := m.restoreSession("q", lichclient.Session{ID: "y"}, 5)
	if !ok || !slices.Equal(ids, []string{"x", "y"}) {
		t.Fatalf("order %v, want y at the end", ids)
	}
	if m.active != "a" || m.projects[1].ActiveSessionID != "y" {
		t.Fatalf("screen %q, q's active %q: want the screen kept and q pointing at y", m.active, m.projects[1].ActiveSessionID)
	}
}

func TestUndoIntoAClosedProjectIsDropped(t *testing.T) {
	m := newModel(nil, func() {})
	if _, ok := m.restoreSession("gone", lichclient.Session{ID: "y"}, 0); ok {
		t.Fatal("restored into a project that is no longer open")
	}
}

func TestUndoOfARowAlreadyBackOnlyActivatesIt(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = []lichclient.Project{{ID: "p", Sessions: []lichclient.Session{{ID: "a"}, {ID: "b"}}}}
	m.activeProject, m.active = "p", "a"
	ids, _ := m.restoreSession("p", lichclient.Session{ID: "b"}, 0)
	if !slices.Equal(ids, []string{"a", "b"}) || m.active != "b" {
		t.Fatalf("order %v, active %q: want no duplicate, b active", ids, m.active)
	}
}
