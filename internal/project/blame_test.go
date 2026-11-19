package project

import (
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestParseBlameMapsEachLineToItsCommit(t *testing.T) {
	a, b := "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"
	out := a + " 1 1 2\n" +
		"author Ada\nauthor-mail <ada@example.com>\nauthor-time 1700000000\nauthor-tz +0000\nsummary first\nfilename f.go\n\tone\n" +
		a + " 2 2\n\ttwo\n" +
		uncommittedHash + " 3 3 1\nauthor Not Committed Yet\nauthor-time 1800000000\nsummary Version of f.go from f.go\nfilename f.go\n\tmine\n" +
		b + " 3 4 1\nauthor Bob\nauthor-time 1710000000\nsummary second\nprevious " + a + " f.go\nfilename f.go\n\tfour\n"
	got := parseBlame(out)
	if want := []string{a, a, "", b}; !slices.Equal(got.Lines, want) {
		t.Fatalf("lines %v, want %v", got.Lines, want)
	}
	if c := got.Commits[a]; c != (api.GitBlameCommit{Author: "Ada", Time: 1700000000, Summary: "first", Message: "first"}) {
		t.Errorf("commit a = %+v", c)
	}
	if _, ok := got.Commits[uncommittedHash]; ok || len(got.Commits) != 2 {
		t.Errorf("commits %v, want a and b only: an uncommitted line names no commit", got.Commits)
	}
}

func TestGitBlameReadsTheWorkingTree(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	write(t, repo, "f.go", "one\ntwo\nthree\n")
	git("add", "f.go")
	git("commit", "-q", "--author=Ada <ada@example.com>", "-m", "feat: three lines\n\nWhy they are three.")
	head := strings.TrimSpace(git("rev-parse", "HEAD"))
	write(t, repo, "f.go", "one\nTWO\nthree\nfour\n")
	got, err := New(nil).GitBlame(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: "f.go"})
	if err != nil {
		t.Fatalf("GitBlame: %v", err)
	}
	if want := []string{head, "", head, ""}; !slices.Equal(got.Lines, want) {
		t.Fatalf("lines %v, want %v", got.Lines, want)
	}
	c := got.Commits[head]
	if c.Author != "Ada" || c.Summary != "feat: three lines" || c.Message != "feat: three lines\n\nWhy they are three." || c.Time == 0 {
		t.Errorf("commit %+v", c)
	}
}

func TestGitBlameOfAFileGitDoesNotTrackIsAllUncommitted(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	write(t, repo, "new.go", "x\n")
	got, err := New(nil).GitBlame(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: "new.go"})
	if err != nil || len(got.Lines) != 0 {
		t.Fatalf("got %+v, %v; want no committed line", got, err)
	}
}

func TestGitBlameRefusesWhatIsNoFileOfARepository(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	if _, err := New(nil).GitBlame(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: "../x"}); err == nil {
		t.Error("a path out of the checkout: want an error")
	}
	if _, err := New(nil).GitBlame(t.Context(), api.CheckoutFileOptions{Path: t.TempDir(), Rel: "f.go"}); err == nil {
		t.Error("a plain folder: want an error")
	}
}
