package browser

import (
	"context"
	"errors"

	"github.com/chromedp/cdproto/network"
	"github.com/lichdotdev/lich/api"
)

var errUnknownInput = errors.New("unknown input type")

// errDialogOpen is a page call that waits on an open JavaScript dialog: the
// page runs no script and takes no input until it is answered.
var errDialogOpen = errors.New("the page holds a JavaScript dialog open")

// Viewport is the pane a page is drawn into: W and H in CSS pixels, Scale the
// device pixels per CSS pixel.
type Viewport struct {
	W, H  int
	Scale float64
}

// Handle is what Open and OpenVisible return: the id later calls take, and
// enough for a card or a tool result to say which window it is.
type Handle struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Owner  string `json:"owner"`
	Headed bool   `json:"headed"`
}

// Target picks an element for click and type. Index is the one page-info
// stamped (data-lich-idx); selector is a CSS query; x/y are viewport
// coordinates. Coordinates win when set, then selector, then index.
type Target struct {
	Index    *int     `json:"index,omitempty"`
	Selector string   `json:"selector,omitempty"`
	X        *float64 `json:"x,omitempty"`
	Y        *float64 `json:"y,omitempty"`
}

func (t Target) empty() bool {
	return t.Index == nil && t.Selector == "" && (t.X == nil || t.Y == nil)
}

// Rect is an element's viewport box, integer pixels.
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Interactive is one clickable or fillable node from page-info. It never
// carries a control's value — filled/checked say that typing landed without
// putting the content in the transcript.
type Interactive struct {
	Index   int    `json:"index"`
	Tag     string `json:"tag"`
	Type    string `json:"type,omitempty"`
	Role    string `json:"role,omitempty"`
	Text    string `json:"text"`
	Href    string `json:"href,omitempty"`
	Rect    Rect   `json:"rect"`
	Filled  *bool  `json:"filled,omitempty"`
	Checked *bool  `json:"checked,omitempty"`
}

// PageInfo is what the agent reads instead of a screenshot: the URL, the
// visible text, and the interactive nodes it can click or type into.
type PageInfo struct {
	URL         string        `json:"url"`
	Title       string        `json:"title"`
	Text        string        `json:"text"`
	Interactive []Interactive `json:"interactive"`
	Truncated   bool          `json:"truncated"`
	// Downloads and the console errors are the service's to fill, never the
	// page script's.
	Downloads         []api.BrowserDownload `json:"-"`
	ConsoleErrors     []string              `json:"-"`
	ConsoleErrorsMore int                   `json:"-"`
	Dialogs           []api.BrowserDialog   `json:"-"`
}

// Page is one Chromium tab the service drives. chrome.go is the real one;
// tests inject a fake.
type Page interface {
	Navigate(ctx context.Context, url string) error
	Info(ctx context.Context) (PageInfo, error)
	// HasText says whether text is in the page's visible text, the whole
	// document's and not only the viewport's.
	HasText(ctx context.Context, text string) (bool, error)
	// Settle waits until a navigation the last action may have started has
	// its document, bounded by domSettle.
	Settle(ctx context.Context) error
	// Click returns where it clicked, in CSS pixels.
	Click(ctx context.Context, t Target) (x, y float64, err error)
	Type(ctx context.Context, text string, clear bool, t Target) error
	Press(ctx context.Context, key string, t Target) error
	Screenshot(ctx context.Context, dest string) error
	Reload(ctx context.Context) error
	Back(ctx context.Context) error
	Forward(ctx context.Context) error
	// Scroll turns the wheel by dy CSS pixels at at, or at the viewport's
	// middle when at is nil, and returns where it turned it.
	Scroll(ctx context.Context, at *point, dy int) (x, y float64, err error)
	Focus(ctx context.Context) error
	// Cookies and SetCookies carry a browser's cookies, session ones included,
	// into the process that replaces it.
	Cookies(ctx context.Context) ([]*network.CookieParam, error)
	SetCookies(ctx context.Context, cookies []*network.CookieParam) error
	// Listen hands the page what to call with each screencast frame (base64
	// JPEG), with its URL and title whenever they change, with each step of a
	// download it starts, with each error its console shows, and with each
	// JavaScript dialog it opens, then nil once that dialog is closed.
	Listen(onFrame func(data string), onInfo func(url, title string), onDownload func(guid string, d api.BrowserDownload), onConsoleError func(text string), onDialog func(*api.BrowserDialog))
	// AnswerDialog answers the open JavaScript dialog: OK with text (a
	// prompt's answer) when accept, Cancel otherwise. Every other call that
	// runs the page's script or input fails with errDialogOpen while one is
	// open.
	AnswerDialog(ctx context.Context, accept bool, text string) error
	Screencast(ctx context.Context, v Viewport) error
	StopScreencast(ctx context.Context) error
	Resize(ctx context.Context, v Viewport) error
	Mouse(ctx context.Context, m api.BrowserMouseOptions) error
	Key(ctx context.Context, k api.BrowserKeyOptions) error
	InsertText(ctx context.Context, text string) error
	// Selection is the text selected in the page, "" in a password field.
	Selection(ctx context.Context) (string, error)
	// ElementAt is the element under x,y, zero when the point holds none;
	// Locate is the box of the element each selector finds, zero for none,
	// after scrolling reveal's element into view when reveal is set.
	ElementAt(ctx context.Context, x, y float64) (api.BrowserElement, error)
	Locate(ctx context.Context, selectors []string, reveal string) ([]api.BrowserRect, error)
	Alive() bool
	Close() error
}

// Launcher starts a Chromium with its own user-data-dir. headed is a real
// window; false is headless. The directory is the caller's to delete.
type Launcher func(dir string, headed bool) (Page, error)
