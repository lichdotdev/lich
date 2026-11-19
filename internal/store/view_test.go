package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

// TestSessionViewRidesTheRowAndItsResume: the view a session is drawn in is
// read back by the hydration call, and a park then resume keeps it, so a chat
// comes back as a chat, on the mode its chat was set to.
func TestSessionViewRidesTheRowAndItsResume(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "base", Label: "Session 1", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "chat", Kind: "claude", NextSeq: 3})

	if got := svc.SessionView("s1"); got != "" {
		t.Fatalf("SessionView of a new session = %q, want \"\"", got)
	}
	if err := svc.SetSessionView("s1", api.ViewChat); err != nil {
		t.Fatalf("SetSessionView: %v", err)
	}
	if got := svc.SessionView("s1"); got != api.ViewChat {
		t.Fatalf("SessionView = %q, want %q", got, api.ViewChat)
	}
	sessions, err := svc.sessionsOf("p1")
	if err != nil {
		t.Fatalf("sessionsOf: %v", err)
	}
	if sessions[1].View != api.ViewChat {
		t.Errorf("hydrated View = %q, want %q", sessions[1].View, api.ViewChat)
	}

	if err := svc.SetSessionChatMode("s1", "plan"); err != nil {
		t.Fatalf("SetSessionChatMode: %v", err)
	}
	_ = svc.SetProviderSession("s1", "conv-1")
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "s1", ActiveID: "base"})
	restored, err := svc.ReopenSession(t.Context(), api.ReopenSessionOptions{SessionID: "s1", NewSessionID: "s2"})
	if err != nil || restored == nil {
		t.Fatalf("ReopenSession = %+v, %v", restored, err)
	}
	if restored.View != api.ViewChat || svc.SessionView("s2") != api.ViewChat {
		t.Errorf("resumed view = %q / %q, want %q", restored.View, svc.SessionView("s2"), api.ViewChat)
	}
	if got := svc.SessionChatMode("s2"); got != "plan" {
		t.Errorf("resumed chat mode = %q, want plan", got)
	}
	if project, kind, cwd, err := svc.SessionSpawn("s2"); err != nil || project != "p1" || kind != "claude" || cwd != "/tmp/alpha" {
		t.Errorf("SessionSpawn = %q %q %q %v, want the project's checkout", project, kind, cwd, err)
	}
	if _, _, _, err := svc.SessionSpawn("missing"); err == nil {
		t.Error("SessionSpawn of a missing row succeeded")
	}
	if got := svc.SessionChatMode("missing"); got != "" {
		t.Errorf("SessionChatMode of a missing row = %q, want \"\"", got)
	}
	if got := svc.SessionView("missing"); got != "" {
		t.Errorf("SessionView of a missing row = %q, want \"\"", got)
	}
}
