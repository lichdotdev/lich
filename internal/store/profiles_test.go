package store

import "testing"

// TestClaudeProfileResolvesLikeTheSandboxRung: a project's own value wins over
// the global one, "" in a project inherits, and "personal" is how a project
// names Personal over a global profile.
func TestClaudeProfileResolvesLikeTheSandboxRung(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")
	if got := svc.ClaudeProfile("p1"); got != "" {
		t.Errorf("nothing set: %q, want Personal", got)
	}
	_ = svc.SetSetting(claudeProfileKey, "", "work")
	_ = svc.SetSetting(claudeProfileKey, "p2", "personal")
	if got := svc.ClaudeProfile("p1"); got != "work" {
		t.Errorf("inherited: %q, want work", got)
	}
	if got := svc.ClaudeProfile("p2"); got != "" {
		t.Errorf("project naming Personal: %q, want Personal", got)
	}
	_ = svc.SetSetting(claudeProfileKey, "p1", "client")
	if got := svc.ClaudeProfile("p1"); got != "client" {
		t.Errorf("project override: %q, want client", got)
	}
}

// TestASessionKeepsTheProfileItWasBornIn: the profile is written with the row,
// for Claude Code only, and the project's setting moving later leaves a session
// that already exists where it is, through a park and resume too.
func TestASessionKeepsTheProfileItWasBornIn(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.SetSetting(claudeProfileKey, "p1", "work")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")
	_ = svc.AddSession("p1", "cx", "Codex", "codex", "", 4, "")
	_ = svc.SetSetting(claudeProfileKey, "p1", "client")

	for id, want := range map[string]string{"base": "work", "wt1": "work", "cx": ""} {
		if got, err := svc.SessionProfile(id); err != nil || got != want {
			t.Errorf("SessionProfile(%s) = %q, %v, want %q", id, got, err, want)
		}
	}
	sessions := mustLoadSessions(t, svc)
	if sessions[0].Profile != "work" {
		t.Errorf("hydrated profile %q, want work", sessions[0].Profile)
	}

	_ = svc.CloseSession("p1", "wt1", "base")
	restored, err := svc.ReopenWorktreeSession("p1", "/wt/foo", "wt2")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.SessionProfile("wt2"); got != "work" || restored.Profile != "work" {
		t.Errorf("resumed profile %q (reported %q), want work", got, restored.Profile)
	}
}

func TestSetSessionProfileMovesOneSession(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "claude", "", 2, "")
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
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")
	_ = svc.SetSetting(claudeProfileKey, "", "work")
	_ = svc.SetSetting(claudeProfileKey, "p1", "work")
	_ = svc.SetSetting(claudeProfileKey, "p2", "client")
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
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.SetSetting(claudeProfileKey, "p1", "claude-work")
	_ = svc.SetSetting(codexProfileKey, "p1", "codex-work")
	if got := svc.CodexProfile("p1"); got != "codex-work" {
		t.Errorf("CodexProfile = %q, want codex-work", got)
	}
	if got := svc.ClaudeProfile("p1"); got != "claude-work" {
		t.Errorf("ClaudeProfile = %q, want claude-work", got)
	}

	_ = svc.AddSession("p1", "cc", "Claude", "claude", "", 2, "")
	_ = svc.AddSession("p1", "cx", "Codex", "codex", "", 3, "")
	_ = svc.AddSession("p1", "cr", "Crush", "crush", "", 4, "")
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
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.SetSetting(codexProfileKey, "p1", "codex-work")
	_ = svc.SetSetting(ompProfileKey, "p1", "omp-work")
	if got := svc.OMPProfile("p1"); got != "omp-work" {
		t.Errorf("OMPProfile = %q, want omp-work", got)
	}
	_ = svc.AddSession("p1", "cx", "Codex", "codex", "", 2, "")
	_ = svc.AddSession("p1", "om", "omp", "omp", "", 3, "")
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
