// Package browserwire is the wire between the backend and lich-browser, the
// process that holds every page (docs/lich-browser.md): one JSON object per
// line each way. Both sides import it, so a field moves in one place.
package browserwire

import "encoding/json"

// Version is what hello answers; a lich-browser answering another one was
// built from another commit and the backend does not start it.
const Version = 1

// Request is one call from the backend; ID pairs it with its Response.
type Request struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Message is a line from lich-browser: a Response when ID is set, otherwise
// an Event.
type Message struct {
	ID     int64           `json:"id,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
	Event  string          `json:"event,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Requests.
const (
	Hello     = "hello"
	Window    = "window"
	Open      = "open"
	Close     = "close"
	NewTab    = "newTab"
	SelectTab = "selectTab"
	CloseTab  = "closeTab"
	Navigate  = "navigate"
	Reload    = "reload"
	Back      = "back"
	Forward   = "forward"
	Eval      = "eval"
	Mouse     = "mouse"
	Key       = "key"
	Text      = "text"
	Snapshot  = "snapshot"
	Zoom      = "zoom"
	Size      = "size"
	Dialog    = "dialog"
	Files     = "files"
	Inspect   = "inspect"
	Clock     = "clock"
	PopOut    = "popOut"
	PopIn     = "popIn"
	Raise     = "raise"
	Draw      = "draw"
)

// Events.
const (
	EventTabs     = "tabs"
	EventLoad     = "load"
	EventRequest  = "request"
	EventConsole  = "console"
	EventDialog   = "dialog"
	EventFiles    = "files"
	EventDownload = "download"
	EventCrashed  = "crashed"
	EventClosed   = "closed"
)

type HelloResult struct {
	Version int `json:"version"`
}

type WindowParams struct {
	XID uint64 `json:"xid"`
}

// PageParams names the page a request acts on, at its front tab.
type PageParams struct {
	Page string `json:"page"`
}

type OpenParams struct {
	Page  string  `json:"page"`
	URL   string  `json:"url"`
	Scale float64 `json:"scale"`
	W     int     `json:"w"`
	H     int     `json:"h"`
	// Downloads is the directory the page's downloads land in.
	Downloads string `json:"downloads"`
}

type TabParams struct {
	Page string `json:"page"`
	Tab  string `json:"tab"`
}

type NavigateParams struct {
	Page string `json:"page"`
	URL  string `json:"url"`
}

type EvalParams struct {
	Page   string `json:"page"`
	Script string `json:"script"`
}

// EvalResult's JSON is empty for undefined.
type EvalResult struct {
	JSON string `json:"json"`
}

// Mouse kinds.
const (
	MouseMove  = "move"
	MouseDown  = "down"
	MouseUp    = "up"
	MouseWheel = "wheel"
)

// Key kinds.
const (
	KeyDown = "down"
	KeyUp   = "up"
)

// Mods is a bit set of the modifiers held.
type Mods uint8

const (
	ModShift Mods = 1 << iota
	ModCtrl
	ModAlt
	ModMeta
)

type MouseParams struct {
	Page string  `json:"page"`
	Kind string  `json:"kind"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	// Button is the DOM's: 0 left, 1 middle, 2 right.
	Button int     `json:"button,omitempty"`
	Clicks int     `json:"clicks,omitempty"`
	Mods   Mods    `json:"mods,omitempty"`
	DX     float64 `json:"dx,omitempty"`
	DY     float64 `json:"dy,omitempty"`
}

// KeyParams names a key the DOM way: Key is KeyboardEvent.key, Code is
// KeyboardEvent.code.
type KeyParams struct {
	Page string `json:"page"`
	Kind string `json:"kind"`
	Key  string `json:"key"`
	Code string `json:"code,omitempty"`
	Mods Mods   `json:"mods,omitempty"`
}

type TextParams struct {
	Page string `json:"page"`
	Text string `json:"text"`
}

// SnapshotParams' Bare leaves the window's layer out of the shot.
type SnapshotParams struct {
	Page string `json:"page"`
	Bare bool   `json:"bare,omitempty"`
}

// LayerID is the id of the element the window's draw scripts draw its layer
// in, which a bare snapshot hides.
const LayerID = "lich-note-rings"

// DrawParams is a script drawing the window's layer over the front tab.
type DrawParams struct {
	Page   string `json:"page"`
	Script string `json:"script"`
}

type SnapshotResult struct {
	PNG []byte `json:"png"`
	W   int    `json:"w"`
	H   int    `json:"h"`
}

type ZoomParams struct {
	Page  string  `json:"page"`
	Scale float64 `json:"scale"`
}

type SizeParams struct {
	Page string `json:"page"`
	W    int    `json:"w"`
	H    int    `json:"h"`
}

type DialogParams struct {
	Page   string `json:"page"`
	Accept bool   `json:"accept"`
	Text   string `json:"text,omitempty"`
}

type FilesParams struct {
	Page  string   `json:"page"`
	Paths []string `json:"paths"`
}

type ClockResult struct {
	NS int64 `json:"ns"`
}

type Tab struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Loading bool   `json:"loading"`
	Front   bool   `json:"front"`
	// XID is the tab's native surface: an X window on Linux, an HWND on
	// Windows, a CAContext id on macOS.
	XID uint64 `json:"xid,omitempty"`
	// Icon is the tab's favicon, PNG.
	Icon []byte `json:"icon,omitempty"`
	// DevTools is WebKit's inspector of the tab before it, Front while the
	// pane shows it in that tab's place; requests still act on that tab.
	DevTools bool `json:"devtools,omitempty"`
}

type TabsEvent struct {
	Page string `json:"page"`
	Tabs []Tab  `json:"tabs"`
}

// Load steps.
const (
	LoadStarted   = "started"
	LoadCommitted = "committed"
	LoadFinished  = "finished"
	LoadFailed    = "failed"
)

type LoadEvent struct {
	Page string `json:"page"`
	Tab  string `json:"tab"`
	Step string `json:"step"`
	URL  string `json:"url"`
	At   int64  `json:"at"`
	// Error is WebKit's text for a failed load.
	Error string `json:"error,omitempty"`
	// Download is a navigation that turned into a download.
	Download bool `json:"download,omitempty"`
}

// Request types.
const (
	RequestFetch      = "fetch"
	RequestXHR        = "xhr"
	RequestDocument   = "document"
	RequestScript     = "script"
	RequestStylesheet = "stylesheet"
	RequestImage      = "image"
	RequestFont       = "font"
	RequestMedia      = "media"
	RequestOther      = "other"
)

// RequestEvent is a request of the front tab starting, or finishing however
// it ended: Finished names the ID that started.
type RequestEvent struct {
	Page     string `json:"page"`
	Tab      string `json:"tab"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Started  bool   `json:"started,omitempty"`
	Finished bool   `json:"finished,omitempty"`
	At       int64  `json:"at"`
}

type ConsoleEvent struct {
	Page string `json:"page"`
	Text string `json:"text"`
}

// Dialog types.
const (
	DialogAlert        = "alert"
	DialogConfirm      = "confirm"
	DialogPrompt       = "prompt"
	DialogBeforeUnload = "beforeunload"
)

type JSDialog struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Default string `json:"default,omitempty"`
}

// DialogEvent's Dialog is nil once the dialog closed.
type DialogEvent struct {
	Page   string    `json:"page"`
	Dialog *JSDialog `json:"dialog"`
}

type FilesEvent struct {
	Page     string `json:"page"`
	Multiple bool   `json:"multiple"`
}

// Download states.
const (
	DownloadInProgress = "inProgress"
	DownloadCompleted  = "completed"
	DownloadCanceled   = "canceled"
)

type DownloadEvent struct {
	Page     string `json:"page"`
	GUID     string `json:"guid"`
	URL      string `json:"url"`
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
	State    string `json:"state"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
}
