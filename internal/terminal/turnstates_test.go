// Reuses helpers from the Unix-tagged terminal_test.go, so it carries the same
// tag. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

func serviceWithSessions(t *testing.T, store Store, ids ...string) *Service {
	t.Helper()
	svc := New(store, nil, events.New())
	svc.mu.Lock()
	for _, id := range ids {
		svc.sessions[id] = &session{done: make(chan struct{}), pty: &shortWritePTY{}}
	}
	svc.mu.Unlock()
	return svc
}

func turnStateOf(t *testing.T, svc *Service, id string) TurnState {
	t.Helper()
	for _, ts := range svc.TurnStates() {
		if ts.ID == id {
			return ts
		}
	}
	t.Fatalf("TurnStates has no entry for %q: %+v", id, svc.TurnStates())
	return TurnState{}
}

// A window that was away reads its cards from this: the last report of each
// live session, with the tool a pre-tool report named.
func TestTurnStatesCarryTheLastReportOfEachLiveSession(t *testing.T) {
	svc := serviceWithSessions(t, hookStore{}, "s1", "s2")
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy, Tool: "Bash", Detail: "go test ./..."})
	svc.onHookState(hookRequest{SessionID: "s2", State: statusBusy})
	svc.onHookState(hookRequest{SessionID: "s2", State: statusWaiting, Reason: "permission"})

	s1 := turnStateOf(t, svc, "s1")
	if s1.State != statusBusy || s1.Tool != "Bash" || s1.Detail != "go test ./..." {
		t.Errorf("s1 = %+v, want busy running Bash: go test ./...", s1)
	}
	s2 := turnStateOf(t, svc, "s2")
	if s2.State != statusWaiting || s2.Reason != "permission" {
		t.Errorf("s2 = %+v, want waiting on permission", s2)
	}
}

// A report the window was never told (a `waiting` outside a turn) is not the
// session's state either.
func TestTurnStatesHoldBackWhatTheWindowWasNotTold(t *testing.T) {
	svc := serviceWithSessions(t, hookStore{}, "s1")
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusDone})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusWaiting})

	if got := turnStateOf(t, svc, "s1").State; got != statusDone {
		t.Errorf("state = %q, want %q: the idle-prompt wait was never published", got, statusDone)
	}
}

func TestTurnStatesRecordAnInterruptedTurn(t *testing.T) {
	svc := serviceWithSessions(t, hookStore{}, "s1")
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	svc.noteInterrupt("s1")

	if got := turnStateOf(t, svc, "s1").State; got != statusInterrupted {
		t.Errorf("state = %q, want %q", got, statusInterrupted)
	}
}

// Since moves on a transition only: the hook repeats a state through a turn and
// a session has been busy since it started.
func TestTurnStatesStampTheTransitionNotTheRepeat(t *testing.T) {
	svc := serviceWithSessions(t, hookStore{}, "s1")
	clock := time.UnixMilli(1_000)
	svc.statuses.now = func() time.Time { return clock }
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	clock = clock.Add(time.Minute)
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy, Tool: "Read"})
	if got := turnStateOf(t, svc, "s1").Since; got != 1_000 {
		t.Errorf("since after a repeat = %d, want 1000", got)
	}
	svc.onHookState(hookRequest{SessionID: "s1", State: statusDone})
	if got := turnStateOf(t, svc, "s1").Since; got != 61_000 {
		t.Errorf("since after a transition = %d, want 61000", got)
	}
}

// A dead session answers for nothing: a closed card, and a PTY that exited,
// are not in the snapshot, and a respawn under the same id starts blank.
func TestTurnStatesForgetASessionThatLeft(t *testing.T) {
	svc := serviceWithSessions(t, hookStore{}, "closed", "respawned")
	svc.onHookState(hookRequest{SessionID: "closed", State: statusBusy})
	svc.onHookState(hookRequest{SessionID: "respawned", State: statusBusy})

	if err := svc.Close("closed"); err != nil {
		t.Fatal(err)
	}
	svc.forgetTurn("respawned")

	if got := svc.TurnStates(); len(got) != 0 {
		t.Errorf("TurnStates = %+v, want none", got)
	}
}

// The footer's readout rides along, so a window that missed the turn's end
// draws the context the turn left rather than a blank footer.
func TestTurnStatesCarryTheUsageReadout(t *testing.T) {
	writeTranscript(t, "uuid-snap", costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n")
	svc := serviceWithSessions(t, newCostStore("uuid-snap"), "s1")
	svc.prices = testRate

	s1 := turnStateOf(t, svc, "s1")
	if s1.State != "" {
		t.Errorf("s1 state = %q, want none: it never reported", s1.State)
	}
	if s1.Usage == nil || s1.Usage.ID != "s1" || s1.Usage.Model != modelOpus {
		t.Errorf("s1 usage = %+v, want the opus readout of s1", s1.Usage)
	}
}

// With no transcript behind it a session has no usage to report, and reports
// its state alone.
func TestTurnStatesLeaveOutUsageThereIsNoneOf(t *testing.T) {
	svc := serviceWithSessions(t, hookStore{}, "s1")
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})

	if got := turnStateOf(t, svc, "s1"); got.Usage != nil {
		t.Errorf("usage = %+v, want none", got.Usage)
	}
}

// A window that attached after a cd reads where the session went, and the host
// it went into, rather than its checkout.
func TestTurnStatesCarryWhereTheSessionLastWas(t *testing.T) {
	svc := serviceWithSessions(t, hookStore{}, "moved", "hosted")
	svc.publishCwd(cwdEvent{ID: "moved", Cwd: "/repo"})
	svc.publishCwd(cwdEvent{ID: "moved", Cwd: "/elsewhere"})
	svc.publishCwd(cwdEvent{ID: "hosted", Host: "tmux"})

	if got := turnStateOf(t, svc, "moved"); got.Cwd != "/elsewhere" || got.Host != "" || got.State != "" {
		t.Errorf("moved = %+v, want only cwd /elsewhere", got)
	}
	if got := turnStateOf(t, svc, "hosted"); got.Cwd != "" || got.Host != "tmux" {
		t.Errorf("hosted = %+v, want host tmux and no cwd", got)
	}

	svc.forgetTurn("moved")
	if _, ok := svc.cwds.of("moved"); ok {
		t.Error("a session whose process left kept its directory")
	}
}
