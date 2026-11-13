package extension

// ToastForever as DurationMs keeps a toast up until it is dismissed.
const ToastForever = -1

var (
	Render       = kind[RenderData]("render")
	ShowToast    = kind[ToastData]("toast")
	DismissToast = kind[DismissToastData]("toast.dismiss")
	OpenExternal = kind[OpenExternalData]("openExternal")
	Clipboard    = kind[ClipboardData]("clipboard")
	Focus        = kind[FocusData]("focus")
	Footer       = kind[FooterData]("footer")

	FireEvent     = kind[EventData]("event")
	SurfaceOpened = kind[SurfaceData]("surface.opened")
	SurfaceClosed = kind[SurfaceData]("surface.closed")
	Language      = kind[LanguageData]("language")
	Context       = kind[ContextData]("context")
)

// RenderData replaces what a surface shows. Shortcuts are live while the
// surface is on screen. Path, for a dock or panel surface, is the checkout the
// tree was drawn for, as ContextData named it: the window draws it only while
// that checkout is the active one, and a placeholder until the tree for the
// active one arrives. An empty Path is drawn for every checkout.
type RenderData struct {
	Surface   string     `json:"surface"`
	Path      string     `json:"path,omitempty"`
	Root      Node       `json:"root"`
	Shortcuts []Shortcut `json:"shortcuts,omitempty"`
}

// FooterData sets the text of a dock or panel surface's footer item for the
// checkout at Path, a path ContextData named: the window shows it while that
// checkout is the active one. An empty Path sets it for every checkout without
// a text of its own, and an empty Text clears it, leaving the item its icon.
type FooterData struct {
	Surface string `json:"surface"`
	Path    string `json:"path,omitempty"`
	Text    string `json:"text,omitempty"`
}

// ContextData is what the window has active, sent to an extension with a dock
// or panel surface when it changes, when it starts, and when a window
// attaches. Path is the active session's checkout, or the project's own
// folder with no session, and is what FooterData and RenderData key on.
type ContextData struct {
	Session string `json:"session,omitempty"`
	Project string `json:"project,omitempty"`
	Path    string `json:"path,omitempty"`
}

// Shortcut calls Handler when Keys is pressed, spelled as "r", "shift+g" or
// "ctrl+k". While a field has focus only a chord with ctrl fires.
type Shortcut struct {
	Keys    string  `json:"keys"`
	Handler Handler `json:"handler"`
}

// OpenExternalData opens an http or https URL in the user's browser.
type OpenExternalData struct {
	URL string `json:"url"`
}

// ClipboardData replaces the clipboard's text.
type ClipboardData struct {
	Text string `json:"text"`
}

// FocusData gives the keyboard to the field under Key on Surface: an input,
// inputGroup, textarea or inlineEdit. It lands on the next frame that draws
// that field and is dropped when that frame has none, or when Surface is not
// the one on screen. Escape hands the keyboard back to the surface.
type FocusData struct {
	Surface string `json:"surface"`
	Key     string `json:"key"`
}

// SurfaceData names a surface the person opened or left. Answer
// surface.opened with a render: lich keeps only what reaches an open window,
// so a render sent before the surface opened may never be drawn. A process
// that starts while its surface is on screen is sent surface.opened again.
// Language is the tag of the interface's language, on surface.opened only.
type SurfaceData struct {
	Surface  string `json:"surface"`
	Language string `json:"language,omitempty"`
}

type ToastButton struct {
	Label   string  `json:"label"`
	OnPress Handler `json:"onPress,omitempty"`
}

type ToastLine struct {
	Icon Icon   `json:"icon,omitempty"`
	Text string `json:"text"`
}

// ToastProgress is a bar from 0 to 100, negative while the total is unknown.
type ToastProgress struct {
	Percent int `json:"percent"`
}

// ToastData raises a toast. A toast with the ID of one on screen replaces it;
// DurationMs 0 is the window's default.
type ToastData struct {
	ID          string         `json:"id,omitempty"`
	Variant     ToastVariant   `json:"variant,omitempty"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Action      *ToastButton   `json:"action,omitempty"`
	Cancel      *ToastButton   `json:"cancel,omitempty"`
	Secondary   *ToastButton   `json:"secondary,omitempty"`
	Icon        Icon           `json:"icon,omitempty"`
	IconColor   string         `json:"iconColor,omitempty"`
	Spin        bool           `json:"spin,omitempty"`
	Lines       []ToastLine    `json:"lines,omitempty"`
	Progress    *ToastProgress `json:"progress,omitempty"`
	DurationMs  int            `json:"durationMs,omitempty"`
	OnDismiss   Handler        `json:"onDismiss,omitempty"`
}

type DismissToastData struct {
	ID string `json:"id"`
}

// EventData is lich calling a Handler. The field that carries a value depends
// on the control: Text for fields and groups, Checked for switches, Index for
// tabs, Order for a sortable list's reorder, Width for a resize handle, Ratio
// for a split pane. A sortable list's move carries the moved item's Key, the
// Index it landed at in the receiving list and the key of the list it came
// From.
type EventData struct {
	Handler Handler  `json:"handler"`
	Text    string   `json:"text,omitempty"`
	Checked bool     `json:"checked,omitempty"`
	Index   int      `json:"index,omitempty"`
	Order   []string `json:"order,omitempty"`
	Width   float64  `json:"width,omitempty"`
	Ratio   float64  `json:"ratio,omitempty"`
	Key     string   `json:"key,omitempty"`
	From    string   `json:"from,omitempty"`
}
