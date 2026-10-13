package project

import (
	"os"
	"path/filepath"
	"testing"
)

// originWithMain wires repo to a second repository as origin, with main
// tracking origin/main, and returns origin's path and a git runner inside it.
func originWithMain(t *testing.T, git func(args ...string) string) (string, func(args ...string) string) {
	t.Helper()
	origin, originGit := initRepo(t)
	git("remote", "add", "origin", origin)
	git("fetch", "origin")
	git("reset", "--hard", "origin/main")
	git("branch", "-u", "origin/main", "main")
	return origin, originGit
}

func commitIn(t *testing.T, dir string, git func(args ...string) string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	git("commit", "-am", content)
}

func TestBaseFreshnessWithoutUpstream(t *testing.T) {
	repo, _ := initRepo(t)

	got, err := New(nil).BaseFreshness(repo, "main")
	if err != nil {
		t.Fatalf("BaseFreshness: %v", err)
	}
	if got != (BaseFreshness{}) {
		t.Errorf("BaseFreshness(no upstream) = %+v, want zero", got)
	}
}

// TestBaseFreshnessBehindFetchesFirst proves the count is taken after a fetch:
// the commit merged on origin since the last fetch is what makes main behind.
func TestBaseFreshnessBehindFetchesFirst(t *testing.T) {
	repo, git := initRepo(t)
	origin, originGit := originWithMain(t, git)
	commitIn(t, origin, originGit, "merged\n")

	got, err := New(nil).BaseFreshness(repo, "main")
	if err != nil {
		t.Fatalf("BaseFreshness: %v", err)
	}
	want := BaseFreshness{Upstream: "origin/main", Behind: 1}
	if got != want {
		t.Errorf("BaseFreshness = %+v, want %+v", got, want)
	}
}

func TestBaseFreshnessAhead(t *testing.T) {
	repo, git := initRepo(t)
	originWithMain(t, git)
	commitIn(t, repo, git, "local\n")

	got, err := New(nil).BaseFreshness(repo, "main")
	if err != nil {
		t.Fatalf("BaseFreshness: %v", err)
	}
	want := BaseFreshness{Upstream: "origin/main", Ahead: 1}
	if got != want {
		t.Errorf("BaseFreshness = %+v, want %+v", got, want)
	}
}

// TestBaseFreshnessFetchFailure proves an unreachable remote is reported in
// the answer, not as an error: the dialog still opens the worktree, from the
// local branch, and says why.
func TestBaseFreshnessFetchFailure(t *testing.T) {
	repo, git := initRepo(t)
	originWithMain(t, git)
	git("remote", "set-url", "origin", filepath.Join(t.TempDir(), "unreachable"))

	got, err := New(nil).BaseFreshness(repo, "main")
	if err != nil {
		t.Fatalf("BaseFreshness: %v", err)
	}
	if got.Upstream != "origin/main" || got.FetchError == "" {
		t.Errorf("BaseFreshness(unreachable) = %+v, want origin/main with a fetch error", got)
	}
}

func TestBaseFreshnessNotABranch(t *testing.T) {
	repo, _ := initRepo(t)

	if _, err := New(nil).BaseFreshness(repo, "nope"); err == nil {
		t.Error("BaseFreshness(unknown branch) = nil error, want a refusal")
	}
}
