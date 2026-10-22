package main

import "testing"

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
