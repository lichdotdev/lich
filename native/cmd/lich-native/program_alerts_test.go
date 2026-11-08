package main

import (
	"context"
	"testing"
	"time"
)

func TestProgramAlertsHoldTheOtherStateAndDropARepeat(t *testing.T) {
	var a programAlerts
	t0 := time.Now()
	waiting := statusReport{id: "s", status: statusWaiting}
	done := statusReport{id: "s", status: statusDone}

	if send, _ := a.admit(waiting, true, t0); !send {
		t.Fatal("the first alert was held")
	}
	if send, wait := a.admit(waiting, true, t0.Add(time.Second)); send || wait != 0 {
		t.Fatalf("a repeat: send %v wait %v, want dropped", send, wait)
	}
	if send, wait := a.admit(statusReport{id: "s", status: statusBusy}, true, t0.Add(2*time.Second)); !send || wait != 0 {
		t.Fatalf("no alert is raised for busy, and the notifier must still hear it: send %v wait %v", send, wait)
	}
	if send, wait := a.admit(done, true, t0.Add(10*time.Second)); send || wait != 20*time.Second {
		t.Fatalf("another state: send %v wait %v, want held 20s", send, wait)
	}
	if _, wait := a.admit(done, true, t0.Add(11*time.Second)); wait != 0 {
		t.Fatalf("a held alert armed a second timer: %v", wait)
	}
	if send, _ := a.admit(waiting, true, t0.Add(30*time.Second)); !send {
		t.Fatal("the gap ended and the alert was still limited")
	}
}

func TestAHookReportIsNotLimitedAndDropsAHeldAlert(t *testing.T) {
	var a programAlerts
	t0 := time.Now()
	a.admit(statusReport{id: "s", status: statusWaiting}, true, t0)
	a.admit(statusReport{id: "s", status: statusDone}, true, t0.Add(time.Second))
	if send, _ := a.admit(statusReport{id: "s", status: statusDone}, false, t0.Add(2*time.Second)); !send {
		t.Fatal("a hook's alert was limited")
	}
	if _, ok := a.takeHeld("s"); ok {
		t.Fatal("the hook's report left the program's alert held")
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
