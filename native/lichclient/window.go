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

// AttachNative makes this window lich's window: the tray and a second launch
// bring it forward, or start executable again once it closed.
func (c *Client) AttachNative(ctx context.Context, executable string) error {
	return c.Call(ctx, "system.AttachNative", nil, struct {
		Executable string `json:"executable"`
	}{executable})
}

// OpenWindow opens lich's web or native window, closing the other one; kind
// is "web" or "native".
func (c *Client) OpenWindow(ctx context.Context, kind string) error {
	return c.Call(ctx, "system.OpenWindow", nil, struct {
		Kind string `json:"kind"`
	}{kind})
}

// TrayLabels are the tray menu's words; Running carries a {count} placeholder.
type TrayLabels struct {
	Show    string `json:"show"`
	Running string `json:"running"`
	Quit    string `json:"quit"`
	// OpenNative and OpenWeb open the window Show does not.
	OpenNative string `json:"openNative"`
	OpenWeb    string `json:"openWeb"`
}

// SetTrayLabels gives the tray its words; the first call puts it up.
func (c *Client) SetTrayLabels(ctx context.Context, labels TrayLabels) error {
	return c.Call(ctx, "tray.SetLabels", nil, labels)
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
