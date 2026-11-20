package chat

import (
	"context"
	"reflect"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// commandSigil starts a command typed at a provider's prompt.
const commandSigil = "/"

// Commands answers api.ChatCommands.
func (s *Service) Commands(ctx context.Context, opts api.SessionIDOptions) ([]api.ChatCommand, error) {
	c, err := s.ensure(ctx, opts.ID)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.cmds), nil
}

// setCommands records what the provider takes typed at its prompt and tells
// the window when that changed.
func (c *chatSession) setCommands(cmds []api.ChatCommand) {
	c.mu.Lock()
	same := reflect.DeepEqual(c.cmds, cmds)
	c.cmds = cmds
	c.mu.Unlock()
	if !same {
		events.Send(c.svc.hub.Emit, api.ChatCommandsEvent, api.ChatCommandsUpdate{SessionID: c.id, Commands: cmds})
	}
}

// slashed is name as it is typed at the prompt: led by the sigil, which some
// providers list it with and others without.
func slashed(name string) string {
	return commandSigil + strings.TrimPrefix(name, commandSigil)
}
