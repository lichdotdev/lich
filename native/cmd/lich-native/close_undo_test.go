package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

func TestUndoPutsTheReopenedRowBackAtItsSlotAndOnScreen(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = []backend.Project{{ID: "p", Path: "/repo", Sessions: []backend.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}}}}
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
	m.projects = []backend.Project{
		{ID: "p", Sessions: []backend.Session{{ID: "a"}}},
		{ID: "q", Sessions: []backend.Session{{ID: "x"}}},
	}
	m.activeProject, m.active = "p", "a"
	ids, ok := m.restoreSession("q", backend.Session{ID: "y"}, 5)
	if !ok || !slices.Equal(ids, []string{"x", "y"}) {
		t.Fatalf("order %v, want y at the end", ids)
	}
	if m.active != "a" || m.projects[1].ActiveSessionID != "y" {
		t.Fatalf("screen %q, q's active %q: want the screen kept and q pointing at y", m.active, m.projects[1].ActiveSessionID)
	}
}

func TestUndoIntoAClosedProjectIsDropped(t *testing.T) {
	m := newModel(nil, func() {})
	if _, ok := m.restoreSession("gone", backend.Session{ID: "y"}, 0); ok {
		t.Fatal("restored into a project that is no longer open")
	}
}

func TestUndoOfARowAlreadyBackOnlyActivatesIt(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = []backend.Project{{ID: "p", Sessions: []backend.Session{{ID: "a"}, {ID: "b"}}}}
	m.activeProject, m.active = "p", "a"
	ids, _ := m.restoreSession("p", backend.Session{ID: "b"}, 0)
	if !slices.Equal(ids, []string{"a", "b"}) || m.active != "b" {
		t.Fatalf("order %v, active %q: want no duplicate, b active", ids, m.active)
	}
}

// The reopen sends no event, so the window reads the chat of the row it puts
// back itself: without it the card came back with an empty conversation.
func TestBringingBackAChatSessionReadsItsConversation(t *testing.T) {
	for _, reopen := range []struct {
		name string
		run  func(u *window, s backend.Session)
	}{
		{"undo", func(u *window, s backend.Session) {
			parked := make(chan error, 1)
			parked <- nil
			u.undoClose("p", s, 0, parked)
		}},
		{"resume", func(u *window, s backend.Session) {
			u.resumeParked(backend.ClosedSession{ID: s.ID, ProjectID: "p"})
		}},
	} {
		t.Run(reopen.name, func(t *testing.T) {
			f, client := newChatBackend(t)
			f.answers = map[string]any{api.StoreReopenSession.Name: backend.Session{ID: "b2", Kind: "claude", View: api.ViewChat}}
			u := &window{ctx: context.Background(), client: client, model: newModel(client, func() {})}
			u.model.projects = []backend.Project{{ID: "p", Path: "/repo", Sessions: []backend.Session{{ID: "a"}}}}
			u.model.activeProject = "p"
			reopen.run(u, backend.Session{ID: "b", Kind: "claude", View: api.ViewChat})
			if body := f.waitCall(t, api.ChatHistory.Name); !strings.Contains(body, `"id":"b2"`) {
				t.Errorf("read %s, want the restored b2", body)
			}
		})
	}
}
