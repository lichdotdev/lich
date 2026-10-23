package relay

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

type FocusEvent = api.SessionFocus

type Focused = api.FocusedSession

// SetRaiseWindow wires how the relay brings lich's window to the front, or
// opens one on the session's card when none is open. Nil (a test that does not
// care) leaves Focus opening the card wherever the window is. Called at startup.
func (s *Service) SetRaiseWindow(raise func(sessionID string)) {
	s.raiseWindow = raise
}

type FocusOptions = api.FocusSessionOptions

// Focus opens the card of the session Target names and brings the window to
// the front, for an editor that just pointed the person at that session.
// Target resolves the way Send's does.
func (s *Service) Focus(opts FocusOptions) (Focused, error) {
	fromID, target, project := opts.From, opts.Target, opts.Project
	dest, err := s.resolve(fromID, target, project)
	if err != nil {
		return Focused{}, err
	}
	if s.events != nil {
		events.Send(s.events.Emit, api.SessionFocusEvent, FocusEvent{ID: dest.ID})
	}
	if s.raiseWindow != nil {
		s.raiseWindow(dest.ID)
	}
	return Focused{ID: dest.ID, Label: dest.Peer.Label, Project: dest.Peer.Project}, nil
}
