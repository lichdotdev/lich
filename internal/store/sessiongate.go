package store

// kindShell is terminal.KindShell, restated because internal/terminal imports
// this package.
const kindShell = "shell"

// SetSessionGate registers what may refuse a new session: fn's error is the
// refusal, nil lets it open (appupdate.Service.RequiredUpdate). Startup wiring;
// without it nothing is refused, which is what every test that does not wire
// it gets.
func (s *Service) SetSessionGate(fn func() error) {
	s.sessionGate = fn
}

// SessionRefused is why a new session of kind may not open, nil when it may.
// Every insert asks it, so the window, `lich open`, the MCP tools, a handoff
// and a fork are all answered here; a row that already exists is never asked,
// so a running or parked session keeps its work.
//
// A shell is let through on purpose: the update prompt's own Install action is
// a shell session with the command typed into it, and that command restarts
// lich through the LICH_PORT and LICH_TOKEN only a lich session exports.
func (s *Service) SessionRefused(kind string) error {
	if s.sessionGate == nil || kind == kindShell {
		return nil
	}
	return s.sessionGate()
}
