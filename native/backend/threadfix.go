package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type ThreadFix = api.ThreadFix

// AwaitThreadFix answers a thread just handed to a session on GitHub and has
// it resolve on the pull request's next push.
func (c *Client) AwaitThreadFix(ctx context.Context, opts api.ThreadFixOptions) error {
	_, err := callMethod(ctx, c, api.ProjectAwaitThreadFix, opts)
	return err
}

// ThreadFixes is every review thread waiting for its push.
func (c *Client) ThreadFixes(ctx context.Context) ([]ThreadFix, error) {
	return callMethod(ctx, c, api.ProjectThreadFixes, api.None{})
}
