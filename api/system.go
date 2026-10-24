package api

var (
	SystemShowWindow         = define[ShowWindowOptions, None]("system.ShowWindow")
	SystemQuit               = define[QuitOptions, None]("system.Quit")
	SystemAttachNative       = define[AttachNativeOptions, None]("system.AttachNative")
	SystemDiagnostics        = define[None, Diagnostics]("system.Diagnostics")
	SystemNotify             = define[NotifyOptions, None]("system.Notify")
	SystemOpenExternal       = define[OpenExternalOptions, None]("system.OpenExternal")
	SystemOpenFile           = define[OpenFileOptions, None]("system.OpenFile")
	SystemOpenFolder         = define[OpenFolderOptions, None]("system.OpenFolder")
	SystemOpenFolderInEditor = define[OpenFolderOptions, string]("system.OpenFolderInEditor")
	SystemRevealLog          = define[None, None]("system.RevealLog")
	SystemSandboxBackend     = define[None, string]("system.SandboxBackend")
	SystemSSHAgentKeys       = define[None, []string]("system.SSHAgentKeys")
	SystemTakeUncleanExit    = define[None, bool]("system.TakeUncleanExit")
	TraySetLabels            = define[TrayLabels, None]("tray.SetLabels")
)

// ShowWindowOptions is one ShowWindow call. It carries nothing yet; it is an
// object so a newer launch can still reach an older running lich the day it
// does (the CLI's options-object rule).
type ShowWindowOptions struct{}

// QuitOptions is one Quit call, an object for the reason ShowWindowOptions is.
type QuitOptions struct{}

// NotifyOptions is a desktop notification: its summary line and the detail
// under it.
type NotifyOptions struct {
	Summary string `json:"summary"`
	Detail  string `json:"detail"`
}

// OpenExternalOptions names the link to open in the system's browser.
type OpenExternalOptions struct {
	URL string `json:"url"`
}

// OpenFileOptions names a file by its checkout and its path inside it.
type OpenFileOptions struct {
	Dir string `json:"dir"`
	Rel string `json:"rel"`
}

// OpenFolderOptions names a folder to open.
type OpenFolderOptions struct {
	Dir string `json:"dir"`
}

// AttachNativeOptions is the native window announcing itself.
type AttachNativeOptions struct {
	// Executable is the native window's own binary, the one started again when
	// lich is asked for its window after that window closed.
	Executable string `json:"executable"`
}

// Diagnostics is what a bug report opens with: the three facts that decide
// whether a report is actionable, and the log path the user attaches.
type Diagnostics struct {
	Version  string `json:"version"`
	Platform string `json:"platform"`
	LogPath  string `json:"logPath"`
}

// TrayLabels are the tray menu's words, in the interface language. Running and
// ServicesRunning carry a {count} placeholder and are written so they need no
// plural form.
type TrayLabels struct {
	Show            string `json:"show"`
	Running         string `json:"running"`
	Quit            string `json:"quit"`
	ServicesRunning string `json:"servicesRunning"`
	ShowOutput      string `json:"showOutput"`
	Restart         string `json:"restart"`
	Stop            string `json:"stop"`
	StopAll         string `json:"stopAll"`
}
