package main

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/omartelo/lich/native/lichclient"
)

const (
	// touchedEventName is terminal.touchedEventName: a session likely changed
	// files on disk.
	touchedEventName = "session-touched"
	// focusEventName is relay.FocusEventName: `lich focus` asked for a card.
	focusEventName = "session-focus"
)

// sessionTouched reads a session's git state now rather than at the next
// tick, and tells the Review tab's diff. A session of a project that is not on
// screen has nothing drawn from git, so it waits for its project to be shown.
func (m *model) sessionTouched(ctx context.Context, data json.RawMessage) {
	var e struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(data, &e) != nil {
		return
	}
	m.mu.Lock()
	p, i := m.sessionAt(e.ID)
	if p == nil || p != m.shownProject() {
		m.mu.Unlock()
		return
	}
	shown, checkout := m.shownPath(*p, p.Sessions[i]), sessionPath(*p, p.Sessions[i])
	m.mu.Unlock()
	go m.refreshPath(ctx, shown)
	if m.diffTouched != nil {
		m.diffTouched(checkout)
	}
}

// checkoutWritten reads again every card of the project on screen that sits in
// checkout, after an edit made through lich rather than by a session.
func (m *model) checkoutWritten(ctx context.Context, checkout string) {
	m.mu.Lock()
	var paths []string
	if p := m.shownProject(); p != nil {
		for _, s := range p.Sessions {
			if path := m.shownPath(*p, s); sessionPath(*p, s) == checkout && !slices.Contains(paths, path) {
				paths = append(paths, path)
			}
		}
	}
	m.mu.Unlock()
	for _, path := range paths {
		go m.refreshPath(ctx, path)
	}
}

// sessionFocus brings the card `lich focus` named up. One whose project is not
// open here has nowhere to be shown, so it is left alone.
func (m *model) sessionFocus(data json.RawMessage) {
	var e struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(data, &e) != nil || m.focusSession == nil {
		return
	}
	m.mu.Lock()
	p, _ := m.sessionAt(e.ID)
	projectID := ""
	if p != nil {
		projectID = p.ID
	}
	m.mu.Unlock()
	if projectID != "" {
		m.focusSession(projectID, e.ID)
	}
}

// loginStep is one step of a profile login of the provider kind names. The
// profiles are read again before the screen hears the login ended, so what it
// draws next is the account that just signed in, whichever window or terminal
// ran the login.
func (m *model) loginStep(ctx context.Context, kind string, step lichclient.ProfileLogin) {
	if !step.Done {
		m.forwardLogin(kind, step)
		return
	}
	go func() {
		if book := m.profileBook(kind); book != nil {
			book.loginFinished(ctx)
		}
		m.forwardLogin(kind, step)
	}()
}

// loginEventKind is the provider a login event name belongs to.
func loginEventKind(event string) string {
	if event == lichclient.CodexProfileLoginEvent {
		return codexKind
	}
	return claudeKind
}

func (m *model) forwardLogin(kind string, step lichclient.ProfileLogin) {
	if m.loginProgress != nil {
		m.loginProgress(kind, step)
	}
}
