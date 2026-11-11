package project

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func commitTo(t *testing.T, repo string, git func(...string) string, name, body, message string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", message)
	return strings.TrimSpace(git("rev-parse", "HEAD"))
}

func subjects(p api.GitLogPage) []string {
	out := []string{}
	for _, c := range p.Commits {
		out = append(out, c.Subject)
	}
	return out
}

func TestGitLogListsEveryBranchWithParentsAfterChildren(t *testing.T) {
	repo, git := initRepo(t)
	base := commitTo(t, repo, git, "a.txt", "a", "on main")
	git("checkout", "-b", "feat/x")
	tip := commitTo(t, repo, git, "b.txt", "b", "on feat")
	git("checkout", "main")

	page, err := new(Service).GitLog(context.Background(), api.GitLogOptions{Path: repo, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range page.Commits {
		for _, p := range c.Parents {
			if seen[p] {
				t.Errorf("parent %s listed before its child %s", p, c.Hash)
			}
		}
		seen[c.Hash] = true
	}
	if !seen[tip] || !seen[base] {
		t.Fatalf("the log must hold both branches: %v", subjects(page))
	}
	if page.More {
		t.Error("More reported on the whole history")
	}
}

func TestGitLogDecoratesRefsByKind(t *testing.T) {
	repo, git := initRepo(t)
	git("tag", "v1")
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	page, err := new(Service).GitLog(context.Background(), api.GitLogOptions{Path: repo, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	c := page.Commits[0]
	if !c.Head {
		t.Error("the tip of the checked out branch is HEAD")
	}
	want := map[string]string{"main": api.GitRefLocal, "v1": api.GitRefTag, "origin/main": api.GitRefRemote}
	for _, r := range c.Refs {
		if want[r.Name] != r.Kind {
			t.Errorf("ref %q kind %q, want %q", r.Name, r.Kind, want[r.Name])
		}
		if r.Head != (r.Name == "main") {
			t.Errorf("ref %q head = %v", r.Name, r.Head)
		}
	}
	if len(c.Refs) != len(want) {
		t.Errorf("refs = %+v", c.Refs)
	}
}

func TestGitLogPagesWithoutRepeating(t *testing.T) {
	repo, git := initRepo(t)
	for _, m := range []string{"one", "two", "three"} {
		commitTo(t, repo, git, "f.txt", m, m)
	}
	s := new(Service)
	first, err := s.GitLog(context.Background(), api.GitLogOptions{Path: repo, Limit: 2})
	if err != nil || !first.More || len(first.Commits) != 2 {
		t.Fatalf("first page %+v, %v", first, err)
	}
	second, err := s.GitLog(context.Background(), api.GitLogOptions{Path: repo, Skip: 2, Limit: 2})
	if err != nil || second.More || len(second.Commits) != 2 {
		t.Fatalf("second page %+v, %v", second, err)
	}
	if got := slices.Concat(subjects(first), subjects(second)); !slices.Equal(got, []string{"three", "two", "one", "init"}) {
		t.Errorf("pages = %v", got)
	}
}

func TestGitLogFilters(t *testing.T) {
	repo, git := initRepo(t)
	hash := commitTo(t, repo, git, "src/a.txt", "a", "Fix the [parser]")
	commitTo(t, repo, git, "b.txt", "b", "unrelated")
	s := new(Service)
	cases := []struct {
		name string
		opts api.GitLogOptions
		want []string
	}{
		{"text is literal and ignores case", api.GitLogOptions{Query: "fix the [PARSER]"}, []string{"Fix the [parser]"}},
		{"hash finds the commit", api.GitLogOptions{Query: hash[:8]}, []string{"Fix the [parser]"}},
		{"file", api.GitLogOptions{File: "src/a.txt"}, []string{"Fix the [parser]"}},
		{"author nobody", api.GitLogOptions{Author: "no such person"}, []string{}},
		{"since the far future", api.GitLogOptions{Since: 4102444800}, []string{}},
		{"a hex word naming no commit is text", api.GitLogOptions{Query: "abcd1234"}, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.opts.Path, c.opts.Limit = repo, 50
			page, err := s.GitLog(context.Background(), c.opts)
			if err != nil {
				t.Fatal(err)
			}
			if got := subjects(page); !slices.Equal(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestGitLogRefOptionIsNotAFlag(t *testing.T) {
	repo, _ := initRepo(t)
	if _, err := new(Service).GitLog(context.Background(), api.GitLogOptions{Path: repo, Ref: "--output=/tmp/x", Limit: 1}); err == nil {
		t.Fatal("a ref spelled like a flag must fail as a bad revision")
	}
}

func TestGitLogIgnoresShowSignatureConfig(t *testing.T) {
	repo, git := initRepo(t)
	git("config", "log.showSignature", "true")
	if _, err := new(Service).GitLog(context.Background(), api.GitLogOptions{Path: repo, Limit: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestGitLogOfARepositoryWithoutCommits(t *testing.T) {
	repo, git := initRepo(t)
	git("checkout", "--orphan", "fresh")
	git("update-ref", "-d", "refs/heads/main")
	page, err := new(Service).GitLog(context.Background(), api.GitLogOptions{Path: repo, Limit: 10})
	if err != nil || len(page.Commits) != 0 {
		t.Fatalf("page %+v, %v", page, err)
	}
}

func TestGitRefsMarksHeadAndMovesItsStamp(t *testing.T) {
	repo, git := initRepo(t)
	git("branch", "feat/x")
	git("update-ref", "refs/remotes/origin/HEAD", "HEAD")
	s := new(Service)
	before, err := s.GitRefs(context.Background(), api.PathOptions{Path: repo})
	if err != nil {
		t.Fatal(err)
	}
	head := slices.IndexFunc(before.Refs, func(r api.GitRef) bool { return r.Head })
	if head < 0 || before.Refs[head].Name != "main" {
		t.Fatalf("head not on main: %+v", before.Refs)
	}
	if slices.ContainsFunc(before.Refs, func(r api.GitRef) bool { return strings.HasSuffix(r.Name, "/HEAD") }) {
		t.Errorf("a remote's HEAD is no branch: %+v", before.Refs)
	}
	same, _ := s.GitRefs(context.Background(), api.PathOptions{Path: repo})
	if same.Stamp != before.Stamp {
		t.Error("the stamp moved with nothing changed")
	}
	commitTo(t, repo, git, "a.txt", "a", "move")
	after, _ := s.GitRefs(context.Background(), api.PathOptions{Path: repo})
	if after.Stamp == before.Stamp {
		t.Error("the stamp did not move with a commit")
	}
}

func TestGitCommitReadsFilesAndRename(t *testing.T) {
	repo, git := initRepo(t)
	commitTo(t, repo, git, "old.txt", "one\ntwo\nthree\nfour\nfive\n", "add")
	git("mv", "old.txt", "new.txt")
	hash := commitTo(t, repo, git, "bin.dat", "\x00\x01", "Rename it\n\nThe body.")
	d, err := new(Service).GitCommit(context.Background(), api.GitCommitOptions{Path: repo, Hash: hash})
	if err != nil {
		t.Fatal(err)
	}
	if d.Hash != hash || d.Message != "Rename it\n\nThe body." {
		t.Errorf("detail %+v", d)
	}
	byPath := map[string]api.GitCommitFile{}
	for _, f := range d.Files {
		byPath[f.Path] = f
	}
	if f := byPath["new.txt"]; f.OldPath != "old.txt" {
		t.Errorf("rename not read: %+v", d.Files)
	}
	if f := byPath["bin.dat"]; !f.Binary {
		t.Errorf("binary not read: %+v", d.Files)
	}
}

func TestGitCommitShowsAMergeAgainstItsFirstParent(t *testing.T) {
	repo, git := initRepo(t)
	git("checkout", "-b", "side")
	commitTo(t, repo, git, "side.txt", "s", "side")
	git("checkout", "main")
	commitTo(t, repo, git, "main.txt", "m", "main")
	git("merge", "--no-ff", "-m", "merge", "side")
	hash := strings.TrimSpace(git("rev-parse", "HEAD"))
	d, err := new(Service).GitCommit(context.Background(), api.GitCommitOptions{Path: repo, Hash: hash})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Files) != 1 || d.Files[0].Path != "side.txt" {
		t.Errorf("merge files = %+v", d.Files)
	}
}

func TestGitCommitRefusesWhatIsNotAHash(t *testing.T) {
	repo, _ := initRepo(t)
	if _, err := new(Service).GitCommit(context.Background(), api.GitCommitOptions{Path: repo, Hash: "--output=x"}); err == nil {
		t.Error("GitCommit took a flag for a hash")
	}
	if _, err := new(Service).GitCommitDiff(context.Background(), api.GitCommitDiffOptions{Path: repo, Hash: "HEAD", File: "x"}); err == nil {
		t.Error("GitCommitDiff took a ref for a hash")
	}
}

func TestGitCommitDiffFollowsARename(t *testing.T) {
	repo, git := initRepo(t)
	commitTo(t, repo, git, "old.txt", "one\ntwo\nthree\nfour\nfive\n", "add")
	git("mv", "old.txt", "new.txt")
	hash := commitTo(t, repo, git, "new.txt", "one\ntwo\nthree\nfour\nsix\n", "rename")
	s := new(Service)
	diff, err := s.GitCommitDiff(context.Background(), api.GitCommitDiffOptions{Path: repo, Hash: hash, File: "new.txt", OldFile: "old.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "rename from old.txt") || !strings.Contains(diff, "+six") {
		t.Errorf("diff = %s", diff)
	}
}

func TestParseGitLogRejectsATornRecord(t *testing.T) {
	if _, err := parseGitLog("hash\x00parents\x00"); err == nil {
		t.Error("a record with too few fields must fail")
	}
}
