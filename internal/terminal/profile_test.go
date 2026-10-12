package terminal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/events"
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

func TestClaudeProfileDirFollowsTheRowAndTheDisk(t *testing.T) {
	_, work := profileMachine(t)
	moved := map[string]string{}
	svc := New(stubBins{profile: "work", movedProfiles: moved}, nil, events.New())

	if dir, err := svc.claudeProfileDir("s1", providers.Codex, ""); err != nil || dir != "" {
		t.Errorf("codex: %q, %v", dir, err)
	}
	if dir, err := svc.claudeProfileDir("s1", providers.Claude, ""); err != nil || dir != work {
		t.Errorf("new conversation: %q, %v, want %q", dir, err, work)
	}

	conversationAt(t, mustPersonal(t), "c1")
	dir, err := svc.claudeProfileDir("s1", providers.Claude, "c1")
	if err != nil || dir != "" {
		t.Errorf("resume of a Personal conversation: %q, %v, want Personal", dir, err)
	}
	if moved["s1"] != "" {
		t.Errorf("row moved to %q, want Personal", moved["s1"])
	}
	if _, ok := moved["s1"]; !ok {
		t.Error("the row was not moved to the profile holding the conversation")
	}
}

func mustPersonal(t *testing.T) string {
	t.Helper()
	dir, err := claudeprofile.PersonalDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestASessionBornInADeletedProfileSaysSo(t *testing.T) {
	profileMachine(t)
	if err := claudeprofile.Delete("work"); err != nil {
		t.Fatal(err)
	}
	svc := New(stubBins{profile: "work"}, nil, events.New())
	if _, err := svc.claudeProfileDir("s1", providers.Claude, ""); !errors.Is(err, claudeprofile.ErrProfileGone) {
		t.Errorf("spawn into a deleted profile: %v, want ErrProfileGone", err)
	}
	err := svc.Start("s1", "p1", t.TempDir(), providers.Claude, "", "", false, false, 80, 24)
	if !errors.Is(err, claudeprofile.ErrProfileGone) {
		t.Errorf("Start: %v, want ErrProfileGone", err)
	}
	if svc.Live("s1") {
		t.Error("a session spawned anyway")
	}
}

func TestAnUnreadableProfileRefusesTheSpawn(t *testing.T) {
	profileMachine(t)
	svc := New(stubBins{profileErr: errors.New("disk gone")}, nil, events.New())
	if _, err := svc.claudeProfileDir("s1", providers.Claude, ""); err == nil {
		t.Error("an unreadable row spawned into Personal")
	}
}

func TestContinueInMovesAParkedConversation(t *testing.T) {
	personal, work := profileMachine(t)
	conversationAt(t, personal, "c1")
	moved := map[string]string{}
	svc := New(stubBins{providerSession: "c1", movedProfiles: moved}, nil, events.New())

	if err := svc.ContinueIn("s1", "work"); err != nil {
		t.Fatal(err)
	}
	if moved["s1"] != "work" {
		t.Errorf("row profile %q, want work", moved["s1"])
	}
	if _, err := os.Stat(filepath.Join(work, "projects", "-work", "c1.jsonl")); err != nil {
		t.Errorf("transcript did not move: %v", err)
	}
	if err := svc.ContinueIn("s1", claudeprofile.Personal); err != nil {
		t.Fatal(err)
	}
	if moved["s1"] != "" {
		t.Errorf("moving back left the row on %q", moved["s1"])
	}
	if svc.Live("s1") {
		t.Error("a parked session was spawned by the move")
	}
}

func TestContinueInRefusesWhatItCannotCarry(t *testing.T) {
	personal, _ := profileMachine(t)
	conversationAt(t, personal, "c1")

	empty := New(stubBins{}, nil, events.New())
	if err := empty.ContinueIn("s1", "work"); err == nil {
		t.Error("moved a session with no conversation")
	}

	svc := New(stubBins{providerSession: "c1"}, nil, events.New())
	svc.turns.report("s1", statusBusy)
	if err := svc.ContinueIn("s1", "work"); err == nil {
		t.Error("moved a session mid-turn")
	}
	svc.turns.report("s1", statusDone)
	if err := svc.ContinueIn("s1", "gone"); err == nil {
		t.Error("moved into a profile that does not exist")
	}
	if _, err := os.Stat(filepath.Join(personal, "projects", "-work", "c1.jsonl")); err != nil {
		t.Errorf("a refused move took the transcript: %v", err)
	}
	if err := svc.ContinueIn("s1", "work"); err != nil {
		t.Errorf("a turn a limit ended, or any finished one, must move: %v", err)
	}
}
