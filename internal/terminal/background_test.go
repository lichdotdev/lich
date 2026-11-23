package terminal

import "testing"

// A terminal session's turn-end report says whether its agent left work
// running in the background, and the relay, watching the same report, reads
// it already recorded.
func TestADoneReportsBackgroundWorkToTheRelay(t *testing.T) {
	hub, _ := newProbeHub(t)
	svc := New(stubBins{}, nil, hub)
	var atDone []bool
	svc.SetSessionState(func(id, state string) {
		if state == statusDone {
			atDone = append(atDone, svc.BackgroundWork(id))
		}
	})

	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusDone, Background: true})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusDone})

	if len(atDone) != 2 || !atDone[0] || atDone[1] {
		t.Errorf("background work at each done = %v, want [true false]", atDone)
	}
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusDone, Background: true})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusIdle})
	if svc.BackgroundWork("s1") {
		t.Error("an ended session still reports background work")
	}
}
