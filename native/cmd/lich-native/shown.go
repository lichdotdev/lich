package main

import (
	"sync"
	"sync/atomic"
)

// shownReport is what the backend was told about the card on screen, the one
// `lich insert --focused` pastes into.
type shownReport struct {
	latest atomic.Value // string
	// mu orders the sends, so one that loses a race to a later one finds the
	// latest already sent and sends nothing.
	mu   sync.Mutex
	sent string
}

// reportShown tells the backend when the card on screen changed. Called every
// frame: every change to it draws one.
func (u *window) reportShown() {
	id := u.model.shownSession()
	if old, _ := u.shown.latest.Swap(id).(string); old == id {
		return
	}
	go u.sendShown()
}

func (u *window) sendShown() {
	u.shown.mu.Lock()
	defer u.shown.mu.Unlock()
	id, _ := u.shown.latest.Load().(string)
	if id == u.shown.sent {
		return
	}
	if err := u.client.SetShown(u.ctx, id); err != nil {
		u.logErr("report the card on screen", err)
		return
	}
	u.shown.sent = id
}
