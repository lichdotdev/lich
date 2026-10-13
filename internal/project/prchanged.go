package project

import "time"

// PRChangedEvent is what a pull request write through lich reports once GitHub
// took it, so every window's pull request store reads it again instead of the
// screen that made the change telling the others.
const PRChangedEvent = "pull-request-changed"

// PRChanged names the checkout the write ran in and the pull request it
// addressed; Number is 0 where the write addresses a thread or a review rather
// than a number (the checkout's repository is then all that is known).
type PRChanged struct {
	Path   string `json:"path"`
	Number int    `json:"number"`
}

// SetEmit wires where pull request writes report (PRChangedEvent).
func (s *Service) SetEmit(emit func(name string, data any)) { s.emit = emit }

// writePR runs a gh write for the pull request and reports it once it landed.
func (s *Service) writePR(timeout time.Duration, path string, number int, args ...string) error {
	if _, err := s.gh(timeout, path, args...); err != nil {
		return err
	}
	if s.emit != nil {
		s.emit(PRChangedEvent, PRChanged{Path: path, Number: number})
	}
	return nil
}
