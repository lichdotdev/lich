// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"errors"
	"testing"

	"github.com/lichdotdev/lich/internal/events"
)

// TestABranchInsideTheSessionBillsItsCopiedHistoryOnce is a conversation
// branched inside the session's own terminal (Claude Code's /branch): the
// branch is a transcript of its own that opens with a copy of its parent's
// responses, ids and token counts included, and the session runs on in it.
func TestABranchInsideTheSessionBillsItsCopiedHistoryOnce(t *testing.T) {
	store := newCostStore("uuid-parent")
	history := costLineJSON("m1", modelOpus, 100, 0, 0, 0) + "\n" +
		costLineJSON("m2", modelOpus, 300, 0, 0, 0) + "\n"
	writeTranscript(t, "uuid-parent", history)
	svc := New(store, nil, events.New())
	svc.prices = testRate
	if _, ok := svc.sessionUsage("s1"); !ok {
		t.Fatal("sessionUsage: want ok on the parent")
	}

	writeTranscript(t, "uuid-branch", history+costLineJSON("m3", modelOpus, 600, 0, 0, 0)+"\n")
	store.providerSession = "uuid-branch"
	svc = New(store, nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if !ok || got.CostUSD == nil {
		t.Fatal("sessionUsage: want a cost on the branch")
	}
	if !nearly(*got.CostUSD, 1.0) {
		t.Errorf("CostUSD = %v, want 1.0 — the parent's 0.4 once, and the branch's own 0.6", *got.CostUSD)
	}
}

// TestACopyItsParentNeverBilledIsBilledByTheBranch: a response the parent
// wrote after its last count reaches the branch's copy unbilled, so the branch
// bills it, and the parent resumed in the same session does not bill it again.
func TestACopyItsParentNeverBilledIsBilledByTheBranch(t *testing.T) {
	store := newCostStore("uuid-parent")
	first := costLineJSON("m1", modelOpus, 100, 0, 0, 0) + "\n"
	history := first + costLineJSON("m2", modelOpus, 300, 0, 0, 0) + "\n"
	writeTranscript(t, "uuid-parent", first)
	svc := New(store, nil, events.New())
	svc.prices = testRate
	if _, ok := svc.sessionUsage("s1"); !ok {
		t.Fatal("sessionUsage: want ok on the parent")
	}

	writeTranscript(t, "uuid-branch", history+costLineJSON("m3", modelOpus, 600, 0, 0, 0)+"\n")
	store.providerSession = "uuid-branch"
	svc = New(store, nil, events.New())
	svc.prices = testRate
	if _, ok := svc.sessionUsage("s1"); !ok {
		t.Fatal("sessionUsage: want ok on the branch")
	}

	// /resume back onto the parent, whose transcript holds m2 as well.
	writeTranscript(t, "uuid-parent", history)
	store.providerSession = "uuid-parent"
	svc = New(store, nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if !ok || got.CostUSD == nil {
		t.Fatal("sessionUsage: want a cost on the resumed parent")
	}
	if !nearly(*got.CostUSD, 1.0) {
		t.Errorf("CostUSD = %v, want 1.0 — m1, m2 and m3 once each", *got.CostUSD)
	}
}

// TestAForkLichSpawnedIsNettedOnce: a fork lich spawns is a session of its own,
// and its offset already takes the copied history off, so the responses its
// parent's session billed are not skipped a second time.
func TestAForkLichSpawnedIsNettedOnce(t *testing.T) {
	store := newCostStore("uuid-branch")
	store.forkOffsets = map[string]float64{"fork": 0.4}
	store.ledgers["parent\x00uuid-parent"] = stubLedger{cost: 0.4}
	store.responses["parent\x00m1"] = "uuid-parent"
	store.responses["parent\x00m2"] = "uuid-parent"
	writeTranscript(t, "uuid-branch", costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n"+
		costLineJSON("m2", modelOpus, 300, 0, 0, 0)+"\n"+
		costLineJSON("m3", modelOpus, 600, 0, 0, 0)+"\n")
	svc := New(store, nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("fork")

	if !ok || got.CostUSD == nil {
		t.Fatal("sessionUsage: want a cost on the fork")
	}
	if !nearly(*got.CostUSD, 0.6) {
		t.Errorf("CostUSD = %v, want the 0.6 the fork spent itself", *got.CostUSD)
	}
}

// TestBilledResponsesThatCannotBeSavedWithholdTheCost: a ledger that moved past
// responses with no record of them would let a later copy bill them again, so
// neither is saved and the next turn counts the stretch over.
func TestBilledResponsesThatCannotBeSavedWithholdTheCost(t *testing.T) {
	store := newCostStore("uuid-cost")
	store.saveResponsesErr = errors.New("disk full")
	writeTranscript(t, "uuid-cost", costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n")
	svc := New(store, nil, events.New())
	svc.prices = testRate

	got, _ := svc.sessionUsage("s1")

	if got.CostUSD != nil {
		t.Errorf("CostUSD = %v, want absent", *got.CostUSD)
	}
	if ledger := store.ledgers["s1\x00uuid-cost"]; ledger != (stubLedger{}) {
		t.Errorf("ledger = %+v, want it left where it was", ledger)
	}
}

// TestABilledResponseThatCannotBeReadStopsTheCount: whether a line was already
// billed is the one thing the scan cannot guess at.
func TestABilledResponseThatCannotBeReadStopsTheCount(t *testing.T) {
	store := newCostStore("uuid-cost")
	store.countedErr = errors.New("database is locked")
	writeTranscript(t, "uuid-cost", costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n")
	svc := New(store, nil, events.New())
	svc.prices = testRate

	got, _ := svc.sessionUsage("s1")

	if got.CostUSD != nil {
		t.Errorf("CostUSD = %v, want absent", *got.CostUSD)
	}
	if got.CostMiss != "" {
		t.Errorf("CostMiss = %q, want none: a read that fails heals by itself", got.CostMiss)
	}
}
