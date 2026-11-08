package main

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestProgramAlertsHoldInsideTheGap(t *testing.T) {
	var a programAlerts
	t0 := time.Now()
	waiting := statusReport{id: "s", status: statusWaiting}
	done := statusReport{id: "s", status: statusDone}

	if send, _ := a.admit(waiting, t0); !send {
		t.Fatal("the first alert was held")
	}
	if send, wait := a.admit(statusReport{id: "s", status: statusBusy}, t0.Add(2*time.Second)); !send || wait != 0 {
		t.Fatalf("no alert is raised for busy, and the notifier must still hear it: send %v wait %v", send, wait)
	}
	if send, wait := a.admit(done, t0.Add(10*time.Second)); send || wait != 20*time.Second {
		t.Fatalf("another state: send %v wait %v, want held 20s", send, wait)
	}
	if _, wait := a.admit(done, t0.Add(11*time.Second)); wait != 0 {
		t.Fatalf("a held alert armed a second timer: %v", wait)
	}
	if send, _ := a.admit(waiting, t0.Add(30*time.Second)); !send {
		t.Fatal("the gap ended and the alert was still limited")
	}
}

func TestAHookReportDropsAHeldAlert(t *testing.T) {
	r, timers := programRig(t)
	r.program(t, "a", statusWaiting)
	r.program(t, "a", statusDone)
	r.report(t, "a", statusBusy)
	(*timers)[0]()
	if len(r.sent) != 1 {
		t.Fatalf("sent %v, want the held alert dropped by the hook's report", r.sent)
	}
}

func TestClosingASessionForgetsItsGap(t *testing.T) {
	var a programAlerts
	t0 := time.Now()
	r := statusReport{id: "s", status: statusWaiting}
	a.admit(r, t0)
	a.forget("s")
	if send, _ := a.admit(r, t0.Add(time.Second)); !send || len(a.last) != 1 {
		t.Fatalf("send %v, last %v, want the gap gone with the session", send, a.last)
	}
}

// programRig is a notifyRig whose held alerts run when the test says.
func programRig(t *testing.T) (*notifyRig, *[]func()) {
	t.Helper()
	r := newNotifyRig(notifyPrefs{attention: optInYes, finishedTurn: true})
	var timers []func()
	r.m.alerts.after = func(_ time.Duration, fn func()) { timers = append(timers, fn) }
	return r, &timers
}

func (r *notifyRig) program(t *testing.T, id, state string) {
	t.Helper()
	if err := r.m.applyStatus(context.Background(), statusEv(t, statusEvent{ID: id, State: state, Program: true})); err != nil {
		t.Fatal(err)
	}
}

func TestProgramReportMovesTheCardAtOnceAndHoldsOnlyTheAlert(t *testing.T) {
	r, timers := programRig(t)
	r.program(t, "a", statusWaiting)
	r.program(t, "a", statusDone)
	if got := r.m.status.status("a"); got != statusDone {
		t.Fatalf("card = %q, want done at once", got)
	}
	if len(r.sent) != 1 || len(*timers) != 1 {
		t.Fatalf("sent %v with %d timers, want the first alert and one held", r.sent, len(*timers))
	}
	(*timers)[0]()
	if len(r.sent) != 2 {
		t.Fatalf("sent %v, want the held alert out when the gap ends", r.sent)
	}
}

func TestAHeldAlertIsDroppedWhenTheStateMovedOn(t *testing.T) {
	r, timers := programRig(t)
	r.program(t, "a", statusWaiting)
	r.program(t, "a", statusDone)
	r.program(t, "a", statusBusy)
	(*timers)[0]()
	if len(r.sent) != 1 {
		t.Fatalf("sent %v, want nothing for a state the session left", r.sent)
	}
}

func TestAProgramsIdleLeavesTheAgentAndRelayMarks(t *testing.T) {
	r, _ := programRig(t)
	r.m.agent["a"] = "crush"
	r.m.relay["a"] = sessionRelay{ID: "a", Direction: relayOut}
	r.program(t, "a", "idle")
	if r.m.agent["a"] != "crush" {
		t.Fatal("a program's idle dropped the provider mark")
	}
	if _, ok := r.m.relay["a"]; !ok {
		t.Fatal("a program's idle dropped the relay mark")
	}
	r.report(t, "a", "idle")
	if _, ok := r.m.agent["a"]; ok {
		t.Fatal("the hook's idle kept the provider mark")
	}
}

func TestAProgramsDoneLeavesThePullRequestLookup(t *testing.T) {
	r, _ := programRig(t)
	path := r.m.shownPath(r.m.projects[0], r.m.projects[0].Sessions[0])
	r.m.prs[path] = prLookup{head: "abc"}
	r.program(t, "a", statusDone)
	if r.m.prs[path].head != "abc" {
		t.Fatal("a program's done re-ran the pull request lookup")
	}
}

func TestReenteringTheAlertedStateAlertsAgainAfterTheGap(t *testing.T) {
	r, timers := programRig(t)
	r.program(t, "a", statusWaiting)
	r.program(t, "a", statusBusy)
	r.program(t, "a", statusWaiting)
	if len(r.sent) != 1 || len(*timers) != 1 {
		t.Fatalf("sent %v with %d timers, want one alert and the second question held", r.sent, len(*timers))
	}
	(*timers)[0]()
	if len(r.sent) != 2 {
		t.Fatalf("sent %v, want the second question out when the gap ends", r.sent)
	}
}

func TestAProgramsBusyWithoutALabelClearsTheTool(t *testing.T) {
	var s statusEntry
	s.applyTool(statusEvent{State: statusBusy, Tool: "cargo", Program: true})
	s.applyTool(statusEvent{State: statusBusy, Program: true})
	if s.tool != nil {
		t.Fatalf("tool = %+v, want cleared", s.tool)
	}
	s.applyTool(statusEvent{State: statusBusy, Tool: "Bash"})
	s.applyTool(statusEvent{State: statusBusy})
	if s.tool == nil {
		t.Fatal("a hook's gap lost its tool")
	}
}

func TestAProgramsDoneIsNotRecappedFromTheAgentsLastReply(t *testing.T) {
	r, _ := programRig(t)
	said := make(chan string, 1)
	r.n.sink = recapping{next: fakeNotes{sent: &r.sent, withdrawn: &r.withdrawn}, said: func(id string) (string, error) {
		said <- id
		return "an old reply", nil
	}}
	r.program(t, "a", statusDone)
	select {
	case id := <-said:
		t.Fatalf("read the transcript of %s for a program's done", id)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAHeldAlertStillTakesDownAStaleNotification(t *testing.T) {
	r, _ := programRig(t)
	r.program(t, "a", statusWaiting)
	r.program(t, "a", statusDone)
	if !slices.Contains(r.withdrawn, "a") {
		t.Fatalf("withdrawn %v, want the needs-input notification down at once", r.withdrawn)
	}
}
