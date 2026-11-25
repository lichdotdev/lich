package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type ViewedFile = api.ViewedFile

// ViewedFiles is every file of the pull request at url ticked as viewed.
func (c *Client) ViewedFiles(ctx context.Context, url string) ([]ViewedFile, error) {
	return callMethod(ctx, c, api.StoreViewedFiles, api.ViewedFilesOptions{PR: url})
}

// MarkViewed ticks path of the pull request at url, hash being its diff's.
func (c *Client) MarkViewed(ctx context.Context, url, path, hash string) error {
	_, err := callMethod(ctx, c, api.StoreMarkViewed, api.MarkViewedOptions{PR: url, Path: path, Hash: hash})
	return err
}

// UnmarkViewed unticks path of the pull request at url.
func (c *Client) UnmarkViewed(ctx context.Context, url, path string) error {
	_, err := callMethod(ctx, c, api.StoreUnmarkViewed, api.UnmarkViewedOptions{PR: url, Path: path})
	return err
}
