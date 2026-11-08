package main

import (
	"sync"
	"time"
)

// programAlertGap is how often a session's own program may raise an alert. A
// program already owns its PTY, but nothing else bounds how often it can flip
// between blocked and working, and each flip is a toast and a notification.
const programAlertGap = 30 * time.Second

// programAlerts limits the alerts raised by OSC 7501 reports. Only the alerts:
// the store has already taken the report by the time it gets here, so the card
// and the tray always say what the program says now. A hook's reports are not
// limited, and neither is a state no alert is raised for, which the notifier
// still needs to take down a notification gone stale.
//
// The zero value is ready to use.
type programAlerts struct {
	mu   sync.Mutex
	last map[string]sentAlert
	// held is the newest alert waiting for the gap to end, per session.
	held map[string]statusReport
	// after schedules a held alert; time.AfterFunc when unset.
	after func(time.Duration, func())
}

type sentAlert struct {
	at time.Time
}

// admit answers whether r goes out now, and how long until a held alert should
// be looked at again, zero when no timer is owed. Inside the gap r is held, the
// newest one winning, including a state the session returned to: a second
// question is still a question (decideStatusNotice), only later.
func (a *programAlerts) admit(r statusReport, program bool, now time.Time) (send bool, wait time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !program {
		delete(a.held, r.id)
		return true, 0
	}
	if r.status != statusWaiting && r.status != statusDone {
		return true, 0
	}
	last, sent := a.last[r.id]
	if !sent || now.Sub(last.at) >= programAlertGap {
		a.markSent(r, now)
		return true, 0
	}
	_, armed := a.held[r.id]
	if a.held == nil {
		a.held = map[string]statusReport{}
	}
	a.held[r.id] = r
	if armed {
		return false, 0
	}
	return false, last.at.Add(programAlertGap).Sub(now)
}

func (a *programAlerts) markSent(r statusReport, now time.Time) {
	if a.last == nil {
		a.last = map[string]sentAlert{}
	}
	a.last[r.id] = sentAlert{at: now}
}

// takeHeld removes and returns the alert held for id.
func (a *programAlerts) takeHeld(id string) (statusReport, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, ok := a.held[id]
	delete(a.held, id)
	return r, ok
}

func (a *programAlerts) sent(r statusReport, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.markSent(r, now)
}

func (a *programAlerts) schedule(wait time.Duration, fn func()) {
	if a.after != nil {
		a.after(wait, fn)
		return
	}
	time.AfterFunc(wait, fn)
}

// raiseAlert hands a report the store has taken to the alert channels.
func (m *model) raiseAlert(r statusReport, program bool) {
	send, wait := m.alerts.admit(r, program, time.Now())
	if wait > 0 {
		m.alerts.schedule(wait, func() { m.releaseAlert(r.id) })
	}
	if send {
		m.notify(r)
	}
}

// releaseAlert raises the alert held for id if the session is still in that
// state, read again because the window may have moved on the session meanwhile.
func (m *model) releaseAlert(id string) {
	held, ok := m.alerts.takeHeld(id)
	if !ok {
		return
	}
	m.mu.Lock()
	r, routed := m.statusReport(id, held.previous)
	m.mu.Unlock()
	if !routed || r.status != held.status {
		return
	}
	m.alerts.sent(r, time.Now())
	m.notify(r)
}
