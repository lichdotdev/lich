package main

import "testing"

// Ask and Quit hold a close until lich counts its sessions. A window lich
// dropped has nobody to count them: holding the close there only raised a
// toast, and the window never closed, not even for the Windows installer.
func TestCloseIsHeldOnlyWhileLichCanCountTheSessions(t *testing.T) {
	u, b := newGateWindow(t)
	for _, action := range []string{closeAsk, closeQuit} {
		u.settings.look.prefs.closeAction = action
		if !u.closeIsHeld() {
			t.Errorf("%s: a close is let through with lich attached, want it held to count the sessions", action)
		}
	}
	u.settings.look.prefs.closeAction = closeBackground
	if u.closeIsHeld() {
		t.Error("background: a close is held, want it let through")
	}

	events, err := u.client.Events(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	go u.model.follow(t.Context(), events)
	waitFor(t, "the window attached to /events", func() bool { return b.eventsOpened.Load() == 1 })
	b.goAway()
	waitFor(t, "the window lost", u.model.connectionLost)
	for _, action := range []string{closeAsk, closeQuit} {
		u.settings.look.prefs.closeAction = action
		if u.closeIsHeld() {
			t.Errorf("%s: a close is held on a window lich dropped, want it let through", action)
		}
	}
}
