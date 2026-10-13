package quota

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/codexprofile"
)

// TestProfilePlanReadsTheProfilesOwnLogin: a profile is read from its own
// directory with no token in front of it, once per cache window, and a profile
// that is gone is an error rather than somebody else's numbers.
func TestProfilePlanReadsTheProfilesOwnLogin(t *testing.T) {
	writeCreds(t, "", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("LICH_DEV", "")
	t.Setenv(claudeTokenVar, "personal-token")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(claudeDirVar, "")
	p, err := claudeprofile.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Dir, ".credentials.json"), []byte(claudeCredsJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	url, calls := serve(t, http.StatusOK, claudeLimitsBody)
	svc := newService(url, "", time.Now())

	got, err := svc.ProfilePlan(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusOK || got.Plan != "Max 5x" {
		t.Errorf("profile plan %+v, want the profile's own Max 5x login", got)
	}
	if _, err := svc.ProfilePlan(p.Slug); err != nil || *calls != 1 {
		t.Errorf("second reading made %d requests, %v; want the cached one", *calls, err)
	}
	if _, err := svc.ProfilePlan("gone"); err == nil {
		t.Error("a plan for a profile that does not exist")
	}
}

// TestPersonalProfilePlanSharesTheMachineReading: Personal is the machine's own
// login, so reading it through the profile route and through Plans("") is one
// upstream request, not two under different cache keys.
func TestPersonalProfilePlanSharesTheMachineReading(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, calls := serve(t, http.StatusOK, claudeLimitsBody)
	svc := newService(url, "", time.Now())

	machine := svc.Plans("")[0]
	got, err := svc.ProfilePlan(claudeprofile.Personal)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, machine) {
		t.Errorf("personal plan = %+v, want the machine's %+v", got, machine)
	}
	if *calls != 1 {
		t.Errorf("calls = %d, want 1: both routes share one reading", *calls)
	}
}

// TestCodexProfilePlanReadsTheProfilesOwnLogin: a Codex profile is read from its
// own CODEX_HOME, under its own account, once per cache window, and Personal is
// the machine's reading.
func TestCodexProfilePlanReadsTheProfilesOwnLogin(t *testing.T) {
	writeCreds(t, "", codexCredsJSON)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("LICH_DEV", "")
	t.Setenv(codexprofile.HomeVar, filepath.Join(home, ".codex"))
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := codexprofile.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	login := codexCredsWith(idToken(t, map[string]any{"email": "work@example.com", "exp": now.Unix() + 3600}))
	if err := os.WriteFile(filepath.Join(p.Dir, "auth.json"), []byte(login), 0o600); err != nil {
		t.Fatal(err)
	}
	url, calls := serve(t, http.StatusOK, codexUsageBody)
	svc := newService("", url, now)

	got, err := svc.CodexProfilePlan(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusOK || got.Account != "work@example.com" {
		t.Errorf("profile plan %+v, want the profile's own login", got)
	}
	if _, err := svc.CodexProfilePlan(p.Slug); err != nil || *calls != 1 {
		t.Errorf("second reading made %d requests, %v; want the cached one", *calls, err)
	}
	if _, err := svc.CodexProfilePlan("gone"); err == nil {
		t.Error("a plan for a profile that does not exist")
	}
	machine, err := svc.CodexProfilePlan(codexprofile.Personal)
	if err != nil || machine.Provider != "codex" || machine.Account == "work@example.com" {
		t.Errorf("personal plan %+v, %v, want the machine's own reading", machine, err)
	}
}
