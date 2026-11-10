package quota

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

const claudeSpentBody = `{
	"five_hour": {"utilization": 100.0, "resets_at": "2026-08-17T23:20:00+00:00"},
	"seven_day": {"utilization": 40.0, "resets_at": "2026-08-24T15:00:00+00:00"}
}`

// TestHasQuotaLeftReadsTheProfilesPlan: a profile with every window under its
// end has quota left; one with a window spent has none; and one whose plan
// cannot be read (no login there) or reads with no windows is not known to
// have any, which answers no.
func TestHasQuotaLeftReadsTheProfilesPlan(t *testing.T) {
	for _, tc := range []struct {
		name  string
		creds string
		body  string
		want  bool
	}{
		{"quota left", claudeCredsJSON, claudeLimitsBody, true},
		{"a window spent", claudeCredsJSON, claudeSpentBody, false},
		{"signed out", "", claudeLimitsBody, false},
		{"no windows", claudeCredsJSON, `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slug := claudeProfileWithLogin(t, tc.creds)
			url, _ := serve(t, http.StatusOK, tc.body)
			if got := newService(url, "", time.Now()).HasQuotaLeft(providers.Claude, slug); got != tc.want {
				t.Errorf("HasQuotaLeft = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasQuotaLeftAnswersNoForAProviderWithoutProfileReadings(t *testing.T) {
	url, calls := serve(t, http.StatusOK, claudeLimitsBody)
	if newService(url, url, time.Now()).HasQuotaLeft(providers.OMP, "work") || *calls != 0 {
		t.Errorf("an oh-my-pi profile read as having quota after %d requests", *calls)
	}
}

// claudeProfileWithLogin creates a Claude Code profile under a fresh home with
// creds as its login ("" for none) and returns its slug.
func claudeProfileWithLogin(t *testing.T, creds string) string {
	t.Helper()
	writeCreds(t, "", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("LICH_DEV", "")
	t.Setenv(claudeTokenVar, "")
	t.Setenv(claudeDirVar, "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := claudeprofile.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if creds != "" {
		if err := os.WriteFile(filepath.Join(p.Dir, ".credentials.json"), []byte(creds), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return p.Slug
}
