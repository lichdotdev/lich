package terminal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// TestInheritClaudeTrustReachesOnlyAClaudeProfile: a Claude Code spawn into a
// profile, in a repository trusted in Personal, starts trusted there; a spawn
// of another provider into the same directory leaves the file alone.
func TestInheritClaudeTrustReachesOnlyAClaudeProfile(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	// Spelled the way git spells it, which is what Claude Code keys its table
	// by: resolved, and in forward slashes on Windows.
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	top := strings.TrimSpace(string(out))
	personal, profile := t.TempDir(), t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", personal)
	trusted, err := json.Marshal(map[string]any{
		"projects": map[string]any{filepath.ToSlash(top): map[string]bool{"hasTrustDialogAccepted": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(personal, ".claude.json"), trusted, 0o600); err != nil {
		t.Fatal(err)
	}
	profileFile := filepath.Join(profile, ".claude.json")
	if err := os.WriteFile(profileFile, []byte(`{"projects":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	inheritClaudeTrust(providers.Codex, repo, profile)
	if data, _ := os.ReadFile(profileFile); string(data) != `{"projects":{}}` {
		t.Fatalf("a Codex spawn wrote Claude Code's config: %s", data)
	}
	inheritClaudeTrust(providers.Claude, repo, profile)
	var doc struct {
		Projects map[string]map[string]bool `json:"projects"`
	}
	data, err := os.ReadFile(profileFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Projects[filepath.ToSlash(top)]["hasTrustDialogAccepted"] {
		t.Errorf("the profile does not trust %s: %s", top, data)
	}
}
