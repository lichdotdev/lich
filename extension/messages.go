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

	FireEvent     = kind[EventData]("event")
	SurfaceOpened = kind[SurfaceData]("surface.opened")
	SurfaceClosed = kind[SurfaceData]("surface.closed")
)

// RenderData replaces what a surface shows. Shortcuts are live while the
// surface is on screen.
type RenderData struct {
	Surface   string     `json:"surface"`
	Root      Node       `json:"root"`
	Shortcuts []Shortcut `json:"shortcuts,omitempty"`
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
// so a render sent before the surface opened may never be drawn.
type SurfaceData struct {
	Surface string `json:"surface"`
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
// tabs, Order for a sortable list, Width for a resize handle, Ratio for a
// split pane.
type EventData struct {
	Handler Handler  `json:"handler"`
	Text    string   `json:"text,omitempty"`
	Checked bool     `json:"checked,omitempty"`
	Index   int      `json:"index,omitempty"`
	Order   []string `json:"order,omitempty"`
	Width   float64  `json:"width,omitempty"`
	Ratio   float64  `json:"ratio,omitempty"`
}
