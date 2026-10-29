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

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
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
	lines := strings.Split(string(data), "\n")
	return lines[:len(lines)-1]
}

// TestSpawnLinesCountsOnlyWholeLines: `>>` creates the log before echo writes
// to it, so a read in between sees an empty file, or half a line.
func TestSpawnLinesCountsOnlyWholeLines(t *testing.T) {
	t.Parallel()
	log := filepath.Join(t.TempDir(), "spawns")
	for _, c := range []struct{ data, want string }{
		{"", "[]"},
		{"a|b", "[]"},
		{"a|b|x\n", `["a|b|x"]`},
		{"a|b|x\nc|", `["a|b|x"]`},
	} {
		if err := os.WriteFile(log, []byte(c.data), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%q", spawnLines(t, log)); got != c.want {
			t.Errorf("spawnLines over %q = %s, want %s", c.data, got, c.want)
		}
	}
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
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}) })

	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.Claude, Resume: "c1", Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 1 }, "the first spawn")
	if got := spawnLines(t, log)[0]; !strings.HasPrefix(got, "personal|personal-token|") {
		t.Errorf("Personal spawn %q, want lich's own environment", got)
	}
	if got := svc.ProfileSessions("work"); got != 0 {
		t.Errorf("work counts %d sessions before the move", got)
	}

	if _, err := svc.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: "work"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 2 }, "the respawn")
	got := spawnLines(t, log)[1]
	if !strings.HasPrefix(got, work+"||") || !strings.Contains(got, "--resume c1") {
		t.Errorf("respawn %q, want the work profile, no token, resuming c1", got)
	}
	if !svc.live("s1") {
		t.Error("the card is not running after the move")
	}
	if got := svc.ProfileSessions("work"); got != 1 {
		t.Errorf("work counts %d sessions, want 1", got)
	}
}

// TestContinueInKeepsTheCard: the process ends but the card stays, so what
// lives as long as the card (its browser) is not told it closed, while the
// relay still hears that the process it knew is gone.
func TestContinueInKeepsTheCard(t *testing.T) {
	personal, _ := profileMachine(t)
	conversationAt(t, personal, "c1")
	log := filepath.Join(t.TempDir(), "spawns")
	svc := New(stubBins{bin: spawnLogBin(t, log), providerSession: "c1", movedProfiles: map[string]string{}}, nil, events.New())
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}) })
	var cardsClosed, processesClosed []string
	svc.SetCardClosed(func(id string) { cardsClosed = append(cardsClosed, id) })
	svc.SetSessionClosed(func(id, _ string) { processesClosed = append(processesClosed, id) })

	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.Claude, Resume: "c1", Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 1 }, "the first spawn")
	if _, err := svc.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: "work"}); err != nil {
		t.Fatal(err)
	}
	if len(cardsClosed) != 0 {
		t.Errorf("Continue in closed the card %v", cardsClosed)
	}
	if fmt.Sprint(processesClosed) != "[s1]" {
		t.Errorf("the relay heard %v closed, want [s1]", processesClosed)
	}

	if _, err := svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(cardsClosed) != "[s1]" {
		t.Errorf("closing the card told %v, want [s1]", cardsClosed)
	}
}

// A card that has not had a prompt yet has an id but no transcript on disk:
// there is nothing to move, so it starts afresh in the target profile.
func TestContinueInStartsAfreshACardWithNoTranscriptYet(t *testing.T) {
	_, work := profileMachine(t)
	log := filepath.Join(t.TempDir(), "spawns")
	moved := map[string]string{}
	svc := New(stubBins{bin: spawnLogBin(t, log), providerSession: "c1", movedProfiles: moved}, nil, events.New())
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}) })

	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.Claude, Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 1 }, "the first spawn")

	if _, err := svc.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: "work"}); err != nil {
		t.Fatalf("ContinueIn of a card with no transcript yet: %v", err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 2 }, "the respawn")
	got := spawnLines(t, log)[1]
	if !strings.HasPrefix(got, work+"|") || strings.Contains(got, "--resume") {
		t.Errorf("respawn %q, want a fresh session in the work profile", got)
	}
	if moved["s1"] != "work" {
		t.Errorf("row profile %q, want work", moved["s1"])
	}
}

