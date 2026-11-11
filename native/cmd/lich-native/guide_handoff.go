package main

import driver "github.com/lichdotdev/gio-driver"

const handoffID = "handoff"

// tourDelegates is the footer's delegates chip.
const tourDelegates = "handoff.delegates"

// The handoff has no button: the agent starts it (handoff_session, lich
// handoff), so the guide teaches asking for it. The successor does not take
// the screen, which is why the card steps point at the caller's row.
var handoffTour = []tourStep{
	{anchor: tourStage, side: driver.Right, until: (*window).tourHandedOff,
		text: tourText{"tourHandoff.ask.title", "tourHandoff.ask.body", "tourHandoff.ask.hint"}},
	{anchor: tourSession, side: driver.Right, text: tourText{"tourHandoff.successor.title", "tourHandoff.successor.body", ""}},
	{anchor: tourDelegates, side: driver.Top, text: tourText{"tourHandoff.delegates.title", "tourHandoff.delegates.body", ""}},
	{anchor: tourSession, side: driver.Right, text: tourText{"tourHandoff.close.title", "tourHandoff.close.body", ""}},
}

func (u *window) tourAgentOpen() bool {
	sv, ok := u.model.activeSession()
	return ok && sv.Kind != "shell"
}

// tourHandedOff: a successor's origin is the session that handed off, which
// stays the active one.
func (u *window) tourHandedOff() bool {
	id := u.model.shownSession()
	return id != "" && len(u.model.delegatesOf(id)) > 0
}

func init() {
	registerGuide(&guide{id: handoffID, order: 40, label: "tourHandoff.label", steps: handoffTour,
		ready: (*window).tourAgentOpen, unavailable: "tourHandoff.unavailable"})
}
