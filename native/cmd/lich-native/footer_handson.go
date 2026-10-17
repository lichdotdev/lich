package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lichdotdev/lich/native/i18n"
)

// handsOnRung says which signals beat a provider's hands-on clock
// (hands-on.ts RUNG): a "turn" provider's hooks open a turn, a "tool" one is
// heard only through its tool calls. Every provider is spelled out so a new
// one has to pick a side.
var handsOnRung = map[string]string{
	"claude": "turn", "codex": "turn", "antigravity": "turn", "opencode": "turn",
	"omp": "turn", "crush": "tool", "cursor": "turn", "kiro": "turn",
}

// handsOn reads the active session's hands-on time, re-read on every turn of
// the wall clock as the web's HandsOnReadout does through useNow.
type handsOn struct {
	fetch    func(sessionID string) (int64, error)
	onChange func()
	now      func() time.Time
	after    func(time.Duration) <-chan time.Time

	mu      sync.Mutex
	session string
	seconds int64
	err     error
	seq     int
}

func newHandsOn(fetch func(string) (int64, error), onChange func()) *handsOn {
	return &handsOn{fetch: fetch, onChange: onChange, now: time.Now, after: time.After}
}

// follow points the readout at session id ("" for none). A different session
// blanks the figure at once rather than showing the last one's under it.
func (h *handsOn) follow(id string) {
	h.mu.Lock()
	if id == h.session {
		h.mu.Unlock()
		return
	}
	h.session, h.seconds, h.err = id, 0, nil
	h.mu.Unlock()
	h.onChange()
	go h.refresh()
}

// read is the followed session's figure, or why it could not be read.
func (h *handsOn) read() (int64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seconds, h.err
}

// run re-reads on the minute until ctx ends.
func (h *handsOn) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.after(untilNextMinute(h.now())):
			h.refresh()
		}
	}
}

func (h *handsOn) refresh() {
	h.mu.Lock()
	h.seq++
	id, mine := h.session, h.seq
	h.mu.Unlock()
	if id == "" {
		return
	}
	seconds, err := h.fetch(id)
	h.mu.Lock()
	if mine != h.seq {
		h.mu.Unlock()
		return
	}
	// A failed read answers zero beside its error, as useRemoteResource's
	// empty value does.
	if err != nil {
		seconds = 0
	}
	changed := seconds != h.seconds || (err == nil) != (h.err == nil)
	h.seconds, h.err = seconds, err
	h.mu.Unlock()
	if changed {
		h.onChange()
	}
}

// handsOnReadout is HandsOnReadout. False under a minute: "0m" reads as a
// broken readout rather than a new session.
func handsOnReadout(seconds int64, err error, provider string) (footerReadout, bool) {
	label := i18n.T("shell.footerSession.handsOnTime")
	if err != nil {
		return footerReadout{Label: label, Text: "—", Detail: []string{i18n.T("shell.footerSession.handsOnFailed")}}, true
	}
	hours, minutes, ok := handsOnParts(seconds)
	if !ok {
		return footerReadout{}, false
	}
	return footerReadout{
		Label:  label,
		Text:   hours + minutes,
		Detail: []string{joinNonEmpty(" ", hours, minutes), handsOnDetail(provider)},
	}, true
}

// handsOnParts is the figure's hours ("" under one) and minutes, the minutes
// zero-padded behind an hour: "1h5m" and "1h50m" differ by one glyph in a
// strip nobody stops to parse.
func handsOnParts(seconds int64) (hours, minutes string, ok bool) {
	if seconds < 60 {
		return "", "", false
	}
	m := seconds / 60
	if m < 60 {
		return "", fmt.Sprintf("%dm", m), true
	}
	return fmt.Sprintf("%dh", m/60), fmt.Sprintf("%02dm", m%60), true
}

// handsOnDetail is the tooltip's sentence on what the clock listened to. A
// plain shell keeps the turn wording. The 15 minutes is handsOnIdleGap in
// internal/terminal/handson.go.
func handsOnDetail(provider string) string {
	if handsOnRung[provider] == "tool" {
		return i18n.T("session.handsOn.detailTool")
	}
	return i18n.T("session.handsOn.detailTurn")
}
