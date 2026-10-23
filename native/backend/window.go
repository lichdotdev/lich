package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// LiveCount is how many sessions have a process running right now.
func (c *Client) LiveCount(ctx context.Context) (int, error) {
	var n int
	return n, c.Call(ctx, "terminal.LiveCount", &n)
}

// Quit ends lich: every session's process ends with it. The reply can be
// lost to the exit it announces.
func (c *Client) Quit(ctx context.Context) error {
	_, err := callMethod(ctx, c, api.SystemQuit, api.QuitOptions{})
	return err
}

// AttachNative makes this window lich's window: the tray and a second launch
// bring it forward, or start executable again once it closed.
func (c *Client) AttachNative(ctx context.Context, executable string) error {
	_, err := callMethod(ctx, c, api.SystemAttachNative, api.AttachNativeOptions{Executable: executable})
	return err
}

type TrayLabels = api.TrayLabels

// SetTrayLabels gives the tray its words; the first call puts it up.
func (c *Client) SetTrayLabels(ctx context.Context, labels TrayLabels) error {
	_, err := callMethod(ctx, c, api.TraySetLabels, labels)
	return err
}

// Notify raises a desktop notification: summary is the headline, detail the
// second line ("" omits it).
func (c *Client) Notify(ctx context.Context, summary, detail string) error {
	_, err := callMethod(ctx, c, api.SystemNotify, api.NotifyOptions{Summary: summary, Detail: detail})
	return err
}

// LastTurnSaid is the last thing the agent said in session id's conversation,
// "" when its transcript has none to read.
func (c *Client) LastTurnSaid(ctx context.Context, id string) (string, error) {
	var said struct {
		Text string `json:"text"`
	}
	return said.Text, c.Call(ctx, "terminal.LastTurnSaid", &said, id)
}
