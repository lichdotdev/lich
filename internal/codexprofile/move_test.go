package codexprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const conversation = "01a132f5-f11f-7c70-8cd9-a0e57b649b37"

// conversationIn writes what Codex 0.162.1 left for one conversation that
// generated an image, plus a second conversation beside it.
func conversationIn(t *testing.T, dir string) []string {
	t.Helper()
	rel := []string{
		filepath.Join("sessions", "2026", "10", "12", "rollout-2026-10-12T22-52-21-"+conversation+".jsonl"),
		filepath.Join("generated_images", conversation, "ig_1.png"),
	}
	for _, r := range rel {
		mustWrite(t, filepath.Join(dir, r), "x")
	}
	mustWrite(t, filepath.Join(dir, "sessions", "2026", "10", "12", "rollout-2026-10-12T23-00-00-other.jsonl"), "x")
	return rel
}

func TestMoveConversationRoundTrips(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	rel := conversationIn(t, personal)

	if owner, _, ok := Owner(conversation); !ok || !owner.Personal {
		t.Fatalf("owner before the move: %+v %v", owner, ok)
	}
	if err := MoveConversation(conversation, p.Slug); err != nil {
		t.Fatal(err)
	}
	for _, r := range rel {
		if _, err := os.Stat(filepath.Join(p.Dir, r)); err != nil {
			t.Errorf("%s did not arrive: %v", r, err)
		}
		if _, err := os.Stat(filepath.Join(personal, r)); !os.IsNotExist(err) {
			t.Errorf("%s stayed behind in Personal", r)
		}
	}
	if _, err := os.Stat(filepath.Join(personal, "sessions", "2026", "10", "12", "rollout-2026-10-12T23-00-00-other.jsonl")); err != nil {
		t.Errorf("another conversation moved too: %v", err)
	}
	owner, path, ok := Owner(conversation)
	if !ok || owner.Slug != p.Slug || !strings.HasPrefix(path, p.Dir) {
		t.Fatalf("owner after the move: %+v %q %v", owner, path, ok)
	}

	if err := MoveConversation(conversation, Personal); err != nil {
		t.Fatal(err)
	}
	if owner, _, _ := Owner(conversation); !owner.Personal {
		t.Errorf("moving back left it in %q", owner.Slug)
	}
}

func TestMoveConversationRefusesWhatItCannotDo(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := MoveConversation(conversation, p.Slug); err == nil {
		t.Error("moved a conversation no profile holds")
	}
	rel := conversationIn(t, personal)
	if err := MoveConversation(conversation, Personal); err == nil {
		t.Error("moved a conversation onto the profile it is in")
	}
	if err := MoveConversation(conversation, "gone"); err == nil {
		t.Error("moved a conversation into a profile that does not exist")
	}
	mustWrite(t, filepath.Join(p.Dir, "generated_images", conversation, "stale"), "x")
	if err := MoveConversation(conversation, p.Slug); err == nil {
		t.Error("moved over state the target already holds")
	}
	if _, err := os.Stat(filepath.Join(personal, rel[0])); err != nil {
		t.Errorf("a refused move still took the rollout: %v", err)
	}
}

func TestMoveCopiesAcrossDirectoriesItCannotRename(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a")
	mustWrite(t, filepath.Join(src, "sub", "f"), "data")
	dst := filepath.Join(t.TempDir(), "b")
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "sub", "f")); err != nil || string(data) != "data" {
		t.Errorf("copied %q, %v", data, err)
	}
}
