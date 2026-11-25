package profilestore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// countingLocate finds a conversation at <dir>/convos/<id>.jsonl and counts
// every directory it is asked to search.
func countingLocate(calls *int) func(dir, id string) (string, bool) {
	return func(dir, id string) (string, bool) {
		*calls++
		path := filepath.Join(dir, "convos", id+".jsonl")
		if _, err := os.Stat(path); err != nil {
			return "", false
		}
		return path, true
	}
}

// conversationID is unique to the test: located is process-wide, and a shared
// id would let one test read another's answer.
func conversationID(t *testing.T) string {
	return strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
}

func TestOwnerSearchesOnceForAConversationItFound(t *testing.T) {
	s := toy(machine(t))
	work, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	id := conversationID(t)
	mustWrite(t, filepath.Join(work.Dir, "convos", id+".jsonl"), "{}")

	var calls int
	locate := countingLocate(&calls)
	for range 3 {
		p, path, ok := s.Owner(id, locate)
		if !ok || p.Slug != work.Slug || path != filepath.Join(work.Dir, "convos", id+".jsonl") {
			t.Fatalf("Owner = %q, %q, %v, want the Work profile's transcript", p.Slug, path, ok)
		}
	}
	// Personal is searched before Work, so the first call costs two searches;
	// every later one is answered from what that call found.
	if calls != 2 {
		t.Errorf("locate ran %d times over three calls, want 2", calls)
	}
}

func TestOwnerFollowsAConversationThatMovedProfile(t *testing.T) {
	personal := machine(t)
	s := toy(personal)
	work, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	id := conversationID(t)
	from := filepath.Join(personal, "convos", id+".jsonl")
	mustWrite(t, from, "{}")
	var calls int
	locate := countingLocate(&calls)
	if p, _, ok := s.Owner(id, locate); !ok || p.Slug != Personal {
		t.Fatalf("Owner = %q, %v before the move, want Personal", p.Slug, ok)
	}

	to := filepath.Join(work.Dir, "convos", id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	if p, path, ok := s.Owner(id, locate); !ok || p.Slug != work.Slug || path != to {
		t.Errorf("Owner = %q, %q, %v after the move, want Work at %s", p.Slug, path, ok, to)
	}
}

func TestOwnerFindsATranscriptWrittenAfterAMiss(t *testing.T) {
	personal := machine(t)
	s := toy(personal)
	id := conversationID(t)
	var calls int
	locate := countingLocate(&calls)
	if _, _, ok := s.Owner(id, locate); ok {
		t.Fatal("Owner found a conversation that has no transcript yet")
	}
	mustWrite(t, filepath.Join(personal, "convos", id+".jsonl"), "{}")
	if _, _, ok := s.Owner(id, locate); !ok {
		t.Error("Owner kept answering a miss after the transcript was written")
	}
}
