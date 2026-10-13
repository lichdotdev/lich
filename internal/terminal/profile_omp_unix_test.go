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
	"github.com/omartelo/lich/internal/ompprofile"
	"github.com/omartelo/lich/internal/providers"
)

// ompMachine is profileMachine for oh-my-pi: Personal's agent directory exists
// under the scratch home and nothing in lich's environment moves it.
func ompMachine(t *testing.T) (home, personal string) {
	t.Helper()
	profileMachine(t)
	home = os.Getenv("HOME")
	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	personal = filepath.Join(home, ".omp", "agent")
	if err := os.MkdirAll(personal, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, personal
}

// ompSessionAt files a session for conversation id under an omp agent dir, the
// way omp lays one out (omp 18.8.9).
func ompSessionAt(t *testing.T, dir, id string) string {
	t.Helper()
	path := filepath.Join(dir, "sessions", "--work--", "1760000000_"+id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	header := fmt.Sprintf(`{"type":"session","id":%q,"cwd":"/work"}`+"\n", id)
	if err := os.WriteFile(path, []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestOMPSessionsResolveInOMPProfiles: an oh-my-pi row names an omp profile,
// which has its own set of slugs, and a resume follows the session file into
// whichever profile holds it.
func TestOMPSessionsResolveInOMPProfiles(t *testing.T) {
	_, personal := ompMachine(t)
	ow, err := ompprofile.Create("Omp Work")
	if err != nil {
		t.Fatal(err)
	}
	moved := map[string]string{}
	svc := New(stubBins{profile: ow.Slug, movedProfiles: moved}, nil, events.New())

	if dir, err := svc.profileDir("s1", providers.OMP, ""); err != nil || dir != ow.Dir {
		t.Errorf("new conversation: %q, %v, want %q", dir, err, ow.Dir)
	}

	ompSessionAt(t, personal, "c9")
	dir, err := svc.profileDir("s1", providers.OMP, "c9")
	if err != nil || dir != "" {
		t.Errorf("resume of a Personal session: %q, %v, want Personal", dir, err)
	}
	if got, ok := moved["s1"]; !ok || got != "" {
		t.Errorf("row moved to %q (set %v), want Personal", got, ok)
	}
}

// TestContinueInMovesAnOMPSession: the move picks omp's profiles from the row
// when nothing is running.
func TestContinueInMovesAnOMPSession(t *testing.T) {
	_, personal := ompMachine(t)
	ow, err := ompprofile.Create("Omp Work")
	if err != nil {
		t.Fatal(err)
	}
	session := ompSessionAt(t, personal, "c9")
	moved := map[string]string{}
	svc := New(stubBins{providerSession: "c9", movedProfiles: moved, kinds: map[string]string{"s1": providers.OMP}}, nil, events.New())

	if err := svc.ContinueIn("s1", ow.Slug); err != nil {
		t.Fatal(err)
	}
	if moved["s1"] != ow.Slug {
		t.Errorf("row profile %q, want %q", moved["s1"], ow.Slug)
	}
	if _, err := os.Stat(filepath.Join(ow.Dir, "sessions", "--work--", filepath.Base(session))); err != nil {
		t.Errorf("session did not move: %v", err)
	}
	if got := svc.OMPProfileSessions(ow.Slug); got != 0 {
		t.Errorf("a parked session counts as live: %d", got)
	}
}

// TestAnOMPSpawnRunsInItsProfile: the process is told the profile by name, which
// is how omp finds ~/.omp/profiles/<name>, and gets none of the variables that
// would send it to another account.
func TestAnOMPSpawnRunsInItsProfile(t *testing.T) {
	ompMachine(t)
	ow, err := ompprofile.Create("Omp Work")
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "spawns")
	bin := filepath.Join(t.TempDir(), "fake-omp")
	script := fmt.Sprintf("#!/bin/sh\necho \"${OMP_PROFILE:-personal}|${OMP_AUTH_BROKER_URL:-}|$*\" >> %q\nexec cat\n", log)
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	svc := New(stubBins{bin: bin, profile: ow.Slug, movedProfiles: map[string]string{}},
		[]string{"OMP_AUTH_BROKER_URL=https://broker.example"}, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), providers.OMP, "", "", false, false, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 1 }, "the spawn")
	if got := spawnLines(t, log)[0]; !strings.HasPrefix(got, ow.Slug+"||") {
		t.Errorf("spawn %q, want OMP_PROFILE=%s and no broker", got, ow.Slug)
	}
	if got := svc.OMPProfileSessions(ow.Slug); got != 1 {
		t.Errorf("the profile counts %d sessions, want 1", got)
	}
	if got := svc.CodexProfileSessions(ow.Slug); got != 0 {
		t.Errorf("an omp session counted against the Codex profile of the same slug: %d", got)
	}
}

// TestOMPConversationsAreMarkedWithTheirProfile: History lists the sessions of
// every omp profile, each with the one a resume has to run in.
func TestOMPConversationsAreMarkedWithTheirProfile(t *testing.T) {
	_, personal := ompMachine(t)
	ow, err := ompprofile.Create("Omp Work")
	if err != nil {
		t.Fatal(err)
	}
	ompSessionAt(t, personal, "p1")
	ompSessionAt(t, ow.Dir, "w1")

	got := map[string]string{}
	for _, c := range ompConversations() {
		got[c.ID] = c.Profile
	}
	if len(got) != 2 || got["p1"] != ompprofile.Personal || got["w1"] != ow.Slug {
		t.Errorf("conversations = %v, want p1 in personal and w1 in %s", got, ow.Slug)
	}
}
