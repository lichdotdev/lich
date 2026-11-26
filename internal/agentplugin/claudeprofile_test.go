package agentplugin

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/claudeprofile"
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

// claudeUninstallCalls is what taking the install of the past out of a Claude
// Code home runs: the plugin goes, then its marketplace.
var claudeUninstallCalls = []string{
	"plugin uninstall " + pluginKey,
	"plugin marketplace remove " + marketplaceName,
}

// TestInstallInClaudeProfileRunsAsTheProfile: the install of the past comes out
// of the profile's own plugin set, every call run with the profile's directory
// and none of the token that would put another account's policy in charge.
func TestInstallInClaudeProfileRunsAsTheProfile(t *testing.T) {
	dir := workProfile(t)
	writeClaudePlugin(t, dir, testVersion)
	svc, calls := fakeCLI(t, "")
	envLog := filepath.Join(t.TempDir(), "env.log")
	t.Setenv(fakeCLIEnvLog, envLog)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "personal-token")

	if err := svc.InstallInClaudeProfile("work"); err != nil {
		t.Fatal(err)
	}
	if got := calls(); !slices.Equal(got, claudeUninstallCalls) {
		t.Errorf("calls %v, want %v", got, claudeUninstallCalls)
	}
	for _, line := range envLines(t, envLog) {
		if line != dir+"|" {
			t.Errorf("a call ran as %q, want %q", line, dir+"|")
		}
	}
	if _, version, ok := BundleDir(); !ok || version != testVersion {
		t.Errorf("BundleDir version = (%q,%v), want the release the profile's sessions load", version, ok)
	}
	if err := svc.InstallInClaudeProfile("gone"); err == nil {
		t.Error("installed into a profile that does not exist")
	}
}

// A profile with no install of the past costs no Claude Code call at all.
func TestInstallInClaudeProfileWithNothingToTakeOut(t *testing.T) {
	workProfile(t)
	svc, calls := fakeCLI(t, "")
	if err := svc.InstallInClaudeProfile("work"); err != nil {
		t.Fatal(err)
	}
	if got := calls(); got != nil {
		t.Errorf("calls %v, want none", got)
	}
}

func TestInstallInPersonalRunsInLichsEnvironment(t *testing.T) {
	workProfile(t)
	home, _ := os.UserHomeDir()
	writeClaudePlugin(t, filepath.Join(home, ".claude"), testVersion)
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

// A profile's row is the release its sessions load, not what its own plugin
// set holds.
func TestClaudeProfileStatusIsTheRelease(t *testing.T) {
	dir := workProfile(t)
	writeClaudePlugin(t, dir, testVersion)
	svc, _ := fakeCLI(t, "")
	svc.lookPath = func(string) (string, error) { return "claude", nil }
	st, err := svc.ClaudeProfileStatus("work")
	if err != nil {
		t.Fatal(err)
	}
	if st.Installed || ClaudeInstalledIn(dir) {
		t.Errorf("an install in the profile with no release unpacked reads as installed: %+v", st)
	}
	plantRelease(t, bundleHome(t), testVersion)
	st, err = svc.ClaudeProfileStatus("work")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Installed || st.InstalledVersion != testVersion || !ClaudeInstalledIn(dir) {
		t.Errorf("with the release unpacked: %+v", st)
	}
	if _, err := svc.ClaudeProfileStatus("gone"); err == nil {
		t.Error("a status for a profile that does not exist")
	}
}
