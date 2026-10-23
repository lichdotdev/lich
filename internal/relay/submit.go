package relay

import (
	"context"
	"fmt"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// Submit types opts.Message at a session's prompt and sends it, as the person at
// that session would, with no ticket and no answer to wait for: the window's
// pull request handoffs, which open a session on the problem they name. Its mod
// takes it where one is attached, as every ticketless delivery does.
//
// Only a provider session takes one: at a shell's prompt the Enter would run
// the message as a command. A session not running yet is refused, so a caller
// opening one waits for its prompt first. opts.Message is sanitized the way a
// relayed prompt is: it carries text strangers wrote (an issue body, a
// comment), and an ESC[201~ in it would close the paste and type the rest as
// keys.
func (s *Service) Submit(ctx context.Context, opts api.SubmitOptions) (api.None, error) {
	sess, ok := s.sessionOf(opts.SessionID)
	if !ok {
		return api.None{}, fmt.Errorf("no session %q", opts.SessionID)
	}
	if !providers.Known(sess.Kind) {
		return api.None{}, fmt.Errorf("%q runs %q, not an agent: a message sent there would run as a command", sess.Label, sess.Kind)
	}
	live, err := s.term.Live(ctx, api.SessionIDOptions{ID: opts.SessionID})
	if err != nil {
		return api.None{}, err
	}
	if !live {
		return api.None{}, fmt.Errorf("%q is not running", sess.Label)
	}
	return api.None{}, s.deliver(opts.SessionID, sanitize(opts.Message), nil)
}
