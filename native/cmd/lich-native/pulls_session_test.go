package main

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func TestSessionToOpenAtPrefersTheActiveSession(t *testing.T) {
	m, _ := newLayoutsModel(t)
	m.active = "notes"
	if id, ok := m.sessionToOpenAt("lich", "/dev/lich"); !ok || id != "notes" {
		t.Fatalf("opened %q (%v), want the active session notes", id, ok)
	}
}

func TestSessionToOpenAtFallsBackToTheFirstSession(t *testing.T) {
	m, _ := newLayoutsModel(t)
	m.activeProject, m.active = "plugin", "wb"
	if id, ok := m.sessionToOpenAt("lich", "/dev/lich"); !ok || id != "orch" {
		t.Fatalf("opened %q (%v), want the first session orch", id, ok)
	}
	if id, ok := m.sessionToOpenAt("lich", "/dev/elsewhere"); ok {
		t.Fatalf("opened %q in a checkout without sessions", id)
	}
}

// prWindow is a window on the Pulls screen of project p, whose checkout holds
// the PR's branch and session s1.
func prWindow(t *testing.T) (*window, *gateBackend) {
	t.Helper()
	u, b := newGateWindow(t)
	u.model.projects[0].Sessions = []backend.Session{{ID: "s1", Label: "Claude 1", Kind: "claude"}}
	b.answer["project.ListCheckouts"] = []backend.Worktree{{Name: "feat", Path: "/repo"}}
	b.answer["terminal.Live"], b.answer["terminal.Ready"] = true, true
	u.gate = &spawnGate{probe: u.client, invalidate: func() {}, notify: func(t ui.Toast) { u.toaster.Show(t) }}
	u.pulls = newPullsScreen(newPullsStore(u.ctx, u.client, func() {}), &u.root)
	u.pulls.open = true
	return u, b
}

func TestHandoffLeavesThePullsScreenUp(t *testing.T) {
	u, b := prWindow(t)
	u.openPRSession(u.model.projects[0], backend.PRDetail{Number: 7, HeadRefName: "feat"}, "fix it")
	b.await(t, "terminal.Start")
	if args := b.await(t, "relay.Submit"); !strings.Contains(args, `"fix it"`) {
		t.Fatalf("submitted %s, want the handoff prompt", args)
	}
	if !u.pulls.open {
		t.Fatal("the handoff took the Pulls screen down")
	}
	if active, ok := u.model.activeSession(); ok && active.ID == "s1" {
		t.Fatal("the handoff brought its session up")
	}
}

func TestOpenSessionBringsTheSessionUp(t *testing.T) {
	u, _ := prWindow(t)
	u.openPRSession(u.model.projects[0], backend.PRDetail{Number: 7, HeadRefName: "feat"}, "")
	if u.pulls.open {
		t.Fatal("Open session left the Pulls screen up")
	}
	if active, ok := u.model.activeSession(); !ok || active.ID != "s1" {
		t.Fatalf("active session %q, want s1", active.ID)
	}
}
