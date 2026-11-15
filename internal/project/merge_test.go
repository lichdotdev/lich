package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// conflictedMerge leaves repo mid-merge of branch side into main, a.txt in
// conflict and b.txt merged clean.
func conflictedMerge(t *testing.T) (string, func(args ...string) string) {
	t.Helper()
	repo, git := initRepo(t)
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("switch", "-c", "side")
	write("a.txt", "side\n")
	write("b.txt", "new\n")
	git("add", ".")
	git("commit", "-m", "side")
	git("switch", "main")
	write("a.txt", "main\n")
	git("commit", "-am", "main")
	cmd := exec.Command("git", "-C", repo, "merge", "side")
	cmd.Env = append(os.Environ(), testGitIdentity...)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("merge did not conflict: %s", out)
	}
	return repo, git
}

func TestDiffReportsMergeInProgress(t *testing.T) {
	t.Parallel()
	repo, _ := conflictedMerge(t)
	got := New(nil).diff(repo)
	if got.Merging != "side" {
		t.Errorf("Merging = %q, want side", got.Merging)
	}
	if !slices.Equal(got.Conflicted, []string{"a.txt"}) {
		t.Errorf("Conflicted = %q, want [a.txt]", got.Conflicted)
	}
}

func TestDiffOutsideAMergeReportsNone(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	if got := New(nil).diff(repo); got.Merging != "" || got.Conflicted != nil {
		t.Errorf("Merging = %q, Conflicted = %q, want neither", got.Merging, got.Conflicted)
	}
}

func TestMergeSource(t *testing.T) {
	t.Parallel()
	head := "0123456789abcdef0123456789abcdef01234567\n"
	tests := []struct{ msg, want string }{
		{"Merge branch 'fix-post-title'\n\n# Conflicts:\n", "fix-post-title"},
		{"Merge branch 'feat' into dev\n", "feat"},
		{"Merge remote-tracking branch 'origin/main'\n", "origin/main"},
		{"Merge branches 'a' and 'b'\n", "a"},
		{"Merge tag 'v1.2'\n", "v1.2"},
		{"Merge branch 'refs/heads/fix-post-title'\n", "fix-post-title"},
		{"Merge remote-tracking branch 'refs/remotes/origin/main'\n", "origin/main"},
		{"bring the fixes over\n", "0123456"},
		{"", "0123456"},
		{"Merge branch ''\n", "0123456"},
	}
	for _, tt := range tests {
		if got := mergeSource(tt.msg, head); got != tt.want {
			t.Errorf("mergeSource(%q) = %q, want %q", tt.msg, got, tt.want)
		}
	}
	if got := mergeSource("", ""); got != "" {
		t.Errorf("mergeSource with no head = %q, want empty", got)
	}
}

func TestGitMergeAbort(t *testing.T) {
	t.Parallel()
	repo, git := conflictedMerge(t)
	if _, err := New(nil).GitMergeAbort(t.Context(), api.PathOptions{Path: repo}); err != nil {
		t.Fatalf("GitMergeAbort: %v", err)
	}
	if out := git("status", "--porcelain"); out != "" {
		t.Errorf("status after abort = %q, want clean", out)
	}
	if _, err := New(nil).GitMergeAbort(t.Context(), api.PathOptions{Path: repo}); err == nil {
		t.Error("a second abort, with no merge left, succeeded")
	}
}

// Not parallel: the commit reads its identity from the environment.
func TestGitMergeCommitAfterResolving(t *testing.T) {
	setIdentity(t)
	repo, git := conflictedMerge(t)
	s := New(nil)
	if _, err := s.GitMergeCommit(t.Context(), api.PathOptions{Path: repo}); err == nil {
		t.Fatal("committed a merge with a file still in conflict")
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("both\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GitMarkResolved(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: "a.txt"}); err != nil {
		t.Fatalf("GitMarkResolved: %v", err)
	}
	if got := s.diff(repo); got.Conflicted != nil || got.Merging != "side" {
		t.Errorf("after resolving: Merging = %q, Conflicted = %q, want side and none", got.Merging, got.Conflicted)
	}
	if _, err := s.GitMergeCommit(t.Context(), api.PathOptions{Path: repo}); err != nil {
		t.Fatalf("GitMergeCommit: %v", err)
	}
	if parents := strings.Fields(git("log", "-1", "--format=%P")); len(parents) != 2 {
		t.Errorf("HEAD has %d parents, want a merge commit", len(parents))
	}
	if got := s.diff(repo); got.Merging != "" {
		t.Errorf("Merging = %q after the commit, want none", got.Merging)
	}
}

func TestGitMergeCommitRefusedOutsideAMerge(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	if _, err := New(nil).GitMergeCommit(t.Context(), api.PathOptions{Path: repo}); err == nil {
		t.Error("committed staged changes with no merge in progress")
	}
}

func TestGitMarkResolvedRefusesAFileWithoutConflict(t *testing.T) {
	t.Parallel()
	repo, _ := conflictedMerge(t)
	if _, err := New(nil).GitMarkResolved(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: "b.txt"}); err == nil {
		t.Error("marked a file with no conflict resolved")
	}
	if _, err := New(nil).GitMarkResolved(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: "../x"}); err == nil {
		t.Error("accepted a path outside the checkout")
	}
}

