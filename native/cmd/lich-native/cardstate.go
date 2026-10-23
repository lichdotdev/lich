package main

import (
	"context"
	"log"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// sessionRelay is the request a session has open with another; an empty
// direction clears the mark.
type sessionRelay = api.SessionRelay

// sandboxEvent is whether a spawn ran confined, and what that sandbox left
// out of the private home for being a symlink.
type sandboxEvent = api.SessionSandbox

// Directions a relay mark runs (internal/relay DirectionOut, DirectionIn).
const (
	relayOut = "out"
	relayIn  = "in"
)

// applyStatus feeds a session-status report to the status store. An idle
// report is the provider CLI leaving the PTY, so the card drops its agent
// mark, and a report on the session being watched is read on arrival.
func (m *model) applyStatus(ctx context.Context, ev backend.Event) error {
	s, err := backend.Decode(ev, api.SessionStatusEvent)
	if err != nil {
		return err
	}
	m.mu.Lock()
	previous := m.status.status(s.ID)
	_, err = m.status.apply(ev, time.Now())
	report, routed := m.statusReport(s.ID, previous)
	var recheck string
	if previous != statusDone && m.status.status(s.ID) == statusDone {
		recheck = m.recheckMissingPR(s.ID)
	}
	if s.State == "idle" {
		// The CLI left: it can no longer answer a request either.
		delete(m.agent, s.ID)
		delete(m.relay, s.ID)
	}
	read := m.focused && s.ID == m.active && m.status.markSeen(s.ID)
	m.mu.Unlock()
	if recheck != "" {
		go m.refreshPath(ctx, recheck)
	}
	if read {
		m.markRead(ctx, s.ID)
	}
	if routed && m.notify != nil {
		m.notify(report)
	}
	return err
}

// statusReport is the reading of the report just applied to id the desktop
// channel and the attention toast take, false for a session no open project
// holds, which has nowhere to route. The caller holds m.mu.
func (m *model) statusReport(id, previous string) (statusReport, bool) {
	p, i := m.sessionAt(id)
	if p == nil {
		return statusReport{}, false
	}
	s := p.Sessions[i]
	label := s.Label
	if label == "" {
		label = i18n.T("shell.projectEvents.unlabeledSession")
	}
	return statusReport{id: id, projectID: p.ID, status: m.status.status(id), previous: previous, focused: m.focused,
		onScreen: m.shownProject() == p && m.active == id, label: label, project: p.Name, reason: m.status.reason(id),
		kind: s.Kind, branch: m.git[m.shownPath(*p, s)].Branch}, true
}

// applyLive takes the live PTY reports the backend never replays: the
// provider CLI running in a session, and whether its spawn ran confined. It
// reports whether ev was one of them.
func (m *model) applyLive(ev backend.Event) (bool, error) {
	switch ev.Name {
	case api.SessionAgentEvent.Name:
		a, err := backend.Decode(ev, api.SessionAgentEvent)
		if err != nil {
			return true, err
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		// "" is the clear every spawn sends; an unknown kind is a provider
		// from a newer backend, and the card falls back to its own kind.
		if _, known := providerIcons[a.Agent]; known {
			m.agent[a.ID] = a.Agent
		} else {
			delete(m.agent, a.ID)
		}
	case api.SessionRelayEvent.Name:
		r, err := backend.Decode(ev, api.SessionRelayEvent)
		if err != nil {
			return true, err
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if r.Direction == relayOut || r.Direction == relayIn {
			m.relay[r.ID] = r
		} else {
			delete(m.relay, r.ID)
		}
	case api.SessionSandboxEvent.Name:
		s, err := backend.Decode(ev, api.SessionSandboxEvent)
		if err != nil {
			return true, err
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.sandbox[s.ID] = s
	default:
		return false, nil
	}
	return true, nil
}

// setFocused follows the window's focus: a finished turn is read only while
// someone can see it, and coming back to the window reads the one on screen.
func (m *model) setFocused(ctx context.Context, focused bool) {
	m.mu.Lock()
	m.focused = focused
	if focused {
		m.stalePullRequests()
		m.profiles.refreshIfStale(ctx)
		m.codexProfiles.refreshIfStale(ctx)
		m.ompProfiles.refreshIfStale(ctx)
		for _, book := range m.xdgProfiles {
			book.refreshIfStale(ctx)
		}
	}
	id := m.active
	read := focused && m.status.markSeen(id)
	m.mu.Unlock()
	if read {
		m.markRead(ctx, id)
	}
}

// markRead takes down the backend's unread mark of a turn this window just
// read, so a restart does not bring the ring back.
func (m *model) markRead(ctx context.Context, id string) {
	if m.seen != nil {
		m.seen(id)
	}
	go func() {
		if err := m.client.SetSessionUnread(ctx, id, false); err != nil {
			log.Printf("mark %s read: %v", id, err)
		}
	}()
}

// setColor tints a session's card here, ahead of the backend. It reports
// false when no open session has that id.
func (m *model) setColor(id, tint string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, i := m.sessionAt(id)
	if p == nil {
		return false
	}
	p.Sessions[i].Color = tint
	return true
}

// setPinned flips a session's pin here, ahead of the
// backend. It reports false when no open session has that id.
func (m *model) setPinned(id string, pinned bool) bool {
	m.mu.Lock()
	p, i := m.sessionAt(id)
	if p != nil {
		p.Sessions[i].Pinned = pinned
	}
	m.mu.Unlock()
	return p != nil
}
