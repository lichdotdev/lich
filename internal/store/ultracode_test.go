package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func TestSessionUltracodeRoundTrips(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s2", Label: "Session 2", Kind: "claude", NextSeq: 3})

	if err := svc.SetSessionUltracode("s1"); err != nil {
		t.Fatalf("SetSessionUltracode: %v", err)
	}
	if !svc.SessionUltracode("s1") {
		t.Error("SessionUltracode(s1) = false, want true")
	}
	if svc.SessionUltracode("s2") {
		t.Error("SessionUltracode(s2) = true, want false for a session opened without it")
	}
	if svc.SessionUltracode("ghost") {
		t.Error("SessionUltracode(ghost) = true, want false")
	}
}

// TestReopenWorktreeSessionCarriesUltracode pins the park/resume cycle, which
// keeps a card's identity under a new id: a reinsert that dropped the flag would
// respawn the session without ultracode and nothing on screen would say so.
func TestReopenWorktreeSessionCarriesUltracode(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "base", Label: "Session 1", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "wt1", Label: "worker", Kind: "claude", Path: "/wt/foo", NextSeq: 3})
	_ = svc.SetSessionUltracode("wt1")
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "wt1", ActiveID: "base"})

	if _, err := svc.ReopenWorktreeSession(t.Context(), api.ReopenWorktreeSessionOptions{ProjectID: "p1", Path: "/wt/foo", NewSessionID: "wt2"}); err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if !svc.SessionUltracode("wt2") {
		t.Error("resumed session lost ultracode")
	}
}

func TestUltracodeIsOnOnlyForTheLiteralTrue(t *testing.T) {
	svc := newTestStore(t)
	if svc.Ultracode(providers.Claude) {
		t.Error("unconfigured provider runs ultracode, want off")
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.claude.ultracode", ProjectID: globalScope, Value: "yes"})
	if svc.Ultracode(providers.Claude) {
		t.Error(`"yes" turns ultracode on, want only "true" to`)
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.claude.ultracode", ProjectID: globalScope, Value: "true"})
	if !svc.Ultracode(providers.Claude) {
		t.Error("configured provider does not run ultracode, want on")
	}
	if svc.Ultracode(providers.Codex) {
		t.Error("another provider follows Claude's setting, want its own")
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.claude.ultracode", ProjectID: "p1", Value: "false"})
	if !svc.Ultracode(providers.Claude) {
		t.Error("a project-scoped value overrides the global one, want global only")
	}
}

// TestInheritUltracodeFollowsTheForkedConversation pins the lookup a fork makes:
// by the provider conversation id it branches, never by anything the fork's own
// row carries.
func TestInheritUltracodeFollowsTheForkedConversation(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "parent", Label: "Session 1", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "plain", Label: "Session 2", Kind: "claude", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "fork1", Label: "Session 3", Kind: "claude", Path: "/wt/a", NextSeq: 4})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "fork2", Label: "Session 4", Kind: "claude", Path: "/wt/b", NextSeq: 5})
	_ = svc.SetProviderSession("parent", "conv-on")
	_ = svc.SetProviderSession("plain", "conv-off")
	_ = svc.SetSessionUltracode("parent")

	if err := svc.InheritUltracode("fork1", "conv-on"); err != nil {
		t.Fatalf("InheritUltracode: %v", err)
	}
	if !svc.SessionUltracode("fork1") {
		t.Error("fork of an ultracode conversation = off, want on")
	}
	if err := svc.InheritUltracode("fork2", "conv-off"); err != nil {
		t.Fatalf("InheritUltracode: %v", err)
	}
	if svc.SessionUltracode("fork2") {
		t.Error("fork of a plain conversation = on, want off")
	}
}
