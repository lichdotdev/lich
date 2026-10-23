package project

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

import "time"

type PRChanged = api.PRChanged

// SetEmit wires where pull request writes report (api.PullRequestChangedEvent).
func (s *Service) SetEmit(emit func(name string, data any)) { s.emit = emit }

// writePR runs a gh write for the pull request and reports it once it landed.
func (s *Service) writePR(timeout time.Duration, path string, number int, args ...string) error {
	if _, err := s.gh(timeout, path, args...); err != nil {
		return err
	}
	if s.emit != nil {
		events.Send(s.emit, api.PullRequestChangedEvent, PRChanged{Path: path, Number: number})
	}
	return nil
}
