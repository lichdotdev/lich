package chat

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/lichdotdev/lich/api"
)

var errNoAgent = errors.New("opencode named no agent to run the command under")

// runsShell is opencode's alone: ACP has no shell command of the person's.
func (d *acpDriver) runsShell() bool { return d.server != nil }

// shell runs the command as the TUI's `!` does, through opencode's server
// under the agent in force: it lands in the conversation, which the agent
// sees, and reaches the chat as a tool call over ACP. The request answers
// once the command is done (measured on 1.19.4), so it goes off the caller,
// and its answer ends the turn.
func (d *acpDriver) shell(command string) error {
	agent := d.c.currents()[api.OptionMode]
	if agent == "" {
		return errNoAgent
	}
	path := "/session/" + url.PathEscape(d.sessionID()) + "/shell"
	go func() {
		if err := d.openCodeDo(context.Background(), http.MethodPost, path, map[string]string{"agent": agent, "command": command}, nil); err != nil {
			d.c.conv.notice(err.Error())
			d.c.endTurn(stopError)
			return
		}
		d.c.endTurn(stopEndTurn)
	}()
	return nil
}
