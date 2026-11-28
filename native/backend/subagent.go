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

// SubagentCards is every provider's subagent card settings.
func (c *Client) SubagentCards(ctx context.Context) ([]api.SubagentCardProvider, error) {
	return callMethod(ctx, c, api.SubagentCards, api.None{})
}

// SetSubagentCards turns a provider's subagent cards on or off.
func (c *Client) SetSubagentCards(ctx context.Context, opts api.SubagentSetCardsOptions) error {
	_, err := callMethod(ctx, c, api.SubagentSetCards, opts)
	return err
}

// SetSubagentBring turns one item a provider's cards bring on or off.
func (c *Client) SetSubagentBring(ctx context.Context, opts api.SubagentSetBringOptions) error {
	_, err := callMethod(ctx, c, api.SubagentSetBring, opts)
	return err
}
