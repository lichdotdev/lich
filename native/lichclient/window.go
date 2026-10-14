package lichclient

import "context"

// LiveCount is how many sessions have a process running right now.
func (c *Client) LiveCount(ctx context.Context) (int, error) {
	var n int
	return n, c.Call(ctx, "terminal.LiveCount", &n)
}

// Quit ends lich: every session's process ends with it. The reply can be
// lost to the exit it announces.
func (c *Client) Quit(ctx context.Context) error {
	return c.Call(ctx, "system.Quit", nil, struct{}{})
}

// Notify raises a desktop notification: summary is the headline, detail the
// second line ("" omits it).
func (c *Client) Notify(ctx context.Context, summary, detail string) error {
	return c.Call(ctx, "system.Notify", nil, summary, detail)
}

// LastTurnSaid is the last thing the agent said in session id's conversation,
// "" when its transcript has none to read.
func (c *Client) LastTurnSaid(ctx context.Context, id string) (string, error) {
	var said struct {
		Text string `json:"text"`
	}
	return said.Text, c.Call(ctx, "terminal.LastTurnSaid", &said, id)
}
