package api

var (
	BrowserShow = define[SessionIDOptions, BrowserHandle]("browser.Show")
	BrowserOpen = define[BrowserOpenOptions, BrowserHandle]("browser.Open")
	BrowserInfo = define[SessionIDOptions, BrowserPageInfo]("browser.Info")
	// Click and Press answer with the page as it stands after them, so an
	// agent reads what its action did without a browser.Info of its own. Type
	// does not: typing into a field seldom changes the page, and a page per
	// field filled cost a fifth of a session's page text (measured
	// 2026-10-26).
	BrowserClick      = define[BrowserClickOptions, BrowserPageInfo]("browser.Click")
	BrowserType       = define[BrowserTypeOptions, None]("browser.Type")
	BrowserPress      = define[BrowserPressOptions, BrowserPageInfo]("browser.Press")
	BrowserScreenshot = define[BrowserScreenshotOptions, string]("browser.Screenshot")
	BrowserNavigate   = define[BrowserOpenOptions, BrowserHandle]("browser.Navigate")
	BrowserReload     = define[SessionIDOptions, None]("browser.Reload")
	BrowserBack       = define[SessionIDOptions, None]("browser.Back")
	BrowserForward    = define[SessionIDOptions, None]("browser.Forward")
	BrowserScroll     = define[BrowserScrollOptions, None]("browser.Scroll")
	BrowserList       = define[SessionIDOptions, []BrowserHandle]("browser.List")
	BrowserClose      = define[SessionIDOptions, None]("browser.Close")

	// The window's side: a pane that shows a session's page and passes the
	// person's input to it, beside the agent driving the same page.
	BrowserRunning = define[None, []BrowserState]("browser.Running")
	BrowserAttach  = define[BrowserViewOptions, BrowserHandle]("browser.Attach")
	BrowserDetach  = define[SessionIDOptions, None]("browser.Detach")
	BrowserResize  = define[BrowserViewOptions, None]("browser.Resize")
	BrowserMouse   = define[BrowserMouseOptions, None]("browser.Mouse")
	BrowserKey     = define[BrowserKeyOptions, None]("browser.Key")
	BrowserText    = define[BrowserTextOptions, None]("browser.Text")
	BrowserNav     = define[BrowserNavOptions, None]("browser.Nav")
	// BrowserSelection is the text selected in the page, which the pane
	// copies to the system clipboard: the headless browser's own clipboard
	// is not the person's.
	BrowserSelection = define[SessionIDOptions, string]("browser.Selection")
	// BrowserElementAt is the element under a point of the page, for a note
	// the person leaves on it; BrowserLocate is where noted elements stand
	// now, after the page scrolled or changed.
	BrowserElementAt = define[BrowserPointOptions, BrowserElement]("browser.ElementAt")
	BrowserLocate    = define[BrowserLocateOptions, []BrowserRect]("browser.Locate")
)

var (
	// BrowserStateEvent is emitted whenever a session's browser starts, closes,
	// moves to a window or loads another page, so the one store the window
	// keeps of them stays true.
	BrowserStateEvent = defineEvent[BrowserState]("browser-state")

	// BrowserFrameEvents carries a base64 JPEG of a session's page while a pane
	// shows it: per session, like terminal output, so only that pane listens.
	BrowserFrameEvents = defineEventFamily[string]("browser:frame:")

	// BrowserAgentEvent is emitted each time the agent acts on its page, so the
	// pane can show that the page moved because of it, and where it clicked.
	BrowserAgentEvent = defineEvent[BrowserAgentAct]("browser-agent")

	// BrowserDownloadEvent is emitted when a file a session's page downloaded
	// is complete on disk, so the person learns where it went.
	BrowserDownloadEvent = defineEvent[BrowserDownload]("browser-download")
)

// Download states, Chrome's own (Browser.downloadProgress).
const (
	DownloadInProgress = "inProgress"
	DownloadCompleted  = "completed"
	DownloadCanceled   = "canceled"
)

