// These drive a real PTY, Unix-only like the rest of the spawn tests.
//go:build !windows

package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
