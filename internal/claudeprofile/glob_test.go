package claudeprofile

import (
	"os"
	"path/filepath"
	"testing"
)

// A home or config dir may hold glob metacharacters ("[work]", say): they are
// part of the path, never a pattern.
func TestTranscriptInTakesTheDirectoryLiterally(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "me [work]")
	transcript := filepath.Join(dir, "projects", "-repo", "c1.jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := transcriptIn(dir, "c1"); !ok || got != transcript {
		t.Errorf("transcriptIn = %q, %v; want %q", got, ok, transcript)
	}
}