func TestRevertRefusedMidMerge(t *testing.T) {
	t.Parallel()
	repo, _ := conflictedMerge(t)
	s := New(nil)
	for _, rel := range []string{"a.txt", "b.txt"} {
		if _, err := s.DiscardFile(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: rel}); err == nil {
			t.Errorf("DiscardFile(%s) ran mid-merge", rel)
		}
	}
	if _, err := s.RevertLines(t.Context(), api.RevertLinesOptions{Path: repo, Rel: "b.txt", Lines: []api.RevertLine{{Side: "new", Line: 1, Text: "new"}}}); err == nil {
		t.Error("RevertLines ran mid-merge")
	}
}

// sideBranch commits a change to name on branch side and goes back to main.
func sideBranch(t *testing.T, repo string, git func(args ...string) string, name, body string) {
	t.Helper()
	git("switch", "-c", "side")
	if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", name)
	git("commit", "-m", "side")
	git("switch", "main")
}

func setIdentity(t *testing.T) {
	for _, kv := range testGitIdentity {
		k, v, _ := strings.Cut(kv, "=")
		t.Setenv(k, v)
	}
}

func TestGitMergePreview(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	sideBranch(t, repo, git, "a.txt", "side\n")
	s := New(nil)
	got, err := s.GitMergePreview(t.Context(), api.GitMergeOptions{Path: repo, Ref: "side"})
	if err != nil || got.Commits != 1 || got.Conflicts != nil {
		t.Fatalf("clean preview = %+v, %v; want 1 commit and no conflict", got, err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("commit", "-am", "main")
	got, err = s.GitMergePreview(t.Context(), api.GitMergeOptions{Path: repo, Ref: "side"})
	if err != nil || !slices.Equal(got.Conflicts, []string{"a.txt"}) {
		t.Errorf("conflicting preview = %+v, %v; want a.txt", got, err)
	}
	if got, err := s.GitMergePreview(t.Context(), api.GitMergeOptions{Path: repo, Ref: "main"}); err != nil || got.Commits != 0 {
		t.Errorf("preview of HEAD itself = %+v, %v; want nothing to bring", got, err)
	}
	for _, ref := range []string{"", "--help", "nope"} {
		if _, err := s.GitMergePreview(t.Context(), api.GitMergeOptions{Path: repo, Ref: ref}); err == nil {
			t.Errorf("preview of %q succeeded", ref)
		}
	}
}

// Not parallel: the merge commit reads its identity from the environment.
func TestGitMergeCleanAndUndo(t *testing.T) {
	setIdentity(t)
	repo, git := initRepo(t)
	sideBranch(t, repo, git, "b.txt", "new\n")
	if err := os.WriteFile(filepath.Join(repo, "c.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "c.txt")
	git("commit", "-m", "main")
	s := New(nil)
	got, err := s.GitMerge(t.Context(), api.GitMergeOptions{Path: repo, Ref: "side"})
	if err != nil || got.After == "" || got.Conflicts != nil {
		t.Fatalf("GitMerge = %+v, %v; want a clean merge", got, err)
	}
	if parents := strings.Fields(git("log", "-1", "--format=%P")); len(parents) != 2 {
		t.Errorf("HEAD has %d parents, want a merge commit", len(parents))
	}
	if _, err := s.GitMergeUndo(t.Context(), api.GitMergeUndoOptions{Path: repo, Before: got.Before, After: got.After}); err != nil {
		t.Fatalf("GitMergeUndo: %v", err)
	}
	if head := git("rev-parse", "HEAD"); head != got.Before {
		t.Errorf("HEAD after undo = %s, want %s", head, got.Before)
	}
	if _, err := s.GitMergeUndo(t.Context(), api.GitMergeUndoOptions{Path: repo, Before: got.Before, After: got.After}); err == nil {
		t.Error("undid a merge HEAD no longer stands on")
	}
}

func TestGitMergeStopsOnConflict(t *testing.T) {
	setIdentity(t)
	repo, git := initRepo(t)
	sideBranch(t, repo, git, "a.txt", "side\n")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("commit", "-am", "main")
	got, err := New(nil).GitMerge(t.Context(), api.GitMergeOptions{Path: repo, Ref: "side"})
	if err != nil || got.After != "" || !slices.Equal(got.Conflicts, []string{"a.txt"}) {
		t.Fatalf("GitMerge = %+v, %v; want stopped on a.txt", got, err)
	}
	if from := mergingFrom(repo); from != "side" {
		t.Errorf("mergingFrom = %q, want side", from)
	}
	if _, err := New(nil).GitMerge(t.Context(), api.GitMergeOptions{Path: repo, Ref: "side"}); err == nil {
		t.Error("started a second merge over a running one")
	}
}

func TestGitMergeRefusesADirtyCheckout(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	sideBranch(t, repo, git, "b.txt", "new\n")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).GitMerge(t.Context(), api.GitMergeOptions{Path: repo, Ref: "side"}); err == nil {
		t.Error("merged into a checkout with uncommitted changes")
	}
}
