package claudeprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const conversation = "9a36940c-26cb-44ba-b349-231939a7cbd5"

// conversationIn writes what Claude Code 2.1.296 left for one conversation that
// edited a file and ran a subagent, plus a hook's file named after it.
func conversationIn(t *testing.T, dir string) []string {
	t.Helper()
	rel := []string{
		filepath.Join("projects", "-work", conversation+".jsonl"),
		filepath.Join("projects", "-work", conversation+".jsonl.wakatime"),
		filepath.Join("projects", "-work", conversation, "subagents", "agent-a1.jsonl"),
		filepath.Join("file-history", conversation, "3cf026f42473a7c0@v1"),
		filepath.Join("session-env", conversation, "env"),
		filepath.Join("tasks", conversation, "1.json"),
	}
	for _, r := range rel {
		mustWrite(t, filepath.Join(dir, r), "x")
	}
	mustWrite(t, filepath.Join(dir, "projects", "-work", "other.jsonl"), "x")
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
	if _, err := os.Stat(filepath.Join(personal, "projects", "-work", "other.jsonl")); err != nil {
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
	conversationIn(t, personal)
	if err := MoveConversation(conversation, Personal); err == nil {
		t.Error("moved a conversation onto the profile it is in")
	}
	if err := MoveConversation(conversation, "gone"); err == nil {
		t.Error("moved a conversation into a profile that does not exist")
	}
	mustWrite(t, filepath.Join(p.Dir, "tasks", conversation, "stale"), "x")
	if err := MoveConversation(conversation, p.Slug); err == nil {
		t.Error("moved over state the target already holds")
	}
	if _, err := os.Stat(filepath.Join(personal, "projects", "-work", conversation+".jsonl")); err != nil {
		t.Errorf("a refused move still took the transcript: %v", err)
	}
}
