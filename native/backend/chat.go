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

// ChatPrompt sends text to session id's agent.
func (c *Client) ChatPrompt(ctx context.Context, id, text string) error {
	_, err := callMethod(ctx, c, api.ChatPrompt, api.ChatPromptOptions{ID: id, Text: text})
	return err
}

// ChatSteer sends text into session id's turn in flight, a prompt when none
// is.
func (c *Client) ChatSteer(ctx context.Context, id, text string) error {
	_, err := callMethod(ctx, c, api.ChatSteer, api.ChatPromptOptions{ID: id, Text: text})
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

// ChatNew starts an empty conversation in session id's chat.
func (c *Client) ChatNew(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ChatNew, api.SessionIDOptions{ID: id})
	return err
}

// ChatSetOption sets option optionID of session id's chat to the choice value.
func (c *Client) ChatSetOption(ctx context.Context, id, optionID, value string) error {
	_, err := callMethod(ctx, c, api.ChatSetOption, api.ChatSetOptionOptions{ID: id, OptionID: optionID, Value: value})
	return err
}
