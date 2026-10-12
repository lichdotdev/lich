// These drive a real PTY, Unix-only like the rest of the spawn tests.
//go:build !windows

package terminal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
)

// spawnLogBin is a stand-in Claude Code that writes the config dir it was given
// and its arguments to log, one spawn per line, and then waits like a session.
func spawnLogBin(t *testing.T, log string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-claude")
	script := fmt.Sprintf("#!/bin/sh\necho \"${CLAUDE_CONFIG_DIR:-personal}|${CLAUDE_CODE_OAUTH_TOKEN:-}|$*\" >> %q\nexec cat\n", log)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func spawnLines(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// TestContinueInRespawnsTheCardInTheTargetProfile: the running process ends,
// the conversation moves, and the same card comes back up resuming it with the
// target profile's directory and none of the token that would override it.
func TestContinueInRespawnsTheCardInTheTargetProfile(t *testing.T) {
	personal, work := profileMachine(t)
	conversationAt(t, personal, "c1")
	log := filepath.Join(t.TempDir(), "spawns")
	moved := map[string]string{}
	svc := New(stubBins{bin: spawnLogBin(t, log), providerSession: "c1", movedProfiles: moved},
		[]string{"CLAUDE_CODE_OAUTH_TOKEN=personal-token"}, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), providers.Claude, "c1", "", false, false, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 1 }, "the first spawn")
	if got := spawnLines(t, log)[0]; !strings.HasPrefix(got, "personal|personal-token|") {
		t.Errorf("Personal spawn %q, want lich's own environment", got)
	}
	if got := svc.ProfileSessions("work"); got != 0 {
		t.Errorf("work counts %d sessions before the move", got)
	}

	if err := svc.ContinueIn("s1", "work"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 2 }, "the respawn")
	got := spawnLines(t, log)[1]
	if !strings.HasPrefix(got, work+"||") || !strings.Contains(got, "--resume c1") {
		t.Errorf("respawn %q, want the work profile, no token, resuming c1", got)
	}
	if !svc.Live("s1") {
		t.Error("the card is not running after the move")
	}
	if got := svc.ProfileSessions("work"); got != 1 {
		t.Errorf("work counts %d sessions, want 1", got)
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
