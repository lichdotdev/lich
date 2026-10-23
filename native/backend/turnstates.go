package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type TurnState = api.TurnState

// TurnStates is what each live session last reported, the catch-up for the
// events /events dropped while no window was attached.
func (c *Client) TurnStates(ctx context.Context) ([]TurnState, error) {
	return callMethod(ctx, c, api.TerminalTurnStates, api.None{})
}