func TestClaudeProfileDirFollowsTheRowAndTheDisk(t *testing.T) {
	_, work := profileMachine(t)
	moved := map[string]string{}
	svc := New(stubBins{profile: "work", movedProfiles: moved}, nil, events.New())

	if dir, err := svc.profileDir("s1", providers.Crush, ""); err != nil || dir != "" {
		t.Errorf("a provider with no profiles: %q, %v", dir, err)
	}
	if dir, err := svc.profileDir("s1", providers.Claude, ""); err != nil || dir != work {
		t.Errorf("new conversation: %q, %v, want %q", dir, err, work)
	}

	conversationAt(t, mustPersonal(t), "c1")
	dir, err := svc.profileDir("s1", providers.Claude, "c1")
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
	if _, err := svc.profileDir("s1", providers.Claude, ""); !errors.Is(err, claudeprofile.ErrProfileGone) {
		t.Errorf("spawn into a deleted profile: %v, want ErrProfileGone", err)
	}
	_, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.Claude, Cols: 80, Rows: 24})
	if !errors.Is(err, claudeprofile.ErrProfileGone) {
		t.Errorf("Start: %v, want ErrProfileGone", err)
	}
	if svc.live("s1") {
		t.Error("a session spawned anyway")
	}
}

func TestAnUnreadableProfileRefusesTheSpawn(t *testing.T) {
	profileMachine(t)
	svc := New(stubBins{profileErr: errors.New("disk gone")}, nil, events.New())
	if _, err := svc.profileDir("s1", providers.Claude, ""); err == nil {
		t.Error("an unreadable row spawned into Personal")
	}
}

func TestContinueInMovesAParkedConversation(t *testing.T) {
	personal, work := profileMachine(t)
	conversationAt(t, personal, "c1")
	moved := map[string]string{}
	svc := New(stubBins{providerSession: "c1", movedProfiles: moved}, nil, events.New())

	if _, err := svc.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: "work"}); err != nil {
		t.Fatal(err)
	}
	if moved["s1"] != "work" {
		t.Errorf("row profile %q, want work", moved["s1"])
	}
	if _, err := os.Stat(filepath.Join(work, "projects", "-work", "c1.jsonl")); err != nil {
		t.Errorf("transcript did not move: %v", err)
	}
	if _, err := svc.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: claudeprofile.Personal}); err != nil {
		t.Fatal(err)
	}
	if moved["s1"] != "" {
		t.Errorf("moving back left the row on %q", moved["s1"])
	}
	if svc.live("s1") {
		t.Error("a parked session was spawned by the move")
	}
}

func TestContinueInRefusesWhatItCannotCarry(t *testing.T) {
	personal, _ := profileMachine(t)
	conversationAt(t, personal, "c1")

	empty := New(stubBins{}, nil, events.New())
	if _, err := empty.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: "work"}); err == nil {
		t.Error("moved a session with no conversation")
	}

	svc := New(stubBins{providerSession: "c1"}, nil, events.New())
	svc.turns.report("s1", statusBusy)
	if _, err := svc.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: "work"}); err == nil {
		t.Error("moved a session mid-turn")
	}
	svc.turns.report("s1", statusDone)
	if _, err := svc.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: "gone"}); err == nil {
		t.Error("moved into a profile that does not exist")
	}
	if _, err := os.Stat(filepath.Join(personal, "projects", "-work", "c1.jsonl")); err != nil {
		t.Errorf("a refused move took the transcript: %v", err)
	}
	if _, err := svc.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: "work"}); err != nil {
		t.Errorf("a turn a limit ended, or any finished one, must move: %v", err)
	}
}

// TestAResumeThatFollowsItsConversationSaysSoOnceUp: the window re-reads the
// session's account on session-profile, so the event comes with the process
// already running under the profile that holds the conversation.
func TestAResumeThatFollowsItsConversationSaysSoOnceUp(t *testing.T) {
	_, work := profileMachine(t)
	conversationAt(t, work, "c1")
	hub, rec := newProbeHub(t)
	log := filepath.Join(t.TempDir(), "spawns")
	svc := New(stubBins{bin: spawnLogBin(t, log), movedProfiles: map[string]string{}}, nil, hub)
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}) })

	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.Claude, Resume: "c1", Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	if !svc.live("s1") {
		t.Fatal("the session is not running after Start")
	}
	waitFor(t, func() bool { _, ok := rec.payloadOf(api.SessionProfileEvent.Name); return ok }, "the session-profile event")
	payload, _ := rec.payloadOf(api.SessionProfileEvent.Name)
	if fields, _ := payload.(map[string]any); fields["id"] != "s1" || fields["profile"] != "work" {
		t.Errorf("session-profile = %v, want s1 moved to work", payload)
	}
}

