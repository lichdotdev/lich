package terminal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/providers"
)

// profileMachine is a scratch home with Personal's ~/.claude and one profile,
// "work", made the way the Settings screen makes one. It returns Personal's
// directory and the profile's.
func profileMachine(t *testing.T) (personal, work string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("LICH_DEV", "")
	personal = filepath.Join(home, ".claude")
	if err := os.MkdirAll(personal, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := claudeprofile.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	return personal, p.Dir
}

// conversationAt files a transcript for conversation id under a Claude config
// dir, the way Claude Code lays one out.
func conversationAt(t *testing.T, dir, id string) string {
	t.Helper()
	path := filepath.Join(dir, "projects", "-work", id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":"user"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTranscriptsAreFoundInWhicheverProfileHoldsThem(t *testing.T) {
	_, work := profileMachine(t)
	path := conversationAt(t, work, "c1")
	sub := filepath.Join(work, "projects", "-work", "c1", "subagents", "agent-a.jsonl")
	if err := os.MkdirAll(filepath.Dir(sub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sub, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := claudeTranscriptPath("c1"); !ok || got != path {
		t.Errorf("claudeTranscriptPath = %q, %v, want %q", got, ok, path)
	}
	if got := claudeSubagentPaths("c1"); len(got) != 1 || got[0] != sub {
		t.Errorf("claudeSubagentPaths = %v, want [%s]", got, sub)
	}
	if !(&Service{}).ResumeAvailable(providers.Claude, "c1", "") {
		t.Error("a conversation in a profile is not offered for resume")
	}
}

func TestOutsideConversationsSayWhichProfileHoldsThem(t *testing.T) {
	personal, work := profileMachine(t)
	for dir, id := range map[string]string{personal: "p1", work: "w1"} {
		path := filepath.Join(dir, "projects", "-work", id+".jsonl")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"type":"user","cwd":"/work","message":{"content":"hello"}}` + "\n"
		if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]string{}
	for _, c := range claudeConversations() {
		got[c.ID] = c.Profile
	}
	if got["p1"] != claudeprofile.Personal || got["w1"] != "work" {
		t.Errorf("profiles by conversation %v, want p1 personal and w1 work", got)
	}
}
