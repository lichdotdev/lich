package agentplugin

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestInstallInCodexProfileRunsAsTheProfile: the same pinned install Personal
// gets, every call run with the profile's CODEX_HOME and none of the key that
// would stand in for its login.
func TestInstallInCodexProfileRunsAsTheProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("CODEX_HOME", "")
	t.Setenv("LICH_DEV", "")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := codexprofile.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	svc, calls := fakeCLI(t, "")
	envLog := filepath.Join(t.TempDir(), "env.log")
	t.Setenv(fakeCLICodexEnvLog, envLog)
	t.Setenv("CODEX_API_KEY", "personal-key")

	if err := svc.InstallInCodexProfile(p.Slug); err != nil {
		t.Fatal(err)
	}
	if got := calls(); !slices.Equal(got, pinnedCalls[providers.Codex]) {
		t.Errorf("calls %v, want %v", got, pinnedCalls[providers.Codex])
	}
	data, err := os.ReadFile(envLog)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != p.Dir+"|" {
			t.Errorf("a call ran as %q, want %q", line, p.Dir+"|")
		}
	}
	if err := svc.InstallInCodexProfile("gone"); err == nil {
		t.Error("installed into a profile that does not exist")
	}

	if err := os.Remove(envLog); err != nil {
		t.Fatal(err)
	}
	if err := svc.InstallInCodexProfile(codexprofile.Personal); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(envLog); !strings.HasPrefix(string(data), "|personal-key") {
		t.Errorf("Personal ran with %q, want lich's own environment", data)
	}
}
