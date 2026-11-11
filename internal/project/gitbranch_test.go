package project

import (
	"context"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func currentBranch(git func(...string) string) string {
	return strings.TrimSpace(git("branch", "--show-current"))
}

func TestGitBranchLifecycle(t *testing.T) {
	repo, git := initRepo(t)
	s, ctx := new(Service), context.Background()
	if _, err := s.GitCreateBranch(ctx, api.GitCreateBranchOptions{Path: repo, Name: "feat/x", From: "main"}); err != nil {
		t.Fatal(err)
	}
	if currentBranch(git) != "main" {
		t.Error("creating a branch must not switch to it")
	}
	if _, err := s.GitRenameBranch(ctx, api.GitRenameBranchOptions{Path: repo, Name: "feat/x", NewName: "feat/y"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GitCheckout(ctx, api.GitCheckoutOptions{Path: repo, Branch: "feat/y"}); err != nil {
		t.Fatal(err)
	}
	if currentBranch(git) != "feat/y" {
		t.Errorf("on %q after checkout", currentBranch(git))
	}
	git("checkout", "main")
	hash, err := s.GitDeleteBranch(ctx, api.GitBranchOptions{Path: repo, Name: "feat/y"})
	if err != nil || hash != strings.TrimSpace(git("rev-parse", "main")) {
		t.Fatalf("delete = %q, %v", hash, err)
	}
	if _, err := s.GitCreateBranch(ctx, api.GitCreateBranchOptions{Path: repo, Name: "feat/y", From: hash}); err != nil {
		t.Fatalf("undo by recreating at the hash: %v", err)
	}
}

func TestGitBranchOnlyCountsCommitsNothingElseReaches(t *testing.T) {
	repo, git := initRepo(t)
	s, ctx := new(Service), context.Background()
	git("checkout", "-b", "solo")
	commitTo(t, repo, git, "a.txt", "a", "one")
	commitTo(t, repo, git, "b.txt", "b", "two")
	git("checkout", "main")
	if n, err := s.GitBranchOnly(ctx, api.GitBranchOptions{Path: repo, Name: "solo"}); err != nil || n != 2 {
		t.Fatalf("only = %d, %v; want 2", n, err)
	}
	git("branch", "copy", "solo")
	if n, _ := s.GitBranchOnly(ctx, api.GitBranchOptions{Path: repo, Name: "solo"}); n != 0 {
		t.Errorf("only = %d once another branch holds the commits", n)
	}
}

func TestGitCheckoutTracksARemoteBranch(t *testing.T) {
	repo, git := initRepo(t)
	addLocalOrigin(t, git)
	git("push", "origin", "HEAD:refs/heads/feat/r")
	git("fetch", "origin")
	if _, err := new(Service).GitCheckout(context.Background(), api.GitCheckoutOptions{Path: repo, Branch: "origin/feat/r", Remote: true}); err != nil {
		t.Fatal(err)
	}
	if currentBranch(git) != "feat/r" {
		t.Errorf("on %q", currentBranch(git))
	}
	if up := strings.TrimSpace(git("rev-parse", "--abbrev-ref", "feat/r@{upstream}")); up != "origin/feat/r" {
		t.Errorf("upstream = %q", up)
	}
}

func TestGitBranchRefusesNamesGitWouldReadAsOptions(t *testing.T) {
	repo, _ := initRepo(t)
	s, ctx := new(Service), context.Background()
	if _, err := s.GitCheckout(ctx, api.GitCheckoutOptions{Path: repo, Branch: "--orphan"}); err == nil {
		t.Error("checkout took an option for a branch")
	}
	if _, err := s.GitCreateBranch(ctx, api.GitCreateBranchOptions{Path: repo, Name: "ok", From: "--all"}); err == nil {
		t.Error("create took an option for a start point")
	}
	if _, err := s.GitRenameBranch(ctx, api.GitRenameBranchOptions{Path: repo, Name: "main", NewName: "bad..name"}); err == nil {
		t.Error("rename took a name check-ref-format refuses")
	}
	if _, err := s.GitDeleteBranch(ctx, api.GitBranchOptions{Path: repo, Name: ""}); err == nil {
		t.Error("delete took an empty name")
	}
}

func TestGitRefsNamesTheWorktreeABranchIsIn(t *testing.T) {
	repo, git := initRepo(t)
	other := t.TempDir() + "/wt"
	git("worktree", "add", "-b", "elsewhere", other)
	refs, err := new(Service).GitRefs(context.Background(), api.PathOptions{Path: repo})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range refs.Refs {
		if r.Name == "elsewhere" && r.Worktree == "" {
			t.Error("a branch checked out in another worktree must say where")
		}
	}
}
