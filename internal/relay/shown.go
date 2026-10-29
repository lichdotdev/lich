package relay

import (
	"context"
	"errors"
	"sync"

	"github.com/lichdotdev/lich/api"
)

// shownCard is the card the native window has on screen, as it last said.
type shownCard struct {
	mu sync.Mutex
	id string
	// attached is whether a window is connected to hear it; nil is no window.
	attached func() bool
}

// SetWindowAttached wires whether a native window is connected: a card shown
// by a window that has gone is not on screen. Called at startup.
func (s *Service) SetWindowAttached(attached func() bool) {
	s.shown.mu.Lock()
	defer s.shown.mu.Unlock()
	s.shown.attached = attached
}

type ShownSessionOptions = api.ShownSessionOptions

// SetShown records the session whose card the window has on screen.
func (s *Service) SetShown(_ context.Context, opts ShownSessionOptions) (api.None, error) {
	s.shown.mu.Lock()
	defer s.shown.mu.Unlock()
	s.shown.id = opts.SessionID
	return api.None{}, nil
}

// shownID is the session on screen, "" while no window is open.
func (s *Service) shownID() string {
	s.shown.mu.Lock()
	defer s.shown.mu.Unlock()
	if s.shown.attached == nil || !s.shown.attached() {
		return ""
	}
	return s.shown.id
}

// errNothingShown is Focused with no card on screen to name.
var errNothingShown = errors.New("no session is on screen: lich's window is closed or shows no card")

// Focused names the session whose card is on screen, for a caller that acts on
// whatever the person is looking at.
func (s *Service) Focused(ctx context.Context, _ api.None) (Focused, error) {
	id := s.shownID()
	if id == "" {
		return Focused{}, errNothingShown
	}
	projects, err := s.sessions.LoadState(ctx, api.None{})
	if err != nil {
		return Focused{}, err
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == id {
				return Focused{ID: id, Label: sess.Label, Project: p.Name}, nil
			}
		}
	}
	return Focused{}, errNothingShown
}
