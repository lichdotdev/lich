package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type ThreadFix = api.ThreadFix

// AwaitThreadFix keeps a thread just handed to a session, for its agent to
// mark the commit that fixes it.
func (c *Client) AwaitThreadFix(ctx context.Context, opts api.ThreadFixOptions) error {
	_, err := callMethod(ctx, c, api.ProjectAwaitThreadFix, opts)
	return err
}

// ThreadFixes is every review thread handed to a session.
func (c *Client) ThreadFixes(ctx context.Context) ([]ThreadFix, error) {
	return callMethod(ctx, c, api.ProjectThreadFixes, api.None{})
}
