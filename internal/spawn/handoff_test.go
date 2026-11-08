package spawn

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The successor picks the work up where it lies: the caller's worktree and
// folder, on the caller's provider, and the caller stays where it is.
func TestHandoffOpensInTheCallersCheckoutAndFolder(t *testing.T) {
	svc, sessions, worktrees, term, _ := newService(t)
	sessions.projects[0].Sessions[0].Path = "/wt/auth-fix"
	sessions.projects[0].Sessions[0].Folder = "Apps"

	opened, err := svc.Handoff(t.Context(), HandoffOptions{From: "s1"})
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	if len(worktrees.created) != 0 || opened.Path != "/wt/auth-fix" || term.spawns[0].cwd != "/wt/auth-fix" {
		t.Errorf("created %+v, path %q, cwd %q; want the caller's checkout", worktrees.created, opened.Path, term.spawns[0].cwd)
	}
	if opened.Folder != "Apps" || opened.Kind != "codex" {
		t.Errorf("folder %q, kind %q; want the caller's", opened.Folder, opened.Kind)
	}
	if len(term.closed) != 0 || sessions.folders["s1"] != "" {
		t.Errorf("closed %v, refiled %q; the caller is left alone", term.closed, sessions.folders["s1"])
	}
}

// A fresh session of itself: the caller's model, effort, ultracode and
// profile, written on the row before the terminal reads them.
func TestHandoffInheritsTheCallersSettings(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)
	sessions.projects[0].Sessions[0].Kind = "claude"
	sessions.models = map[string]string{"s1": "opus"}
	sessions.efforts = map[string]string{"s1": "high"}
	sessions.ultracodes = map[string]bool{"s1": true}
	sessions.profiles = map[string]string{"s1": "work"}

	opened, err := svc.Handoff(t.Context(), HandoffOptions{From: "s1"})
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	id := opened.ID
	if sessions.models[id] != "opus" || sessions.efforts[id] != "high" || !sessions.ultracodes[id] || sessions.profiles[id] != "work" {
		t.Errorf("model %q, effort %q, ultracode %v, profile %q; want the caller's",
			sessions.models[id], sessions.efforts[id], sessions.ultracodes[id], sessions.profiles[id])
	}
}

// An empty profile on the caller is Personal, not the project's: the new row
// would otherwise be born into whatever profile the project names.
func TestHandoffKeepsACallerOnPersonal(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	opened, err := svc.Handoff(t.Context(), HandoffOptions{From: "s1"})
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	if sessions.profiles[opened.ID] != "personal" {
		t.Errorf("profile %q, want personal", sessions.profiles[opened.ID])
	}
}

func TestHandoffTakesTheSettingsItIsGiven(t *testing.T) {
	svc, sessions, _, term, _ := newService(t)
	sessions.models = map[string]string{"s1": "gpt-5.2"}
	sessions.profiles = map[string]string{"s1": "work"}

	opened, err := svc.Handoff(t.Context(), HandoffOptions{From: "s1", Model: "gpt-5.5", Profile: "spare"})
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	if sessions.models[opened.ID] != "gpt-5.5" || sessions.profiles[opened.ID] != "spare" {
		t.Errorf("model %q, profile %q; want the ones asked for", sessions.models[opened.ID], sessions.profiles[opened.ID])
	}
	if !slices.Equal(term.checkedProfiles, [][2]string{{"codex", "spare"}}) {
		t.Errorf("checked %v, want the profile asked for against codex", term.checkedProfiles)
	}
}

// A model, effort or profile belongs to one provider: handing off to another
// leaves it on its own defaults.
func TestHandoffToAnotherProviderInheritsNothing(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)
	sessions.models = map[string]string{"s1": "gpt-5.2"}
	sessions.efforts = map[string]string{"s1": "high"}
	sessions.profiles = map[string]string{"s1": "work"}

	opened, err := svc.Handoff(t.Context(), HandoffOptions{From: "s1", Kind: "opencode"})
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	id := opened.ID
	if opened.Kind != "opencode" || sessions.models[id] != "" || sessions.efforts[id] != "" || sessions.profiles[id] != "" {
		t.Errorf("kind %q, model %q, effort %q, profile %q; want opencode on its defaults",
			opened.Kind, sessions.models[id], sessions.efforts[id], sessions.profiles[id])
	}
}

func TestHandoffRefusesAProfileTheProviderLacks(t *testing.T) {
	svc, sessions, _, term, _ := newService(t)
	term.profileErr = errors.New(`codex profile is gone: "spare" was deleted`)

	if _, err := svc.Handoff(t.Context(), HandoffOptions{From: "s1", Profile: "spare"}); err == nil || !strings.Contains(err.Error(), "spare") {
		t.Fatalf("Handoff = %v, want the profile refused", err)
	}
	if len(sessions.rows) != 0 || len(term.spawns) != 0 {
		t.Errorf("rows %+v, spawns %+v; nothing should open", sessions.rows, term.spawns)
	}
}

func TestHandoffRefusesACallerThatIsNoSession(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	if _, err := svc.Handoff(t.Context(), HandoffOptions{From: ""}); err == nil || !strings.Contains(err.Error(), "handoff") {
		t.Fatalf("Handoff = %v, want it refused outside a session", err)
	}
	if len(sessions.rows) != 0 {
		t.Errorf("rows %+v, want none", sessions.rows)
	}
}

// The successor is still opened when its profile cannot be written, and the
// failure says so: hiding the card would leave the user nothing to look at.
func TestHandoffReportsAProfileItCouldNotRecord(t *testing.T) {
	svc, sessions, _, term, _ := newService(t)
	sessions.profileErr = errors.New("disk full")

	if _, err := svc.Handoff(t.Context(), HandoffOptions{From: "s1"}); err == nil || !strings.Contains(err.Error(), "profile") {
		t.Fatalf("Handoff = %v, want the profile write reported", err)
	}
	if len(term.spawns) != 1 {
		t.Errorf("spawns %+v, want the session started anyway", term.spawns)
	}
}