// BrowserDownload is one file a session's page downloaded: Path is where it
// lands in the user's Downloads folder, under the name it keeps once
// complete; Bytes is what arrived so far.
type BrowserDownload struct {
	Owner string `json:"owner,omitempty"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
	State string `json:"state"`
}

// BrowserState is a session's browser as the window shows it. Open false means
// it is gone; ByAgent says the agent started it rather than the window.
type BrowserState struct {
	Owner   string `json:"owner"`
	Open    bool   `json:"open"`
	Headed  bool   `json:"headed"`
	ByAgent bool   `json:"byAgent"`
	URL     string `json:"url"`
	Title   string `json:"title"`
}

// BrowserAgentAct is one agent action on its page. Action is click, type,
// press or scroll at a point; back, forward, reload or navigate, which the
// pane's toolbar does; or "" for one that changes nothing on screen (reading
// the page, a screenshot). X and Y are where it acted, in CSS pixels, when it
// acted at a point; DY is a scroll's distance, positive down; URL is where a
// navigate went.
type BrowserAgentAct struct {
	Owner  string   `json:"owner"`
	Action string   `json:"action,omitempty"`
	X      *float64 `json:"x,omitempty"`
	Y      *float64 `json:"y,omitempty"`
	DY     int      `json:"dy,omitempty"`
	URL    string   `json:"url,omitempty"`
}

// BrowserViewOptions sizes a session's page to the pane showing it: W and H in
// CSS pixels, Scale the device pixels per CSS pixel.
type BrowserViewOptions struct {
	ID    string  `json:"id"`
	W     int     `json:"w"`
	H     int     `json:"h"`
	Scale float64 `json:"scale"`
}

// BrowserMouseOptions is one pointer event over the pane, in CSS pixels.
// Type is press, release, move or wheel; Button is left, middle, right or
// none; DX and DY are a wheel's scroll in CSS pixels.
type BrowserMouseOptions struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Button    string  `json:"button,omitempty"`
	Clicks    int     `json:"clicks,omitempty"`
	DX        float64 `json:"dx,omitempty"`
	DY        float64 `json:"dy,omitempty"`
	Modifiers int     `json:"modifiers,omitempty"`
}

// BrowserKeyOptions is one key going down or up, for keys that do not type
// text (Enter, arrows, shortcuts): text goes through BrowserText. Key and Code
// are the DOM's KeyboardEvent.key and .code; Modifiers is CDP's bit set (Alt
// 1, Ctrl 2, Meta 4, Shift 8).
type BrowserKeyOptions struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Key       string `json:"key"`
	Code      string `json:"code,omitempty"`
	KeyCode   int    `json:"keyCode,omitempty"`
	Modifiers int    `json:"modifiers,omitempty"`
}

// BrowserNavOptions is the pane's toolbar: Action is back, forward, reload or
// go, and URL is where go loads.
type BrowserNavOptions struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	URL    string `json:"url,omitempty"`
}

// BrowserTextOptions types text into whatever has focus, the way an input
// method commits it.
type BrowserTextOptions struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// BrowserHandle is one session's browser: the id later calls take, and enough
// for a tool result to say which window it is. Downloads, on an open or a
// navigate, are what that load downloaded: a URL that answers with a file
// saves it and leaves the page where it was.
type BrowserHandle struct {
	ID        string            `json:"id"`
	URL       string            `json:"url"`
	Title     string            `json:"title"`
	Owner     string            `json:"owner"`
	Headed    bool              `json:"headed"`
	Downloads []BrowserDownload `json:"downloads,omitempty"`
}

// BrowserOpenOptions loads a URL in the session's browser, creating a headless
// one when the card has not opened a window yet.
type BrowserOpenOptions struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// BrowserTarget picks an element. Index is the one page-info stamped;
// selector is a CSS query; x/y are viewport coordinates. Coordinates win when
// set, then selector, then index.
type BrowserTarget struct {
	Index    *int     `json:"index,omitempty"`
	Selector string   `json:"selector,omitempty"`
	X        *float64 `json:"x,omitempty"`
	Y        *float64 `json:"y,omitempty"`
}

// BrowserClickOptions clicks in the session's browser.
type BrowserClickOptions struct {
	ID string `json:"id"`
	BrowserTarget
}

// BrowserTypeOptions types into the session's browser. An empty target types
// into whatever is focused.
type BrowserTypeOptions struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Clear bool   `json:"clear"`
	BrowserTarget
}

// BrowserPressOptions presses one named key. An empty target presses into
// whatever is focused.
type BrowserPressOptions struct {
	ID  string `json:"id"`
	Key string `json:"key"`
	BrowserTarget
}

// BrowserScreenshotOptions captures the page to Path, or to a file lich
// chooses when Path is empty.
type BrowserScreenshotOptions struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// BrowserScrollOptions scrolls the page by DY CSS pixels. Positive is down.
type BrowserScrollOptions struct {
	ID string `json:"id"`
	DY int    `json:"dy"`
}

// BrowserRect is an element's viewport box, integer pixels.
type BrowserRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// BrowserInteractive is one clickable or fillable node. It never carries a
// control's value: filled and checked say that typing landed.
type BrowserInteractive struct {
	Index   int         `json:"index"`
	Tag     string      `json:"tag"`
	Type    string      `json:"type,omitempty"`
	Role    string      `json:"role,omitempty"`
	Text    string      `json:"text"`
	Href    string      `json:"href,omitempty"`
	Rect    BrowserRect `json:"rect"`
	Filled  *bool       `json:"filled,omitempty"`
	Checked *bool       `json:"checked,omitempty"`
}

// BrowserPageInfo is what the agent reads instead of a screenshot.
type BrowserPageInfo struct {
	URL         string               `json:"url"`
	Title       string               `json:"title"`
	Text        string               `json:"text"`
	Interactive []BrowserInteractive `json:"interactive"`
	Truncated   bool                 `json:"truncated"`
	// Downloads are the ones the agent has not heard of in their current
	// state: what its last action saved, or one it saw start and that ended
	// since.
	Downloads []BrowserDownload `json:"downloads,omitempty"`
}

// BrowserPointOptions is a point of session ID's page, in CSS pixels.
type BrowserPointOptions struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// BrowserElement is the element under a point, as a note on it names it.
// Label is its tag and text (button "Buy now"), empty when the point holds
// none; Selector finds it again, in the form browser.Click takes. Component
// and Source are the React component that drew it and where, when the page
// is a React development build. HTML is its markup, shortened, with no
// field's value. URL is the page's, without query or fragment.
type BrowserElement struct {
	Label     string      `json:"label"`
	Selector  string      `json:"selector"`
	Component string      `json:"component,omitempty"`
	Source    string      `json:"source,omitempty"`
	HTML      string      `json:"html"`
	URL       string      `json:"url"`
	Box       BrowserRect `json:"box"`
}

// BrowserLocateOptions asks where the elements Selectors find stand in
// session ID's page; one gone from the page, or hidden by what it scrolls
// in, answers an empty box. Reveal, when set, is a selector whose element
// the page first scrolls into view unless it already shows whole.
type BrowserLocateOptions struct {
	ID        string   `json:"id"`
	Selectors []string `json:"selectors"`
	Reveal    string   `json:"reveal,omitempty"`
}
