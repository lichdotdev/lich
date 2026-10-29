package cli

import (
	"context"
	"fmt"

	"github.com/lichdotdev/lich/api"
)

// indexedTab is a tab as browser_tabs and lich browser tabs list it: by the
// index a tab action takes, not Chrome's id.
type indexedTab struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Front bool   `json:"front,omitempty"`
}

func mcpBrowserTabs(ctx context.Context, c *client, _ mcpArgs) (string, error) {
	tabs, err := indexedTabs(ctx, c)
	if err != nil {
		return "", err
	}
	return encodeJSON(tabs)
}

func indexedTabs(ctx context.Context, c *client) ([]indexedTab, error) {
	tabs, err := browserTabs(ctx, c)
	if err != nil {
		return nil, err
	}
	out := make([]indexedTab, len(tabs))
	for i, t := range tabs {
		out[i] = indexedTab{Index: i, Title: t.Title, URL: t.URL, Front: t.Front}
	}
	return out, nil
}

func mcpBrowserTab(ctx context.Context, c *client, a mcpArgs) (string, error) {
	id, err := c.browserOwner()
	if err != nil {
		return "", err
	}
	info, err := actOnTab(ctx, c, api.BrowserTabOptions{ID: id, Action: a.text("action"), URL: a.text("url")}, a.optionalInt("index"))
	if err != nil {
		return "", err
	}
	return pageText(info, c.lang()), nil
}

// actOnTab runs a tab action, on the tab at index for select and close.
func actOnTab(ctx context.Context, c *client, opts api.BrowserTabOptions, index *int) (api.BrowserPageInfo, error) {
	if opts.Action != api.TabNew {
		var err error
		if opts.Tab, err = tabAt(ctx, c, index); err != nil {
			return api.BrowserPageInfo{}, err
		}
	}
	return callMethod(ctx, c, api.BrowserTabAction, opts, openCall)
}

// tabAt is the id of the tab at index in the browser's list.
func tabAt(ctx context.Context, c *client, index *int) (string, error) {
	if index == nil {
		return "", fmt.Errorf("an index is required to select or close a tab; the tabs list gives it")
	}
	tabs, err := browserTabs(ctx, c)
	if err != nil {
		return "", err
	}
	if *index < 0 || *index >= len(tabs) {
		return "", fmt.Errorf("no tab at index %d; the browser has %d", *index, len(tabs))
	}
	return tabs[*index].ID, nil
}

func browserTabs(ctx context.Context, c *client) ([]api.BrowserTab, error) {
	id, err := c.browserOwner()
	if err != nil {
		return nil, err
	}
	return callMethod(ctx, c, api.BrowserTabs, api.SessionIDOptions{ID: id}, shortCall)
}
