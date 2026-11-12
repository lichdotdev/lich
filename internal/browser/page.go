package browser

import (
	"context"
	"errors"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
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
// stamped (data-lich-idx); selector is a CSS query; x/y are pixels of the
// last screenshot's image, the viewport's CSS pixels scaled by shotScale.
// Coordinates win when set, then selector, then index.
type Target struct {
	Index    *int     `json:"index,omitempty"`
	Selector string   `json:"selector,omitempty"`
	X        *float64 `json:"x,omitempty"`
	Y        *float64 `json:"y,omitempty"`
	// shotW and shotH are the viewport of the agent's last screenshot, the
	// one x and y were read off; zero before it took one.
	shotW, shotH int
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
	// Error is the page script's refusal of a within selector.
	Error string `json:"error,omitempty"`
	// InteractiveSame is the service's: Interactive is left out as the list
	// the agent last heard.
	InteractiveSame bool `json:"-"`
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
	// Info reads the page, or, when within is not empty, the subtree of the
	// first element that CSS selector matches. A selector that matches nothing
	// or is invalid is an error that leaves the browser as it is.
	Info(ctx context.Context, within string) (PageInfo, error)
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
	// Screenshot writes the viewport, at shotScale image pixels per CSS
	// pixel, and returns the viewport's size in CSS pixels.
	Screenshot(ctx context.Context, dest string) (w, h int, err error)
	// AnnotatedScreenshot is Screenshot after a fresh Info read, with each
	// element's index drawn on it at its box.
	AnnotatedScreenshot(ctx context.Context, dest string) (w, h int, err error)
	Reload(ctx context.Context) error
	Back(ctx context.Context) error
	Forward(ctx context.Context) error
	// Scroll turns the wheel by dy CSS pixels at at, or at the viewport's
	// middle when at is nil, and returns where it turned it.
	Scroll(ctx context.Context, at *point, dy int) (x, y float64, err error)
	Focus(ctx context.Context) error
	// Eval runs script in the page, awaiting a promise it returns, and
	// returns its value: a string as it is, anything else as JSON. A script
	// that throws is an error carrying the exception's text.
	Eval(ctx context.Context, script string) (string, error)
	// Cookies and SetCookies carry a browser's cookies, session ones included,
	// into the process that replaces it.
	Cookies(ctx context.Context) ([]*network.CookieParam, error)
	SetCookies(ctx context.Context, cookies []*network.CookieParam) error
	// Listen hands the page what to call with each screencast frame (a tab's
	// id and its base64 JPEG), with its URL and title whenever they change, with each step of a
	// download it starts, with each error its console shows, and with each
	// JavaScript dialog it opens, then nil once that dialog is closed, and
	// with its tabs whenever one opens, closes, comes to the front or changes
	// its URL or title.
	Listen(onFrame func(tab, data string), onInfo func(url, title string), onDownload func(guid string, d api.BrowserDownload), onConsoleError func(text string), onDialog func(*api.BrowserDialog), onTabs func([]api.BrowserTab))
	// OnReload hands the page what to call with true when its front tab starts
	// a reload and false once the reloaded document is ready.
	OnReload(onReload func(started bool))
	// NewTab opens a blank tab in front; SelectTab brings the tab id to the
	// front; CloseTab closes it, unless it is the last one. Every other call
	// acts on the front tab.
	NewTab(ctx context.Context) error
	SelectTab(ctx context.Context, id string) error
	CloseTab(ctx context.Context, id string) error
	// AnswerDialog answers the open JavaScript dialog: OK with text (a
	// prompt's answer) when accept, Cancel otherwise. Every other call that
	// runs the page's script or input fails with errDialogOpen while one is
	// open.
	AnswerDialog(ctx context.Context, accept bool, text string) error
	// OnFileChooser hands the page what to call when a file input asks for
	// files, in the input's own mode; the paths it answers fill the input,
	// and none leaves it as it was.
	OnFileChooser(pick func(page.FileChooserOpenedMode) ([]string, error))
	Screencast(ctx context.Context, v Viewport) error
	// Still is what the screencast shows, as the front tab's id and a base64
	// PNG.
	Still(ctx context.Context) (tab, data string, err error)
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
// window; false is headless, which draws at scale until it is closed. The
// directory is the caller's to delete.
type Launcher func(dir string, headed bool, scale float64) (Page, error)
