package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// ShowBrowser moves session id's browser to its own Chromium window, or
// brings that window forward when it is there already.
func (c *Client) ShowBrowser(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.BrowserShow, api.SessionIDOptions{ID: id})
	return err
}

// CloseBrowser ends session id's browser.
func (c *Client) CloseBrowser(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.BrowserClose, api.SessionIDOptions{ID: id})
	return err
}

// KeepBrowser answers the question about session id's idle browser with
// keep.
func (c *Client) KeepBrowser(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.BrowserKeep, api.SessionIDOptions{ID: id})
	return err
}

// RunningBrowsers is every session browser the backend runs.
func (c *Client) RunningBrowsers(ctx context.Context) ([]api.BrowserState, error) {
	return callMethod(ctx, c, api.BrowserRunning, api.None{})
}

// AttachBrowser starts the frames of session id's page at the pane's size,
// starting a browser when the session has none.
func (c *Client) AttachBrowser(ctx context.Context, v api.BrowserViewOptions) error {
	_, err := callMethod(ctx, c, api.BrowserAttach, v)
	return err
}

// DetachBrowser stops the frames of session id's page.
func (c *Client) DetachBrowser(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.BrowserDetach, api.SessionIDOptions{ID: id})
	return err
}

// ResizeBrowser lays session id's page out at the pane's new size.
func (c *Client) SetBrowserDefaultView(ctx context.Context, v api.BrowserDefaultViewOptions) error {
	_, err := callMethod(ctx, c, api.BrowserDefaultView, v)
	return err
}

func (c *Client) ResizeBrowser(ctx context.Context, v api.BrowserViewOptions) error {
	_, err := callMethod(ctx, c, api.BrowserResize, v)
	return err
}

// BrowserMouse passes one pointer event to the page.
func (c *Client) BrowserMouse(ctx context.Context, m api.BrowserMouseOptions) error {
	_, err := callMethod(ctx, c, api.BrowserMouse, m)
	return err
}

// BrowserKey passes one key to the page.
func (c *Client) BrowserKey(ctx context.Context, k api.BrowserKeyOptions) error {
	_, err := callMethod(ctx, c, api.BrowserKey, k)
	return err
}

// BrowserText types committed text into the page.
func (c *Client) BrowserText(ctx context.Context, id, text string) error {
	_, err := callMethod(ctx, c, api.BrowserText, api.BrowserTextOptions{ID: id, Text: text})
	return err
}

// BrowserSelection is the text selected in session id's page.
func (c *Client) BrowserSelection(ctx context.Context, id string) (string, error) {
	return callMethod(ctx, c, api.BrowserSelection, api.SessionIDOptions{ID: id})
}

// BrowserNav runs the pane's toolbar: back, forward, reload, or go to url.
func (c *Client) BrowserNav(ctx context.Context, id, action, url string) error {
	_, err := callMethod(ctx, c, api.BrowserNav, api.BrowserNavOptions{ID: id, Action: action, URL: url})
	return err
}

// BrowserTabNav runs the pane's tabs: open a new one, or select or close tab.
func (c *Client) BrowserTabNav(ctx context.Context, id, action, tab string) error {
	_, err := callMethod(ctx, c, api.BrowserTabNav, api.BrowserTabOptions{ID: id, Action: action, Tab: tab})
	return err
}

// BrowserAnswer answers the JavaScript dialog session's page holds open.
func (c *Client) BrowserAnswer(ctx context.Context, o api.BrowserDialogOptions) error {
	_, err := callMethod(ctx, c, api.BrowserAnswer, o)
	return err
}

// BrowserElementAt is the element under x,y of session id's page.
func (c *Client) BrowserElementAt(ctx context.Context, id string, x, y float64) (api.BrowserElement, error) {
	return callMethod(ctx, c, api.BrowserElementAt, api.BrowserPointOptions{ID: id, X: x, Y: y})
}

// BrowserLocate is where the elements selectors find stand in session id's
// page, an empty box for one gone, after scrolling reveal's element into
// view when reveal is set.
func (c *Client) BrowserLocate(ctx context.Context, id string, selectors []string, reveal string) ([]api.BrowserRect, error) {
	return callMethod(ctx, c, api.BrowserLocate, api.BrowserLocateOptions{ID: id, Selectors: selectors, Reveal: reveal})
}
