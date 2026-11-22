package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// ChatMemory is session id's instruction files, saved memories and memory
// settings as its provider reports them.
func (c *Client) ChatMemory(ctx context.Context, id string) (api.ChatMemoryList, error) {
	return callMethod(ctx, c, api.ChatMemory, api.SessionIDOptions{ID: id})
}

// ChatSetMemory turns memory setting of session id on or off.
func (c *Client) ChatSetMemory(ctx context.Context, id, setting string, enabled bool) error {
	_, err := callMethod(ctx, c, api.ChatSetMemory, api.ChatSetMemoryOptions{ID: id, Setting: setting, Enabled: enabled})
	return err
}

// ChatCreateMemory creates the missing instruction file path of session id.
func (c *Client) ChatCreateMemory(ctx context.Context, id, path string) error {
	_, err := callMethod(ctx, c, api.ChatCreateMemory, api.ChatPathOptions{ID: id, Path: path})
	return err
}

// ChatAgents is session id's subagents as its provider reports them.
func (c *Client) ChatAgents(ctx context.Context, id string) (api.ChatAgentList, error) {
	return callMethod(ctx, c, api.ChatAgents, api.SessionIDOptions{ID: id})
}

// ChatNewAgent writes subagent name of session id in scope, and answers its
// file.
func (c *Client) ChatNewAgent(ctx context.Context, id, scope, name, desc string) (string, error) {
	res, err := callMethod(ctx, c, api.ChatNewAgent, api.ChatNewAgentOptions{ID: id, Scope: scope, Name: name, Description: desc})
	return res.Path, err
}
