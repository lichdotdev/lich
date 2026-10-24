package terminal

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// statusBook is the last session-status payload published for each session.
// The /events hub drops what it cannot deliver and replays nothing, so a window
// that was away would read every card as idle until the next hook; the book is
// what TurnStates answers it from.
//
// The zero value is ready to use.
type statusBook struct {
	mu   sync.Mutex
	byID map[string]bookedStatus
	// now stamps a transition; time.Now when unset.
	now func() time.Time
}

type bookedStatus struct {
	event statusEvent
	// since is when the session entered event.State, in Unix milliseconds. A
	// repeat of the state keeps it: the hook repeats `busy` all through a turn.
	since int64
}

func (b *statusBook) note(ev statusEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.byID == nil {
		b.byID = make(map[string]bookedStatus)
	}
	now := time.Now
	if b.now != nil {
		now = b.now
	}
	since := now().UnixMilli()
	if prev, ok := b.byID[ev.ID]; ok && prev.event.State == ev.State {
		since = prev.since
	}
	b.byID[ev.ID] = bookedStatus{event: ev, since: since}
}

func (b *statusBook) of(id string) (bookedStatus, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.byID[id]
	return st, ok
}

func (b *statusBook) forget(id string) {
	b.mu.Lock()
	delete(b.byID, id)
	b.mu.Unlock()
}

type TurnState = api.TurnState

// TurnStates answers turnStates over RPC.
func (s *Service) TurnStates(_ context.Context, _ api.None) ([]TurnState, error) {
	return s.turnStates(), nil
}

// turnStates is the catch-up a window takes right after attaching to /events:
// what each live session last reported, taken from the same reports the event
// stream carries, so an event emitted from here on is newer than the snapshot
// and a window applying both in that order ends on the latest.
func (s *Service) turnStates() []TurnState {
	s.mu.Lock()
	ids := make([]string, 0, len(s.sessions))
	for id, sess := range s.sessions {
		if sess.service == nil {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	slices.Sort(ids)

	out := make([]TurnState, 0, len(ids))
	for _, id := range ids {
		ts := TurnState{ID: id}
		if st, ok := s.statuses.of(id); ok {
			ts.State, ts.Tool, ts.Detail, ts.Reason = st.event.State, st.event.Tool, st.event.Detail, st.event.Reason
			ts.Since = st.since
		}
		if usage, ok := s.sessionUsage(id); ok {
			ts.Usage = &usage
		}
		if c, ok := s.cwds.of(id); ok {
			ts.Cwd, ts.Host = c.Cwd, c.Host
		}
		if ts.State != "" || ts.Usage != nil || ts.Cwd != "" || ts.Host != "" {
			out = append(out, ts)
		}
	}
	return out
}

// publishStatus records a session-status payload and pushes it. Recorded first:
// a snapshot taken between the two is then never older than an event the
// window has already been sent.
func (s *Service) publishStatus(ev statusEvent) {
	s.statuses.note(ev)
	events.Send(s.hub.Emit, api.SessionStatusEvent, ev)
}

// forgetTurn drops what a session's process left behind: the open turn, the
// last reported state and where it last was.
func (s *Service) forgetTurn(id string) {
	s.turns.forget(id)
	s.statuses.forget(id)
	s.cwds.forget(id)
}
