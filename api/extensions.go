package api

import "encoding/json"

var (
	ExtensionsList = define[None, []Extension]("extensions.List")
	ExtensionsSend = define[ExtensionSendOptions, None]("extensions.Send")
	// ExtensionsRestart starts an extension again, the one way back from failed.
	ExtensionsRestart = define[ExtensionIDOptions, None]("extensions.Restart")
)

// ExtensionIDOptions names one extension.
type ExtensionIDOptions struct {
	ID string `json:"id"`
}

// The states an Extension is in. Invalid never started, its Error says why;
// restarting is the wait after a crash; failed is lich having given up on it.
const (
	ExtensionInvalid    = "invalid"
	ExtensionRunning    = "running"
	ExtensionRestarting = "restarting"
	ExtensionFailed     = "failed"
	ExtensionStopped    = "stopped"
)

// ExtensionSurface is a full screen an extension declares.
type ExtensionSurface struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
}

// Extension is one directory under lich's extensions directory and where its
// process is. Name and Surfaces come from its manifest, so they are the
// extension's own words.
type Extension struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Version  string             `json:"version"`
	Status   string             `json:"status"`
	Error    string             `json:"error,omitempty"`
	Dir      string             `json:"dir"`
	Surfaces []ExtensionSurface `json:"surfaces"`
}

// ExtensionSendOptions writes Message, one line of the extension contract, to
// the process of extension ID. Only event, surface.opened and surface.closed
// are accepted.
type ExtensionSendOptions struct {
	ID      string          `json:"id"`
	Message json.RawMessage `json:"message"`
}

// ExtensionMessage is a line extension ExtensionID wrote and lich accepted.
type ExtensionMessage struct {
	ExtensionID string          `json:"extensionId"`
	Message     json.RawMessage `json:"message"`
}
