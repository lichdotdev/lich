package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type (
	ChatItem       = api.ChatItem
	ChatToolCall   = api.ChatToolCall
	ChatPermission = api.ChatPermission
	ChatTurn       = api.ChatTurn
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
