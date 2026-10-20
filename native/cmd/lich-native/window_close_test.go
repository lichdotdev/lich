package main

import "testing"

// Ask and Quit hold a close until lich counts its sessions. A window lich
// dropped has nobody to count them: holding the close there only raised a
// toast, and the window never closed, not even for the Windows installer.
func TestCloseIsHeldOnlyWhileLichCanCountTheSessions(t *testing.T) {
	prev := reconnectWait
	reconnectWait = 3 * reconnectPoll
	t.Cleanup(func() { reconnectWait = prev })

	r := newReconnectRig(t)
	for _, action := range []string{closeAsk, closeQuit} {
		r.u.settings.look.prefs.closeAction = action
		if !r.u.closeIsHeld() {
			t.Errorf("%s: a close is let through with lich attached, want it held to count the sessions", action)
		}
	}
	r.u.settings.look.prefs.closeAction = closeBackground
	if r.u.closeIsHeld() {
		t.Error("background: a close is held, want it let through")
	}

	r.name(t, r.old)
	r.old.goAway()
	waitFor(t, "the window lost", r.lost)
	for _, action := range []string{closeAsk, closeQuit} {
		r.u.settings.look.prefs.closeAction = action
		if r.u.closeIsHeld() {
			t.Errorf("%s: a close is held on a window lich dropped, want it let through", action)
		}
	}
}

func TestCloseIsLetThroughWhileTheWindowReconnects(t *testing.T) {
	u, _ := newGateWindow(t)
	u.settings.look.prefs.closeAction = closeAsk
	u.reconnecting.Store(true)
	if u.closeIsHeld() {
		t.Error("a close is held while the window waits for a restarted lich, want it let through")
	}
}
