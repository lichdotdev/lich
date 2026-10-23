package store

import (
	"github.com/lichdotdev/lich/api"
	"testing"
)

// TestClaudeProfileResolvesLikeTheSandboxRung: a project's own value wins over
// the global one, "" in a project inherits, and "personal" is how a project
// names Personal over a global profile.
func TestClaudeProfileResolvesLikeTheSandboxRung(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p2", Name: "beta", Path: "/tmp/beta"})
	if got := svc.ClaudeProfile("p1"); got != "" {
		t.Errorf("nothing set: %q, want Personal", got)
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeProfileKey, Value: "work"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeProfileKey, ProjectID: "p2", Value: "personal"})
	if got := svc.ClaudeProfile("p1"); got != "work" {
		t.Errorf("inherited: %q, want work", got)
	}
	if got := svc.ClaudeProfile("p2"); got != "" {
		t.Errorf("project naming Personal: %q, want Personal", got)
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeProfileKey, ProjectID: "p1", Value: "client"})
	if got := svc.ClaudeProfile("p1"); got != "client" {
		t.Errorf("project override: %q, want client", got)
	}
}

// TestASessionKeepsTheProfileItWasBornIn: the profile is written with the row,
// for Claude Code only, and the project's setting moving later leaves a session
// that already exists where it is, through a park and resume too.
func TestASessionKeepsTheProfileItWasBornIn(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeProfileKey, ProjectID: "p1", Value: "work"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "base", Label: "Session 1", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "wt1", Label: "worker", Kind: "claude", Path: "/wt/foo", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "cx", Label: "Codex", Kind: "codex", NextSeq: 4})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeProfileKey, ProjectID: "p1", Value: "client"})

	for id, want := range map[string]string{"base": "work", "wt1": "work", "cx": ""} {
		if got, err := svc.SessionProfile(id); err != nil || got != want {
			t.Errorf("SessionProfile(%s) = %q, %v, want %q", id, got, err, want)
		}
	}
	sessions := mustLoadSessions(t, svc)
	if sessions[0].Profile != "work" {
		t.Errorf("hydrated profile %q, want work", sessions[0].Profile)
	}

	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "wt1", ActiveID: "base"})
	restored, err := svc.ReopenWorktreeSession(t.Context(), api.ReopenWorktreeSessionOptions{ProjectID: "p1", Path: "/wt/foo", NewSessionID: "wt2"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.SessionProfile("wt2"); got != "work" || restored.Profile != "work" {
		t.Errorf("resumed profile %q (reported %q), want work", got, restored.Profile)
	}
}

func TestSetSessionProfileMovesOneSession(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", Kind: "claude", NextSeq: 2})
	if err := svc.SetSessionProfile("s1", "work"); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.SessionProfile("s1"); got != "work" {
		t.Errorf("moved to %q, want work", got)
	}
	if err := svc.SetSessionProfile("s1", "personal"); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.SessionProfile("s1"); got != "" {
		t.Errorf("moved back to %q, want Personal", got)
	}
	if got, err := svc.SessionProfile("ghost"); got != "" || err != nil {
		t.Errorf("no row: %q, %v", got, err)
	}
}

func TestSessionProfileOnAClosedStoreIsAnError(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.Close()
	if _, err := svc.SessionProfile("s1"); err == nil {
		t.Error("a failed read answered Personal")
	}
	if err := svc.SetSessionProfile("s1", "work"); err == nil {
		t.Error("a failed write reported nothing")
	}
	if err := svc.ForgetClaudeProfile("work"); err == nil {
		t.Error("a failed forget reported nothing")
	}
}

func TestForgetClaudeProfileClearsOnlyItsSettings(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p2", Name: "beta", Path: "/tmp/beta"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeProfileKey, Value: "work"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeProfileKey, ProjectID: "p1", Value: "work"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeProfileKey, ProjectID: "p2", Value: "client"})
	if err := svc.ForgetClaudeProfile("work"); err != nil {
		t.Fatal(err)
	}
	if got := svc.ClaudeProfile("p1"); got != "" {
		t.Errorf("p1 after forgetting work: %q, want Personal", got)
	}
	if got := svc.ClaudeProfile("p2"); got != "client" {
		t.Errorf("p2 lost its own profile: %q", got)
	}
}

// TestCodexProfilesAreTheirOwnSet: Codex's setting, born profile and forget
// follow the same rules as Claude Code's under a key of their own, so a slug
// set for one provider never reaches the other.
func TestCodexProfilesAreTheirOwnSet(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeProfileKey, ProjectID: "p1", Value: "claude-work"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: codexProfileKey, ProjectID: "p1", Value: "codex-work"})
	if got := svc.CodexProfile("p1"); got != "codex-work" {
		t.Errorf("CodexProfile = %q, want codex-work", got)
	}
	if got := svc.ClaudeProfile("p1"); got != "claude-work" {
		t.Errorf("ClaudeProfile = %q, want claude-work", got)
	}

	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "cc", Label: "Claude", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "cx", Label: "Codex", Kind: "codex", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "cr", Label: "Crush", Kind: "crush", NextSeq: 4})
	for id, want := range map[string]string{"cc": "claude-work", "cx": "codex-work", "cr": ""} {
		if got, err := svc.SessionProfile(id); err != nil || got != want {
			t.Errorf("SessionProfile(%s) = %q, %v, want %q", id, got, err, want)
		}
	}
	for id, want := range map[string]string{"cc": "claude", "cx": "codex", "ghost": ""} {
		if got := svc.SessionKind(id); got != want {
			t.Errorf("SessionKind(%s) = %q, want %q", id, got, want)
		}
	}

	if err := svc.ForgetCodexProfile("codex-work"); err != nil {
		t.Fatal(err)
	}
	if got := svc.CodexProfile("p1"); got != "" {
		t.Errorf("after forgetting: %q, want Personal", got)
	}
	if got := svc.ClaudeProfile("p1"); got != "claude-work" {
		t.Errorf("forgetting a Codex profile took the Claude one: %q", got)
	}
}

