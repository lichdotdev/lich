package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseNumstat(t *testing.T) {
	t.Parallel()
	out := "3\t1\tsrc/a.go\x00" +
		"-\t-\timg.png\x00" +
		"0\t0\t\x00old name.txt\x00new name.txt\x00" +
		"2\t0\twith space.txt\x00"
	got := map[string]FileLines{}
	parseNumstat(out, got)
	want := map[string]FileLines{
		"src/a.go":       {Added: 3, Removed: 1},
		"img.png":        {Binary: true},
		"new name.txt":   {},
		"with space.txt": {Added: 2},
	}
	if len(got) != len(want) {
		t.Fatalf("parseNumstat = %+v, want %+v", got, want)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %+v, want %+v", name, got[name], w)
		}
	}
	parseNumstat("0\t0\t\x00only\x00", got)
}

func TestReadLinesAgainstRealGit(t *testing.T) {
	t.Parallel()
	repo, git := initRepo(t)
	ctx := context.Background()

	clean, ok := ReadLines(ctx, repo)
	if !ok || len(clean.Files) != 0 || len(clean.Head) < 7 {
		t.Fatalf("ReadLines(clean) = %+v, %v, want a head and no files", clean, ok)
	}

	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("kept.txt", "one\ntwo\n")
	git("add", "kept.txt")
	git("commit", "-m", "kept")
	write("kept.txt", "one\nthree\nfour\n")
	write("fresh name.txt", "a\nb\nc\n")
	write("blob.bin", "a\x00b")

	got, ok := ReadLines(ctx, repo)
	if head := strings.TrimSpace(git("rev-parse", "HEAD")); !ok || got.Head != head {
		t.Fatalf("ReadLines(dirty) = %+v, %v, want head %s", got, ok, head)
	}
	if want := (FileLines{Added: 2, Removed: 1}); got.Files["kept.txt"] != want {
		t.Errorf("kept.txt = %+v, want %+v", got.Files["kept.txt"], want)
	}
	if want := (FileLines{Added: 3}); got.Files["fresh name.txt"] != want {
		t.Errorf("an untracked file = %+v, want %+v", got.Files["fresh name.txt"], want)
	}
}

func TestReadLinesWhereGitCannotAnswer(t *testing.T) {
	t.Parallel()
	if _, ok := ReadLines(context.Background(), t.TempDir()); ok {
		t.Error("ReadLines(not a repository) reported an answer")
	}
	repo, _ := initRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, ok := ReadLines(ctx, repo); ok {
		t.Error("ReadLines under a done context reported an answer")
	}
}
