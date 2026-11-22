package chat

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"slices"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// Only Claude Code reports its background shells: Codex lists its background
// terminals on its experimental app-server API alone, with no event and no
// output, and ACP has none to report (docs/ceilings.md).
var errNoShells = errors.New("this agent does not report its background shells")

// terminalEscape is a terminal's control sequence: CSI, OSC, or a lone escape.
var terminalEscape = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)|[@-Z\\-_])`)

// plainText is text with its terminal escapes gone.
func plainText(text string) string { return terminalEscape.ReplaceAllString(text, "") }

// Shells answers api.ChatShells.
func (s *Service) Shells(ctx context.Context, opts api.SessionIDOptions) ([]api.ChatShell, error) {
	c, err := s.ensure(ctx, opts.ID)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.shells), nil
}

// ShellOutput answers api.ChatShellOutput.
func (s *Service) ShellOutput(ctx context.Context, opts api.ChatShellOptions) (string, error) {
	d, err := s.shellDriver(ctx, opts.ID)
	if err != nil {
		return "", err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return d.shellOutput(callCtx, opts.Shell)
}

// StopShell answers api.ChatStopShell.
func (s *Service) StopShell(ctx context.Context, opts api.ChatShellOptions) (api.None, error) {
	d, err := s.shellDriver(ctx, opts.ID)
	if err != nil {
		return api.None{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return api.None{}, d.stopShell(callCtx, opts.Shell)
}

func (s *Service) shellDriver(ctx context.Context, id string) (*claudeDriver, error) {
	c, err := s.ensure(ctx, id)
	if err != nil {
		return nil, err
	}
	d, ok := c.drv.(*claudeDriver)
	if !ok {
		return nil, errNoShells
	}
	return d, nil
}

// setShells records the chat's background shells and tells the window when
// they changed.
func (c *chatSession) setShells(shells []api.ChatShell) {
	c.mu.Lock()
	same := reflect.DeepEqual(c.shells, shells)
	c.shells = shells
	c.mu.Unlock()
	if !same {
		events.Send(c.svc.hub.Emit, api.ChatShellsEvent, api.ChatShellsUpdate{SessionID: c.id, Shells: shells})
	}
}
