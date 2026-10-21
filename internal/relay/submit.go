package relay

import (
	"fmt"

	"github.com/lichdotdev/lich/internal/providers"
)

// Submit types message at a session's prompt and sends it, as the person at
// that session would, with no ticket and no answer to wait for: the window's
// pull request handoffs, which open a session on the problem they name. Its mod
// takes it where one is attached, as every ticketless delivery does.
//
// Only a provider session takes one: at a shell's prompt the Enter would run
// the message as a command. A session not running yet is refused, so a caller
// opening one waits for its prompt first. message is sanitized the way a
// relayed prompt is: it carries text strangers wrote (an issue body, a
// comment), and an ESC[201~ in it would close the paste and type the rest as
// keys.
func (s *Service) Submit(sessionID, message string) error {
	sess, ok := s.sessionOf(sessionID)
	if !ok {
		return fmt.Errorf("no session %q", sessionID)
	}
	if !providers.Known(sess.Kind) {
		return fmt.Errorf("%q runs %q, not an agent: a message sent there would run as a command", sess.Label, sess.Kind)
	}
	if !s.term.Live(sessionID) {
		return fmt.Errorf("%q is not running", sess.Label)
	}
	return s.deliver(sessionID, sanitize(message), nil)
}
