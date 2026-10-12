package quota

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/claudeprofile"
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
