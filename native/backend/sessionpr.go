package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type SessionPullRequest = api.SessionPullRequest

// LinkPullRequest links the pull request ref names (#N in the repository at
// path, or its URL) to session, once the forge has answered for it.
func (c *Client) LinkPullRequest(ctx context.Context, session, path, ref string) error {
	_, err := callMethod(ctx, c, api.ProjectLinkPullRequest, api.LinkPullRequestOptions{Session: session, Path: path, Ref: ref})
	return err
}

// UnlinkPullRequest hides the pull request at url from session.
func (c *Client) UnlinkPullRequest(ctx context.Context, session, url string) error {
	_, err := callMethod(ctx, c, api.ProjectUnlinkPullRequest, api.UnlinkPullRequestOptions{Session: session, URL: url})
	return err
}

// SessionPullRequests is every pull request linked to or unlinked from a
// session by hand, the newest first.
func (c *Client) SessionPullRequests(ctx context.Context) ([]SessionPullRequest, error) {
	return callMethod(ctx, c, api.ProjectSessionPullRequests, api.None{})
}
