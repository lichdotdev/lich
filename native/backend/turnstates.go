package backend

import (
	"context"
	"encoding/json"
)

// TurnState is internal/terminal's TurnState: a live session's last
// session-status report and the session-usage payload behind its footer, both
// raw as the /events stream would have carried them. State is empty for a
// session that never reported; Since is when it entered State, in Unix
// milliseconds. Cwd and Host are its last session-cwd report.
type TurnState struct {
	ID     string          `json:"id"`
	State  string          `json:"state"`
	Tool   string          `json:"tool"`
	Detail string          `json:"detail"`
	Reason string          `json:"reason"`
	Since  int64           `json:"since"`
	Usage  json.RawMessage `json:"usage"`
	Cwd    string          `json:"cwd"`
	Host   string          `json:"host"`
}

// TurnStates is what each live session last reported, the catch-up for the
// events /events dropped while no window was attached.
func (c *Client) TurnStates(ctx context.Context) ([]TurnState, error) {
	var states []TurnState
	return states, c.Call(ctx, "terminal.TurnStates", &states)
}
