package terminal

import (
	"slices"
	"sync"
	"time"
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

// TurnState is what the window missed about one live session while it was not
// attached to /events: the last session-status report (State is empty for a
// session that never reported) and the readout the last session-usage event
// carried (nil when there is none to read). Since is when the session entered
// State, in Unix milliseconds.
type TurnState struct {
	ID     string      `json:"id"`
	State  string      `json:"state,omitempty"`
	Tool   string      `json:"tool,omitempty"`
	Detail string      `json:"detail,omitempty"`
	Reason string      `json:"reason,omitempty"`
	Since  int64       `json:"since,omitempty"`
	Usage  *usageEvent `json:"usage,omitempty"`
}

// TurnStates is the catch-up a window takes right after attaching to /events:
// what each live session last reported, taken from the same reports the event
// stream carries, so an event emitted from here on is newer than the snapshot
// and a window applying both in that order ends on the latest.
func (s *Service) TurnStates() []TurnState {
	s.mu.Lock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
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
		if ts.State != "" || ts.Usage != nil {
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
	s.hub.Emit(statusEventName, ev)
}

// forgetTurn drops what a session's process left behind that belongs to its
// provider: the open turn and the last reported state.
func (s *Service) forgetTurn(id string) {
	s.turns.forget(id)
	s.statuses.forget(id)
}
