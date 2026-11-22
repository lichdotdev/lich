package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type (
	ChatItem       = api.ChatItem
	ChatToolCall   = api.ChatToolCall
	ChatPermission = api.ChatPermission
	ChatQuestion   = api.ChatQuestion
	ChatAsk        = api.ChatAsk
	ChatQReplied   = api.ChatQReplied
	ChatAskAnswer  = api.ChatAskAnswer
	ChatTurn       = api.ChatTurn
	ChatOption     = api.ChatOption
	ChatChoice     = api.ChatChoice
	ChatCommand    = api.ChatCommand
	ChatShell      = api.ChatShell
)

// ChatKinds is the providers whose sessions can be drawn as a chat.
func (c *Client) ChatKinds(ctx context.Context) ([]string, error) {
	return callMethod(ctx, c, api.ChatKinds, api.None{})
}

// ChatSetView asks for session id to be drawn in view; SessionViewEvent says
// when it is.
func (c *Client) ChatSetView(ctx context.Context, id, view string) error {
	_, err := callMethod(ctx, c, api.ChatSetView, api.ChatViewOptions{ID: id, View: view})
	return err
}

// ChatHistory is session id's conversation so far.
func (c *Client) ChatHistory(ctx context.Context, id string) ([]ChatItem, error) {
	return callMethod(ctx, c, api.ChatHistory, api.SessionIDOptions{ID: id})
}

// ChatPrompt sends text to session id's agent, with files attached.
func (c *Client) ChatPrompt(ctx context.Context, id, text string, files []Attachment) error {
	_, err := callMethod(ctx, c, api.ChatPrompt, api.ChatPromptOptions{ID: id, Text: text, Attachments: files})
	return err
}

// ChatSteer sends text, with files attached, into session id's turn in
// flight, a prompt when none is.
func (c *Client) ChatSteer(ctx context.Context, id, text string, files []Attachment) error {
	_, err := callMethod(ctx, c, api.ChatSteer, api.ChatPromptOptions{ID: id, Text: text, Attachments: files})
	return err
}

// ChatCancel stops session id's turn in flight.
func (c *Client) ChatCancel(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ChatCancel, api.SessionIDOptions{ID: id})
	return err
}

// ChatAnswer answers permission item itemID of session id with optionID.
func (c *Client) ChatAnswer(ctx context.Context, id, itemID, optionID string) error {
	_, err := callMethod(ctx, c, api.ChatAnswer, api.ChatAnswerOptions{ID: id, ItemID: itemID, OptionID: optionID})
	return err
}

// ChatReply answers question item itemID of session id, or declines it.
func (c *Client) ChatReply(ctx context.Context, id, itemID string, reply ChatQReplied) error {
	_, err := callMethod(ctx, c, api.ChatReply, api.ChatReplyOptions{ID: id, ItemID: itemID, Reply: reply})
	return err
}

// ChatOptions is what session id's chat can be set to now.
func (c *Client) ChatOptions(ctx context.Context, id string) ([]ChatOption, error) {
	return callMethod(ctx, c, api.ChatOptions, api.SessionIDOptions{ID: id})
}

// ChatCommands is what session id's provider takes typed at its prompt.
func (c *Client) ChatCommands(ctx context.Context, id string) ([]ChatCommand, error) {
	return callMethod(ctx, c, api.ChatCommands, api.SessionIDOptions{ID: id})
}

// ChatShells is session id's background shells, running and ended.
func (c *Client) ChatShells(ctx context.Context, id string) ([]ChatShell, error) {
	return callMethod(ctx, c, api.ChatShells, api.SessionIDOptions{ID: id})
}

// ChatShellOutput is the end of shell's output in session id's chat.
func (c *Client) ChatShellOutput(ctx context.Context, id, shell string) (string, error) {
	return callMethod(ctx, c, api.ChatShellOutput, api.ChatShellOptions{ID: id, Shell: shell})
}

