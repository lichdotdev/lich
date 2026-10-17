package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// gitWorktree is a repository with one commit and a linked worktree of it,
// both as git spells them (resolved, forward slashes on Windows).
func gitWorktree(t *testing.T) (main, worktree string) {
	t.Helper()
	repo := t.TempDir()
	linked := filepath.Join(t.TempDir(), "wt")
	for _, args := range [][]string{
		{"init", "-b", "main", repo},
		{"-C", repo, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "--allow-empty", "-m", "first"},
		{"-C", repo, "worktree", "add", "-b", "side", linked},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Skipf("git %v unavailable: %v (%s)", args, err, out)
		}
	}
	top := func(dir string) string {
		out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
		if err != nil {
			t.Fatal(err)
		}
		return filepath.FromSlash(strings.TrimSpace(string(out)))
	}
	return top(repo), top(linked)
}

// cursorMachine points Cursor CLI's projects dir and the home at scratch
// directories, returning the projects dir.
func cursorMachine(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	data := t.TempDir()
	t.Setenv("CURSOR_DATA_DIR", data)
	return filepath.Join(data, "projects")
}

func markCursorTrusted(t *testing.T, projects, dir string) {
	t.Helper()
	if err := writeCursorTrust(projects, dir); err != nil {
		t.Fatal(err)
	}
}

func TestInheritCursorTrustCarriesTheMainCheckoutsAnswerToItsWorktree(t *testing.T) {
	main, worktree := gitWorktree(t)
	projects := cursorMachine(t)
	markCursorTrusted(t, projects, main)

	inheritCursorTrust(providers.Cursor, worktree)
	if !hasCursorMarker(projects, worktree) {
		t.Fatalf("the worktree of a trusted checkout is not trusted: no %s", filepath.Join(projects, cursorSlug(worktree)))
	}
}

func TestInheritCursorTrustNeverTrustsWhatNobodyTrusted(t *testing.T) {
	main, worktree := gitWorktree(t)
	projects := cursorMachine(t)

	inheritCursorTrust(providers.Cursor, worktree)
	inheritCursorTrust(providers.Cursor, main)
	if _, err := os.Stat(projects); !os.IsNotExist(err) {
		t.Errorf("an untrusted checkout was marked: %v", err)
	}
	markCursorTrusted(t, projects, main)
	inheritCursorTrust(providers.Codex, worktree)
	if hasCursorMarker(projects, worktree) {
		t.Error("a Codex spawn wrote Cursor CLI's trust")
	}
}

// The rule is the bundle's: one dash per run of anything outside [A-Za-z0-9],
// none at either end.
func TestCursorSlug(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"/home/u/.local/share/lich/worktrees/e48f04f46a4a/fuzzy-pine": "home-u-local-share-lich-worktrees-e48f04f46a4a-fuzzy-pine",
		`C:\Users\u\my repo_1\`: "C-Users-u-my-repo-1",
	} {
		if got := cursorSlug(in); got != want {
			t.Errorf("cursorSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

// A marker on a parent counts only below home and three components deep, so
// trusting the home (or / ) once does not trust every folder under it.
func TestCursorTrustsAParentOnlyBelowHome(t *testing.T) {
	t.Parallel()
	sep := string(filepath.Separator)
	home := filepath.Join(sep+"h", "u")
	deep := filepath.Join(home, "src", "app")
	for _, tt := range []struct {
		parent string
		counts bool
	}{
		{filepath.Join(home, "src"), true},
		{home, false},
		{filepath.Dir(home), false},
		{filepath.Join(sep+"opt", "x"), false},
		{filepath.Join(sep+"opt", "x", "y"), true},
	} {
		if got := cursorParentCounts(tt.parent, home); got != tt.counts {
			t.Errorf("cursorParentCounts(%q) = %v, want %v", tt.parent, got, tt.counts)
		}
	}
	projects := t.TempDir()
	markCursorTrusted(t, projects, filepath.Join(home, "src"))
	if !cursorTrusts(projects, home, filepath.Join(deep, "pkg")) {
		t.Error("a folder under a trusted parent is not trusted")
	}
	markCursorTrusted(t, projects, home)
	if cursorTrusts(projects, home, filepath.Join(home, "other")) {
		t.Error("the home's marker trusted a folder under it")
	}
}
