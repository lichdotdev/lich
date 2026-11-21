package chat

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relay"
)

// toolsDriver is a driver whose provider reports its MCP servers and skills:
// Claude Code's and Codex's. The ACP providers report neither.
type toolsDriver interface {
	mcpServers(ctx context.Context) (api.ChatMCPList, error)
	// mcpAct takes one of mcpActions; the service has checked it is one and
	// is not turning lich's own server off.
	mcpAct(ctx context.Context, server, action string) (api.ChatMCPActResult, error)
	skillList(ctx context.Context) (api.ChatSkillList, error)
	setSkill(ctx context.Context, name string, enabled bool) error
}

var (
	errNoTools   = errors.New("this agent does not report its MCP servers or skills")
	errFixedMCP  = fmt.Errorf("the %s server is lich's own and stays on", relay.MCPServerName)
	errNoMCPPage = errors.New("this server signs in through a page that calls back to the agent, which lich cannot relay")
)

// tools is session id's running chat and its driver, which ok says reports
// MCP servers and skills.
func (s *Service) tools(ctx context.Context, id string) (toolsDriver, bool, error) {
	c, err := s.ensure(ctx, id)
	if err != nil {
		return nil, false, err
	}
	d, ok := c.drv.(toolsDriver)
	return d, ok, nil
}

// MCP answers api.ChatMCP.
func (s *Service) MCP(ctx context.Context, opts api.SessionIDOptions) (api.ChatMCPList, error) {
	d, ok, err := s.tools(ctx, opts.ID)
	if err != nil || !ok {
		return api.ChatMCPList{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return d.mcpServers(callCtx)
}

// MCPAct answers api.ChatMCPAct.
func (s *Service) MCPAct(ctx context.Context, opts api.ChatMCPActOptions) (api.ChatMCPActResult, error) {
	d, ok, err := s.tools(ctx, opts.ID)
	if err != nil {
		return api.ChatMCPActResult{}, err
	}
	if !ok {
		return api.ChatMCPActResult{}, errNoTools
	}
	if opts.Action == api.MCPDisable && opts.Server == relay.MCPServerName {
		return api.ChatMCPActResult{}, errFixedMCP
	}
	if !slices.Contains(mcpActions, opts.Action) {
		return api.ChatMCPActResult{}, fmt.Errorf("this agent cannot %s an MCP server", opts.Action)
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return d.mcpAct(callCtx, opts.Server, opts.Action)
}

// Skills answers api.ChatSkills.
func (s *Service) Skills(ctx context.Context, opts api.SessionIDOptions) (api.ChatSkillList, error) {
	d, ok, err := s.tools(ctx, opts.ID)
	if err != nil || !ok {
		return api.ChatSkillList{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return d.skillList(callCtx)
}

// SetSkill answers api.ChatSetSkill.
func (s *Service) SetSkill(ctx context.Context, opts api.ChatSetSkillOptions) (api.None, error) {
	d, ok, err := s.tools(ctx, opts.ID)
	if err != nil {
		return api.None{}, err
	}
	if !ok {
		return api.None{}, errNoTools
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return api.None{}, d.setSkill(callCtx, opts.Name, opts.Enabled)
}

// fixedMCP reports whether name is the server lich adds to every session.
func fixedMCP(name string) bool { return name == relay.MCPServerName }

// mcpActions are the ChatMCPAct actions Claude Code and Codex both take.
var mcpActions = []string{api.MCPEnable, api.MCPDisable, api.MCPReconnect, api.MCPSignIn}