// ChatStopShell stops shell in session id's chat.
func (c *Client) ChatStopShell(ctx context.Context, id, shell string) error {
	_, err := callMethod(ctx, c, api.ChatStopShell, api.ChatShellOptions{ID: id, Shell: shell})
	return err
}

// ChatBackground moves the commands session id's turn waits on to the
// background.
func (c *Client) ChatBackground(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ChatBackground, api.SessionIDOptions{ID: id})
	return err
}

// ChatNew starts an empty conversation in session id's chat.
func (c *Client) ChatNew(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ChatNew, api.SessionIDOptions{ID: id})
	return err
}

// ChatRunShell runs command in session id's shell, as the person's own.
func (c *Client) ChatRunShell(ctx context.Context, id, command string) error {
	_, err := callMethod(ctx, c, api.ChatRunShell, api.ChatRunShellOptions{ID: id, Command: command})
	return err
}

// ChatSetOption sets option optionID of session id's chat to the choice value.
func (c *Client) ChatSetOption(ctx context.Context, id, optionID, value string) error {
	_, err := callMethod(ctx, c, api.ChatSetOption, api.ChatSetOptionOptions{ID: id, OptionID: optionID, Value: value})
	return err
}

// ChatMCP is session id's MCP servers as its provider reports them.
func (c *Client) ChatMCP(ctx context.Context, id string) (api.ChatMCPList, error) {
	return callMethod(ctx, c, api.ChatMCP, api.SessionIDOptions{ID: id})
}

// ChatMCPAct runs action on MCP server server of session id.
func (c *Client) ChatMCPAct(ctx context.Context, id, server, action string) (api.ChatMCPActResult, error) {
	return callMethod(ctx, c, api.ChatMCPAct, api.ChatMCPActOptions{ID: id, Server: server, Action: action})
}

// ChatSkills is session id's skills as its provider reports them.
func (c *Client) ChatSkills(ctx context.Context, id string) (api.ChatSkillList, error) {
	return callMethod(ctx, c, api.ChatSkills, api.SessionIDOptions{ID: id})
}

// ChatSetSkill turns skill name of session id on or off.
func (c *Client) ChatSetSkill(ctx context.Context, id, name string, enabled bool) error {
	_, err := callMethod(ctx, c, api.ChatSetSkill, api.ChatSetSkillOptions{ID: id, Name: name, Enabled: enabled})
	return err
}

// ChatPlugins is session id's plugins and marketplaces as its provider
// reports them.
func (c *Client) ChatPlugins(ctx context.Context, id string) (api.ChatPluginList, error) {
	return callMethod(ctx, c, api.ChatPlugins, api.SessionIDOptions{ID: id})
}

// ChatPluginAct runs action of session id on target, a plugin or a
// marketplace, in scope.
func (c *Client) ChatPluginAct(ctx context.Context, id, action, target, scope string) (api.ChatPluginActResult, error) {
	return callMethod(ctx, c, api.ChatPluginAct, api.ChatPluginActOptions{ID: id, Action: action, Target: target, Scope: scope})
}

// ChatPermissions is session id's permission rules as its provider reports
// them.
func (c *Client) ChatPermissions(ctx context.Context, id string) (api.ChatPermissionList, error) {
	return callMethod(ctx, c, api.ChatPermissions, api.SessionIDOptions{ID: id})
}

// ChatPermissionAct adds or removes a permission rule of session id.
func (c *Client) ChatPermissionAct(ctx context.Context, opts api.ChatPermissionActOptions) (api.ChatPermissionActResult, error) {
	return callMethod(ctx, c, api.ChatPermissionAct, opts)
}

// ChatHooks is session id's hooks as its provider reports them.
func (c *Client) ChatHooks(ctx context.Context, id string) (api.ChatHookList, error) {
	return callMethod(ctx, c, api.ChatHooks, api.SessionIDOptions{ID: id})
}

// ChatHookAct trusts a hook of session id or turns it on or off.
func (c *Client) ChatHookAct(ctx context.Context, opts api.ChatHookActOptions) error {
	_, err := callMethod(ctx, c, api.ChatHookAct, opts)
	return err
}
