package ompprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	conversation = "1a2b3c4d5e6f7a8b"
	imageHash    = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	gcdHash      = "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"
)

// sessionIn writes what omp 18.8.9 left for one conversation that ran a
// sub-agent and pasted an image: the session file, its artifacts directory with
// the sub-agent's own session, and the blob both refer to.
func sessionIn(t *testing.T, dir string) []string {
	t.Helper()
	bucket := filepath.Join("sessions", "--work--")
	rel := []string{
		filepath.Join(bucket, "1760000000_"+conversation+".jsonl"),
		filepath.Join(bucket, "1760000000_"+conversation, "agent-1.jsonl"),
		filepath.Join(bucket, "1760000000_"+conversation, "bash-1.txt"),
	}
	mustWrite(t, filepath.Join(dir, rel[0]), `{"type":"session","id":"`+conversation+`"}`+"\n"+`{"image":"blob:sha256:`+imageHash+`"}`+"\n")
	mustWrite(t, filepath.Join(dir, rel[1]), `{"image":"blob:sha256:`+gcdHash+`"}`+"\n")
	mustWrite(t, filepath.Join(dir, rel[2]), "out")
	mustWrite(t, filepath.Join(dir, "blobs", imageHash), "png")
	mustWrite(t, filepath.Join(dir, bucket, "1760000001_other.jsonl"), "{}")
	return rel
}

func TestMoveConversationRoundTrips(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	rel := sessionIn(t, personal)

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
	if _, err := os.Stat(filepath.Join(personal, "sessions", "--work--", "1760000001_other.jsonl")); err != nil {
		t.Errorf("another conversation moved too: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "blobs", imageHash)); err != nil {
		t.Errorf("the image did not follow the conversation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(personal, "blobs", imageHash)); err != nil {
		t.Errorf("the source lost a blob other sessions may still refer to: %v", err)
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
	sessionIn(t, personal)
	if err := MoveConversation(conversation, Personal); err == nil {
		t.Error("moved a conversation onto the profile it is in")
	}
	if err := MoveConversation(conversation, "gone"); err == nil {
		t.Error("moved a conversation into a profile that does not exist")
	}
	target := filepath.Join(p.Dir, "sessions", "--work--", "1760000000_"+conversation+".jsonl")
	mustWrite(t, target, "stale")
	if err := MoveConversation(conversation, p.Slug); err == nil {
		t.Error("moved over a session the target already holds")
	}
}
