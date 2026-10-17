package agentplugin

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

// workProfile is a scratch home with one Claude Code profile in it, and the
// profile's directory.
func workProfile(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", "")
	t.Setenv("LICH_DEV", "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := claudeprofile.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	return p.Dir
}

// TestInstallInClaudeProfileRunsAsTheProfile: the same pinned install Personal
// gets, every call run with the profile's directory and none of the token that
// would put another account's policy in charge.
func TestInstallInClaudeProfileRunsAsTheProfile(t *testing.T) {
	dir := workProfile(t)
	svc, calls := fakeCLI(t, "")
	envLog := filepath.Join(t.TempDir(), "env.log")
	t.Setenv(fakeCLIEnvLog, envLog)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "personal-token")

	if err := svc.InstallInClaudeProfile("work"); err != nil {
		t.Fatal(err)
	}
	if got := calls(); !slices.Equal(got, pinnedCalls[providers.Claude]) {
		t.Errorf("calls %v, want %v", got, pinnedCalls[providers.Claude])
	}
	data, err := os.ReadFile(envLog)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != dir+"|" {
			t.Errorf("a call ran as %q, want %q", line, dir+"|")
		}
	}
	if err := svc.InstallInClaudeProfile("gone"); err == nil {
		t.Error("installed into a profile that does not exist")
	}
}

func TestInstallInPersonalRunsInLichsEnvironment(t *testing.T) {
	workProfile(t)
	svc, _ := fakeCLI(t, "")
	envLog := filepath.Join(t.TempDir(), "env.log")
	t.Setenv(fakeCLIEnvLog, envLog)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "personal-token")
	if err := svc.InstallInClaudeProfile(claudeprofile.Personal); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(envLog)
	if !strings.HasPrefix(string(data), "|personal-token") {
		t.Errorf("Personal ran with %q, want lich's own environment", data)
	}
}

func TestClaudeProfileStatusReadsTheProfilesPlugins(t *testing.T) {
	dir := workProfile(t)
	svc, _ := fakeCLI(t, "")
	svc.lookPath = func(string) (string, error) { return "claude", nil }
	st, err := svc.ClaudeProfileStatus("work")
	if err != nil {
		t.Fatal(err)
	}
	if st.Installed || ClaudeInstalledIn(dir) {
		t.Errorf("a new profile reads as installed: %+v", st)
	}
	state := `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"` + testVersion + `"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugins", "installed_plugins.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err = svc.ClaudeProfileStatus("work")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Installed || st.InstalledVersion != testVersion || !ClaudeInstalledIn(dir) {
		t.Errorf("after install: %+v", st)
	}
	if _, err := svc.ClaudeProfileStatus("gone"); err == nil {
		t.Error("a status for a profile that does not exist")
	}
}
