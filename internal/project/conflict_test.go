package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestParseHunkHeader(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line string
		want hunk
	}{
		{"@@ -3 +3 @@\n", hunk{api.GitLines{From: 2, To: 3}, api.GitLines{From: 2, To: 3}}},
		{"@@ -5,0 +6,2 @@ func x\n", hunk{api.GitLines{From: 5, To: 5}, api.GitLines{From: 5, To: 7}}},
		{"@@ -1,2 +0,0 @@\n", hunk{api.GitLines{From: 0, To: 2}, api.GitLines{From: 0, To: 0}}},
	}
	for _, tt := range tests {
		got, ok := parseHunkHeader(tt.line)
		if !ok || got != tt.want {
			t.Errorf("parseHunkHeader(%q) = %+v, %v; want %+v", tt.line, got, ok, tt.want)
		}
	}
	for _, bad := range []string{"+++ b/x\n", "@@ -a +1 @@\n", "@@ -1 +1\n"} {
		if _, ok := parseHunkHeader(bad); ok {
			t.Errorf("parseHunkHeader(%q) read a hunk", bad)
		}
	}
}

func h(bf, bt, sf, st int) hunk {
	return hunk{api.GitLines{From: bf, To: bt}, api.GitLines{From: sf, To: st}}
}

func chunk(kind string, base, ours, theirs [2]int) api.GitConflictChunk {
	return api.GitConflictChunk{Kind: kind,
		Base:   api.GitLines{From: base[0], To: base[1]},
		Ours:   api.GitLines{From: ours[0], To: ours[1]},
		Theirs: api.GitLines{From: theirs[0], To: theirs[1]}}
}

func TestDiff3Chunks(t *testing.T) {
	t.Parallel()
	// base: 10 lines. Ours inserts 2 lines after line 1 and edits line 8;
	// theirs edits line 5 and line 8 differently, and the same as ours at 10.
	ours := []hunk{h(1, 1, 1, 3), h(7, 8, 9, 10), h(9, 10, 11, 12)}
	theirs := []hunk{h(4, 5, 4, 5), h(7, 8, 7, 8), h(9, 10, 9, 10)}
	oursLines := make([]string, 12)
	theirsLines := make([]string, 10)
	oursLines[9], theirsLines[7] = "ours\n", "theirs\n"
	oursLines[11], theirsLines[9] = "same\n", "same\n"
	got := diff3Chunks(ours, theirs, oursLines, theirsLines)
	want := []api.GitConflictChunk{
		chunk(api.GitChunkOurs, [2]int{1, 1}, [2]int{1, 3}, [2]int{1, 1}),
		chunk(api.GitChunkTheirs, [2]int{4, 5}, [2]int{6, 7}, [2]int{4, 5}),
		chunk(api.GitChunkConflict, [2]int{7, 8}, [2]int{9, 10}, [2]int{7, 8}),
		chunk(api.GitChunkBoth, [2]int{9, 10}, [2]int{11, 12}, [2]int{9, 10}),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diff3Chunks =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDiff3ChunksJoinsOverlapsAcrossSides(t *testing.T) {
	t.Parallel()
	// Ours rewrites lines 2-4, theirs lines 4-6: one conflict over 2-6.
	ours := []hunk{h(2, 4, 2, 3)}
	theirs := []hunk{h(3, 6, 3, 7)}
	got := diff3Chunks(ours, theirs, make([]string, 10), make([]string, 12))
	want := []api.GitConflictChunk{chunk(api.GitChunkConflict, [2]int{2, 6}, [2]int{2, 5}, [2]int{2, 7})}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diff3Chunks = %+v, want %+v", got, want)
	}
}

func TestGitConflictReadsTheThreeVersions(t *testing.T) {
	setIdentity(t)
	repo, git := initRepo(t)
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a\nb\nc\nd\ne\n")
	git("add", "f.txt")
	git("commit", "-m", "base")
	git("switch", "-c", "side")
	write("a\nb\nTHEIRS\nd\ne\nend\n")
	git("commit", "-am", "side")
	git("switch", "main")
	write("start\na\nb\nOURS\nd\ne\n")
	git("commit", "-am", "main")
	if out, err := exec.Command("git", "-C", repo, "merge", "side").CombinedOutput(); err == nil {
		t.Fatalf("merge did not conflict: %s", out)
	}
	s := New(nil)
	got, err := s.GitConflict(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: "f.txt"})
	if err != nil {
		t.Fatalf("GitConflict: %v", err)
	}
	if got.Base != "a\nb\nc\nd\ne\n" || got.Ours != "start\na\nb\nOURS\nd\ne\n" || got.Theirs != "a\nb\nTHEIRS\nd\ne\nend\n" {
		t.Errorf("versions = %q / %q / %q", got.Base, got.Ours, got.Theirs)
	}
	want := []api.GitConflictChunk{
		chunk(api.GitChunkOurs, [2]int{0, 0}, [2]int{0, 1}, [2]int{0, 0}),
		chunk(api.GitChunkConflict, [2]int{2, 3}, [2]int{3, 4}, [2]int{2, 3}),
		chunk(api.GitChunkTheirs, [2]int{5, 5}, [2]int{6, 6}, [2]int{5, 6}),
	}
	if !reflect.DeepEqual(got.Chunks, want) {
		t.Errorf("chunks =\n%+v\nwant\n%+v", got.Chunks, want)
	}
	if _, err := s.GitConflict(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: "a.txt"}); err == nil {
		t.Error("read a conflict out of a file with none")
	}

	resolved := "start\na\nb\nOURS\nTHEIRS\nd\ne\nend\n"
	if _, err := s.GitResolveFile(t.Context(), api.GitResolveFileOptions{Path: repo, Rel: "f.txt", Content: resolved}); err != nil {
		t.Fatalf("GitResolveFile: %v", err)
	}
	if body, _ := os.ReadFile(filepath.Join(repo, "f.txt")); string(body) != resolved {
		t.Errorf("file = %q, want %q", body, resolved)
	}
	if st := s.diff(repo); st.Conflicted != nil {
		t.Errorf("Conflicted = %q after resolving, want none", st.Conflicted)
	}
	if _, err := s.GitResolveFile(t.Context(), api.GitResolveFileOptions{Path: repo, Rel: "f.txt", Content: "x"}); err == nil {
		t.Error("wrote over a file no longer in conflict")
	}
}

func TestGitConflictAddedOnBothSides(t *testing.T) {
	setIdentity(t)
	repo, git := initRepo(t)
	git("switch", "-c", "side")
	if err := os.WriteFile(filepath.Join(repo, "n.txt"), []byte("theirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "n.txt")
	git("commit", "-m", "side")
	git("switch", "main")
	if err := os.WriteFile(filepath.Join(repo, "n.txt"), []byte("ours\nmore\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "n.txt")
	git("commit", "-m", "main")
	if out, err := exec.Command("git", "-C", repo, "merge", "side").CombinedOutput(); err == nil {
		t.Fatalf("merge did not conflict: %s", out)
	}
	got, err := New(nil).GitConflict(t.Context(), api.CheckoutFileOptions{Path: repo, Rel: "n.txt"})
	if err != nil {
		t.Fatalf("GitConflict: %v", err)
	}
	want := []api.GitConflictChunk{chunk(api.GitChunkConflict, [2]int{0, 0}, [2]int{0, 2}, [2]int{0, 1})}
	if got.Base != "" || !reflect.DeepEqual(got.Chunks, want) {
		t.Errorf("base %q, chunks %+v; want no base and %+v", got.Base, got.Chunks, want)
	}
}
