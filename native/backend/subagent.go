package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// SubagentVetoed is every subagent call lich opened as a card since it
// started, for a chat to draw as opened rather than as failed.
func (c *Client) SubagentVetoed(ctx context.Context) ([]api.SubagentVeto, error) {
	return callMethod(ctx, c, api.SubagentVetoed, api.None{})
}
