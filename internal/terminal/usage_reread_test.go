package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func rereadLine(id string, read int) string {
	return fmt.Sprintf(`{"type":"assistant","message":{"id":%q,"model":"claude-opus-4-5","usage":{"input_tokens":1,"cache_read_input_tokens":%d}}}`+"\n", id, read)
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func TestRereadSumsEachResponseOnceAndResumes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	// m1 is written over two lines, each repeating its usage.
	appendTo(t, path, rereadLine("m1", 100)+rereadLine("m1", 100)+rereadLine("m2", 50)+`{"type":"user"}`+"\n")
	entry := scanCacheRead(path, rereadEntry{})
	if entry.total != 150 {
		t.Fatalf("total %d, want 150", entry.total)
	}
	appendTo(t, path, rereadLine("m3", 25)+`{"type":"assistant","message":{"id":"partial`)
	entry = scanCacheRead(path, entry)
	if entry.total != 175 {
		t.Fatalf("after append %d, want 175: the half-written line waits for its newline", entry.total)
	}
}

func TestRereadRestartsOnATruncatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	appendTo(t, path, rereadLine("m1", 100)+rereadLine("m2", 100))
	entry := scanCacheRead(path, rereadEntry{})
	if err := os.WriteFile(path, []byte(rereadLine("n1", 7)), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := scanCacheRead(path, entry).total; got != 7 {
		t.Fatalf("total %d, want 7 from the replaced file", got)
	}
}

func TestRereadKeepsTheEntryWhenTheFileIsGone(t *testing.T) {
	want := rereadEntry{offset: 10, total: 5}
	if got := scanCacheRead(filepath.Join(t.TempDir(), "none.jsonl"), want); got != want {
		t.Fatalf("entry %+v, want it untouched", got)
	}
}

func TestRereadCountsSumFilesPerSessionAndForget(t *testing.T) {
	dir := t.TempDir()
	main, sub := filepath.Join(dir, "a.jsonl"), filepath.Join(dir, "b.jsonl")
	appendTo(t, main, rereadLine("m1", 100))
	appendTo(t, sub, rereadLine("s1", 40))
	var r rereadCounts
	if got := r.total("s", []string{main, sub}); got != 140 {
		t.Fatalf("total %d, want 140", got)
	}
	appendTo(t, main, rereadLine("m2", 10))
	if got := r.total("s", []string{main, sub}); got != 150 {
		t.Fatalf("after append %d, want 150", got)
	}
	r.forget("s")
	if len(r.byID) != 0 {
		t.Fatalf("forget left %v", r.byID)
	}
}