// TestCodexSessionsResolveInCodexProfiles: a Codex session's row names a Codex
// profile (internal/codexprofile), which has its own set of slugs, and a resume
// follows the rollout into whichever one holds it.
func TestCodexSessionsResolveInCodexProfiles(t *testing.T) {
	personal, _ := profileMachine(t)
	if err := os.MkdirAll(filepath.Join(filepath.Dir(personal), ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	cw, err := codexprofile.Create("Codex Work")
	if err != nil {
		t.Fatal(err)
	}
	moved := map[string]string{}
	svc := New(stubBins{profile: cw.Slug, movedProfiles: moved}, nil, events.New())

	if dir, err := svc.profileDir("s1", providers.Codex, ""); err != nil || dir != cw.Dir {
		t.Errorf("new conversation: %q, %v, want %q", dir, err, cw.Dir)
	}
	if _, err := svc.profileDir("s1", providers.Claude, ""); !errors.Is(err, claudeprofile.ErrProfileGone) {
		t.Errorf("a Codex slug read as a Claude profile: %v", err)
	}

	rollout := filepath.Join(filepath.Dir(personal), ".codex", "sessions", "2026", "10", "12", "rollout-2026-10-12T22-52-21-c9.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollout, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, err := svc.profileDir("s1", providers.Codex, "c9")
	if err != nil || dir != "" {
		t.Errorf("resume of a Personal rollout: %q, %v, want Personal", dir, err)
	}
	if got, ok := moved["s1"]; !ok || got != "" {
		t.Errorf("row moved to %q (set %v), want Personal", got, ok)
	}
}

// TestContinueInMovesACodexRollout: the move picks its provider from the row
// when nothing is running, and counts the profile's sessions by provider.
func TestContinueInMovesACodexRollout(t *testing.T) {
	personal, _ := profileMachine(t)
	codexHome := filepath.Join(filepath.Dir(personal), ".codex")
	cw, err := codexprofile.Create("Codex Work")
	if err != nil {
		t.Fatal(err)
	}
	rollout := filepath.Join("sessions", "2026", "10", "12", "rollout-2026-10-12T22-52-21-c9.jsonl")
	if err := os.MkdirAll(filepath.Join(codexHome, filepath.Dir(rollout)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, rollout), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	moved := map[string]string{}
	svc := New(stubBins{providerSession: "c9", movedProfiles: moved, kinds: map[string]string{"s1": providers.Codex}}, nil, events.New())

	if _, err := svc.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: cw.Slug}); err != nil {
		t.Fatal(err)
	}
	if moved["s1"] != cw.Slug {
		t.Errorf("row profile %q, want %q", moved["s1"], cw.Slug)
	}
	if _, err := os.Stat(filepath.Join(cw.Dir, rollout)); err != nil {
		t.Errorf("rollout did not move: %v", err)
	}
	if got := svc.CodexProfileSessions(cw.Slug); got != 0 {
		t.Errorf("a parked session counts as live: %d", got)
	}

	other := New(stubBins{providerSession: "c9", kinds: map[string]string{"s1": providers.Crush}}, nil, events.New())
	if _, err := other.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: cw.Slug}); err == nil {
		t.Error("moved a session of a provider with no profiles")
	}
}

// TestACodexSpawnRunsInItsProfile: the process gets the profile's CODEX_HOME,
// and none of the variables that would override the login in it.
func TestACodexSpawnRunsInItsProfile(t *testing.T) {
	profileMachine(t)
	cw, err := codexprofile.Create("Codex Work")
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "spawns")
	bin := filepath.Join(t.TempDir(), "fake-codex")
	script := fmt.Sprintf("#!/bin/sh\necho \"${CODEX_HOME:-personal}|${OPENAI_API_KEY:-}|$*\" >> %q\nexec cat\n", log)
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	svc := New(stubBins{bin: bin, profile: cw.Slug, movedProfiles: map[string]string{}},
		[]string{"OPENAI_API_KEY=lich-key"}, events.New())
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: "s1"}) })

	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.Codex, Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 1 }, "the spawn")
	if got := spawnLines(t, log)[0]; !strings.HasPrefix(got, cw.Dir+"||") {
		t.Errorf("spawn %q, want the profile's CODEX_HOME and no OPENAI_API_KEY", got)
	}
	if got := svc.CodexProfileSessions(cw.Slug); got != 1 {
		t.Errorf("the profile counts %d sessions, want 1", got)
	}
	if got := svc.ProfileSessions(cw.Slug); got != 0 {
		t.Errorf("a Codex session counted against the Claude profile of the same slug: %d", got)
	}
}
