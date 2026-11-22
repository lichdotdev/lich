package chat

import (
	"context"
	"errors"

	"github.com/lichdotdev/lich/api"
)

// shellDriver is a driver whose protocol runs the person's own shell command,
// as the provider's TUI does for a line typed after "!": Claude Code's
// bash_command, Codex's thread/shellCommand and opencode's /session/{id}/shell.
// The turn it opens ends through endTurn.
type shellDriver interface {
	// runsShell is false for an ACP agent other than opencode: ACP itself
	// has no such method.
	runsShell() bool
	shell(command string) error
}

var errNoShell = errors.New("this agent runs no shell command of the person's own")

// shellMark is what the person typed before the command, kept on their
// message as they wrote it.
const shellMark = "!"

// Shell answers api.ChatShell: the command runs in a turn of its own, one at
// a time, as Claude Code allows only one at once (measured on 2.1.330).
func (s *Service) Shell(ctx context.Context, opts api.ChatShellOptions) (api.None, error) {
	c, err := s.ensure(ctx, opts.ID)
	if err != nil {
		return api.None{}, err
	}
	d, ok := c.drv.(shellDriver)
	if !ok || !d.runsShell() {
		return api.None{}, errNoShell
	}
	if s.term.TurnOpen(opts.ID) && !c.turnRunning() {
		return api.None{}, errBusy
	}
	if err := c.beginTurn(); err != nil {
		return api.None{}, err
	}
	c.conv.userMessage(shellMark+opts.Command, nil)
	c.markStart()
	if err := d.shell(opts.Command); err != nil {
		c.conv.notice(err.Error())
		c.endTurn(stopError)
		return api.None{}, err
	}
	return api.None{}, nil
}
