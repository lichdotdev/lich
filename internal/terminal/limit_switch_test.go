// stubBins is Unix-tagged (terminal_test.go); nothing here is
// platform-specific.
//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// TestALimitMovesTheSessionToAProfileWithQuotaLeft: the conversation moves to
// the first other profile whose plan has quota, the row follows, and the window
// is told it was lich's move off a limit.
func TestALimitMovesTheSessionToAProfileWithQuotaLeft(t *testing.T) {
	_, work := profileMachine(t)
	personal := filepath.Join(os.Getenv("HOME"), ".claude")
	conversationAt(t, personal, "c1")
	hub, rec := newProbeHub(t)
	moved := map[string]string{}
	svc := New(stubBins{providerSession: "c1", movedProfiles: moved}, nil, hub)
	var asked []string
	svc.SetQuotaLeft(func(kind, slug string) bool {
		asked = append(asked, kind+"/"+slug)
		return slug == "work"
	})

	if !svc.ContinueOffLimit("s1") {
		t.Fatal("the session stayed although work has quota left")
	}
	if moved["s1"] != "work" {
		t.Errorf("row profile = %q, want work", moved["s1"])
	}
	if len(asked) != 1 || asked[0] != "claude/work" {
		t.Errorf("asked about %v, want only claude/work: the profile it ran in is not a target", asked)
	}
	if _, err := os.Stat(filepath.Join(work, "projects", "-work", "c1.jsonl")); err != nil {
		t.Errorf("the conversation did not move to work: %v", err)
	}
	waitFor(t, func() bool { _, ok := rec.payloadOf(api.SessionProfileEvent.Name); return ok }, "the session-profile event")
	payload, _ := rec.payloadOf(api.SessionProfileEvent.Name)
	if fields, _ := payload.(map[string]any); fields["profile"] != "work" || fields["reason"] != api.ProfileMovedOffLimit {
		t.Errorf("session-profile = %v, want work for a usage limit", payload)
	}
}

// TestALimitWithNoProfileWithQuotaLeavesTheSession: a "no" is also what a
// profile whose plan cannot be read answers (quota.HasQuotaLeft), so the
// session waits out its reset where it is.
func TestALimitWithNoProfileWithQuotaLeavesTheSession(t *testing.T) {
	profileMachine(t)
	conversationAt(t, filepath.Join(os.Getenv("HOME"), ".claude"), "c1")
	moved := map[string]string{}
	svc := New(stubBins{providerSession: "c1", movedProfiles: moved}, nil, events.New())
	svc.SetQuotaLeft(func(string, string) bool { return false })

	if svc.ContinueOffLimit("s1") {
		t.Fatal("the session moved with no profile known to have quota")
	}
	if _, ok := moved["s1"]; ok {
		t.Errorf("row profile changed to %q", moved["s1"])
	}
}

// TestALimitSkipsAProfileALimitStoppedBeforeItsReset: the reading is cached for
// minutes, so a profile a limit already stopped a turn in is not trusted to
// have quota until it resets, or two spent profiles would trade the session.
func TestALimitSkipsAProfileALimitStoppedBeforeItsReset(t *testing.T) {
	profileMachine(t)
	conversationAt(t, filepath.Join(os.Getenv("HOME"), ".claude"), "c1")
	moved := map[string]string{"other": "work"}
	svc := New(stubBins{providerSession: "c1", movedProfiles: moved}, nil, events.New())
	svc.SetQuotaLeft(func(string, string) bool { return true })
	svc.markSpent("other", time.Now().Add(time.Hour).Unix())

	if svc.ContinueOffLimit("s1") {
		t.Fatalf("the session moved to %q, a profile spent until its reset", moved["s1"])
	}
	svc.markSpent("other", time.Now().Add(-time.Minute).Unix())
	if !svc.ContinueOffLimit("s1") || moved["s1"] != "work" {
		t.Errorf("after the reset the session is in %q, want work", moved["s1"])
	}
}
