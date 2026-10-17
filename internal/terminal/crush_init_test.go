package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

func TestMarkCrushInitializedAtTheCheckoutsTop(t *testing.T) {
	_, worktree := gitWorktree(t)
	sub := filepath.Join(worktree, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	markCrushInitialized(providers.Crush, sub)
	if _, err := os.Stat(filepath.Join(worktree, ".crush", "init")); err != nil {
		t.Fatalf("no init marker at the checkout's top: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sub, ".crush")); !os.IsNotExist(err) {
		t.Errorf("a .crush was made in the subfolder: %v", err)
	}
	// Crush's own .gitignore keeps the folder out of git status.
	out, err := exec.Command("git", "-C", worktree, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), ".crush") {
		t.Errorf("git status shows the marker folder:\n%s", out)
	}
}

func TestMarkCrushInitializedKeepsWhatIsThere(t *testing.T) {
	dir := t.TempDir()
	crush := filepath.Join(dir, ".crush")
	if err := os.Mkdir(crush, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(crush, ".gitignore"), []byte("crush.db\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	markCrushInitialized(providers.Crush, dir)
	if data, _ := os.ReadFile(filepath.Join(crush, ".gitignore")); string(data) != "crush.db\n" {
		t.Errorf(".gitignore was rewritten: %q", data)
	}
	if _, err := os.Stat(filepath.Join(crush, "init")); err != nil {
		t.Errorf("no init marker outside a repository: %v", err)
	}
	other := t.TempDir()
	markCrushInitialized(providers.OpenCode, other)
	if _, err := os.Stat(filepath.Join(other, ".crush")); !os.IsNotExist(err) {
		t.Errorf("an opencode spawn wrote Crush's marker: %v", err)
	}
}
