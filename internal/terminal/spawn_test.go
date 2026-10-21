// The suite spawns real PTYs and reuses stubBins from terminal_test.go, which
// is Unix-tagged for the same reason; this file carries the tag so the package
// still builds on Windows.
//go:build !windows

package terminal

import (
	"os"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/events"
)

// A spawn that cannot start is reported as itself: the session id is in the
// error, because the window surfaces it and "no such file" alone names
// neither the session nor what lich was trying to run.
func TestStartReportsAPtyThatCannotSpawn(t *testing.T) {
	t.Parallel()
	svc := New(stubBins{bin: "/nonexistent/lich-test-provider"}, nil, events.New())

	err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, 80, 24)

	if err == nil {
		t.Fatal("Start = nil, want the spawn failure")
	}
	if !strings.Contains(err.Error(), `failed to start pty for "s1"`) {
		t.Errorf("Start = %v, want it to name the session it could not spawn", err)
	}
	if svc.Live("s1") {
		t.Error("a session that never spawned was registered as live")
	}
}

// A session opened with no directory of its own starts at home, and says so:
// the effective directory is what the card shows and what the cwd poller
// degrades back to.
func TestASpawnWithNoDirectoryStartsAtHome(t *testing.T) {
	svc := New(stubBins{bin: stayAliveBin(t)}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	_, cwd, err := svc.spawnSession("s1", "p1", "", "claude", "", "", "", false, 80, 24)
	if err != nil {
		t.Fatalf("spawnSession = %v, want nil", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory to fall back to: %v", err)
	}
	if cwd != home {
		t.Errorf("start directory = %q, want the home directory %q", cwd, home)
	}
}
