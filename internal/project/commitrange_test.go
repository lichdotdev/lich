package project

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestCommitRangeDiffLocal(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	commit := func(file, text string) string {
		write(t, repo, file, text)
		git("add", ".")
		git("commit", "-m", file)
		return strings.TrimSpace(git("rev-parse", "HEAD"))
	}
	base := commit("before.txt", "before\n")
	first := commit("first.txt", "first\n")
	last := commit("last.txt", "last\n")
	commit("after.txt", "after\n")

	gh := &fakeGH{err: errors.New("gh must not run")}
	got, err := withGH(gh).CommitRangeDiff(t.Context(), api.CommitRangeOptions{Path: repo, First: first, Last: last})
	if err != nil {
		t.Fatalf("CommitRangeDiff: %v", err)
	}
	if got.Base != base {
		t.Errorf("base = %s, want first's parent %s", got.Base, base)
	}
	for _, want := range []string{"b/first.txt", "b/last.txt"} {
		if !strings.Contains(got.Diff, want) {
			t.Errorf("diff misses %s:\n%s", want, got.Diff)
		}
	}
	for _, out := range []string{"before.txt", "after.txt"} {
		if strings.Contains(got.Diff, out) {
			t.Errorf("diff holds %s, outside the range:\n%s", out, got.Diff)
		}
	}
	if gh.calls != 0 {
		t.Errorf("gh ran %v", gh.history)
	}
}

func TestCommitRangeDiffAsksGitHub(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	first, last, parent := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	diff := "diff --git a/x b/x\n"

	gh := &fakeGH{outs: [][]byte{[]byte(parent + "\n"), []byte(diff)}}
	got, err := withGH(gh).CommitRangeDiff(t.Context(), api.CommitRangeOptions{Path: repo, First: first, Last: last})
	if err != nil {
		t.Fatalf("CommitRangeDiff: %v", err)
	}
	if got != (api.CommitRangeDiff{Base: parent, Diff: diff}) {
		t.Errorf("got %+v", got)
	}
	want := [][]string{
		{"api", "repos/{owner}/{repo}/commits/" + first, "--jq", ".parents[0].sha"},
		{"api", "-H", "Accept: application/vnd.github.diff", "repos/{owner}/{repo}/compare/" + parent + "..." + last},
	}
	if !slices.EqualFunc(gh.history, want, slices.Equal) {
		t.Errorf("gh calls = %v, want %v", gh.history, want)
	}
}

func TestCommitRangeDiffRefusals(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	oid := strings.Repeat("a", 40)
	for name, opts := range map[string]api.CommitRangeOptions{
		"option":     {Path: repo, First: "--output=x", Last: oid},
		"short":      {Path: repo, First: oid, Last: "abc"},
		"uppercased": {Path: repo, First: strings.ToUpper(oid), Last: oid},
	} {
		gh := &fakeGH{err: errors.New("gh must not run")}
		if _, err := withGH(gh).CommitRangeDiff(t.Context(), opts); err == nil || gh.calls != 0 {
			t.Errorf("%s: err = %v after %d gh calls, want a refusal before any", name, err, gh.calls)
		}
	}
	if _, err := withGH(&fakeGH{out: []byte("null\n")}).CommitRangeDiff(t.Context(), api.CommitRangeOptions{Path: repo, First: oid, Last: oid}); err == nil {
		t.Error("an unreadable parent: want an error, got nil")
	}

	git("remote", "add", "origin", "https://gitlab.com/group/project.git")
	gh := &fakeGH{err: errors.New("gh must not run")}
	if _, err := withGH(gh).CommitRangeDiff(t.Context(), api.CommitRangeOptions{Path: repo, First: oid, Last: oid}); !errors.Is(err, errRangeNotLocalGitLab) {
		t.Errorf("GitLab, commits not here: err = %v, want %v", err, errRangeNotLocalGitLab)
	}
}