// TestOMPProfilesAreTheirOwnSet: oh-my-pi's setting, born profile and forget
// follow the same rules under a key of its own.
func TestOMPProfilesAreTheirOwnSet(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: codexProfileKey, ProjectID: "p1", Value: "codex-work"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: ompProfileKey, ProjectID: "p1", Value: "omp-work"})
	if got := svc.OMPProfile("p1"); got != "omp-work" {
		t.Errorf("OMPProfile = %q, want omp-work", got)
	}
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "cx", Label: "Codex", Kind: "codex", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "om", Label: "omp", Kind: "omp", NextSeq: 3})
	for id, want := range map[string]string{"cx": "codex-work", "om": "omp-work"} {
		if got, err := svc.SessionProfile(id); err != nil || got != want {
			t.Errorf("SessionProfile(%s) = %q, %v, want %q", id, got, err, want)
		}
	}

	if err := svc.ForgetOMPProfile("omp-work"); err != nil {
		t.Fatal(err)
	}
	if got := svc.OMPProfile("p1"); got != "" {
		t.Errorf("after forgetting: %q, want Personal", got)
	}
	if got := svc.CodexProfile("p1"); got != "codex-work" {
		t.Errorf("forgetting an omp profile took the Codex one: %q", got)
	}
}

// TestXDGProfilesAreEachProvidersOwnSetting: an opencode, Cursor CLI or Kiro CLI
// session is born in its own provider's profile, and forgetting one clears that
// provider's setting alone.
func TestXDGProfilesAreEachProvidersOwnSetting(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.opencode.profile", ProjectID: "p1", Value: "work"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.kiro.profile", Value: "work"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "oc", Label: "opencode", Kind: "opencode", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "ki", Label: "Kiro", Kind: "kiro", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "cu", Label: "Cursor", Kind: "cursor", NextSeq: 4})
	for id, want := range map[string]string{"oc": "work", "ki": "work", "cu": ""} {
		if got, err := svc.SessionProfile(id); err != nil || got != want {
			t.Errorf("SessionProfile(%s) = %q, %v, want %q", id, got, err, want)
		}
	}
	if err := svc.ForgetXDGProfile("kiro", "work"); err != nil {
		t.Fatal(err)
	}
	if got := svc.bornProfile("kiro", "p1"); got != "" {
		t.Errorf("a forgotten Kiro profile is still born into: %q", got)
	}
	if got := svc.bornProfile("opencode", "p1"); got != "work" {
		t.Errorf("forgetting Kiro's work took opencode's: %q", got)
	}
	if err := svc.ForgetXDGProfile("crush", "work"); err == nil {
		t.Error("forgot a profile of a provider with none")
	}
}
