package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type Library = api.Library

type LibraryItem = api.LibraryItem

type LibraryFound = api.LibraryFound

type LibraryProvider = api.LibraryProvider

// Library reads lich's library, what the providers hold beside it and lich's
// own plugin.
func (c *Client) Library(ctx context.Context) (Library, error) {
	return callMethod(ctx, c, api.LibraryList, api.None{})
}

// MoveIn copies a found item into the library and turns it off where it was.
func (c *Client) MoveIn(ctx context.Context, kind, name string) error {
	_, err := callMethod(ctx, c, api.LibraryMoveIn, api.LibraryItemOptions{Kind: kind, Name: name})
	return err
}

// SetLibrarySwitch turns an item on or off for a provider, or one of its
// profiles.
func (c *Client) SetLibrarySwitch(ctx context.Context, opts api.LibrarySwitchOptions) error {
	_, err := callMethod(ctx, c, api.LibrarySet, opts)
	return err
}

// LibrarySeen records every found item of kind as seen.
func (c *Client) LibrarySeen(ctx context.Context, kind string) error {
	_, err := callMethod(ctx, c, api.LibrarySeen, api.LibrarySeenOptions{Kind: kind})
	return err
}
