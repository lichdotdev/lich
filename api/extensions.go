package api

import "encoding/json"

var (
	ExtensionsList = define[None, []Extension]("extensions.List")
	ExtensionsSend = define[ExtensionSendOptions, None]("extensions.Send")
	// ExtensionsRestart starts an extension again, the one way back from failed.
	ExtensionsRestart = define[ExtensionIDOptions, None]("extensions.Restart")
	// ExtensionsDir is the directory lich reads extensions from.
	ExtensionsDir = define[None, string]("extensions.Dir")
	// ExtensionsPrepareFromGit downloads the latest release of a GitHub
	// repository's extension, checks and stages it, and returns what it is;
	// nothing runs until CommitInstall.
	ExtensionsPrepareFromGit = define[ExtensionGitInstallOptions, ExtensionInstallPreview]("extensions.PrepareFromGit")
	// ExtensionsPrepareFromDir stages a copy of a folder on disk holding an
	// extension, as PrepareFromGit does.
	ExtensionsPrepareFromDir = define[ExtensionDirInstallOptions, ExtensionInstallPreview]("extensions.PrepareFromDir")
	// ExtensionsCommitInstall moves a staged extension into place and starts it.
	// A token commits once.
	ExtensionsCommitInstall = define[ExtensionInstallTokenOptions, Extension]("extensions.CommitInstall")
	// ExtensionsDiscardInstall deletes a staged extension.
	ExtensionsDiscardInstall = define[ExtensionInstallTokenOptions, None]("extensions.DiscardInstall")
	// ExtensionsRemove stops an extension and deletes its directory. Its data
	// directory is kept.
	ExtensionsRemove = define[ExtensionIDOptions, None]("extensions.Remove")
	// ExtensionsDeleteData deletes the data directory of an extension that is
	// not installed, and refuses one that is.
	ExtensionsDeleteData = define[ExtensionIDOptions, None]("extensions.DeleteData")
)

// ExtensionGitInstallOptions names the repository to install from:
// https://github.com/<owner>/<repo>, with or without .git.
type ExtensionGitInstallOptions struct {
	URL string `json:"url"`
}

// ExtensionDirInstallOptions names a folder holding manifest.json and the
// executable at its root.
type ExtensionDirInstallOptions struct {
	Dir string `json:"dir"`
}

// ExtensionInstallTokenOptions names a staged install by the token its
// preview carries.
type ExtensionInstallTokenOptions struct {
	Token string `json:"token"`
}

// ExtensionInstallPreview is a staged install: what the person is asked to
// confirm, read from the staged copy's manifest. Token names the copy for
// CommitInstall or DiscardInstall, and expires when neither comes in time.
type ExtensionInstallPreview struct {
	Token       string   `json:"token"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Source      string   `json:"source"`
	Release     string   `json:"release,omitempty"`
	Permissions []string `json:"permissions"`
}

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

// ExtensionText is an extension's name and surface titles, by surface id, in
// one language of its catalogs. What it leaves out reads the manifest's.
type ExtensionText struct {
	Name     string            `json:"name,omitempty"`
	Surfaces map[string]string `json:"surfaces,omitempty"`
}

// Extension is one directory under lich's extensions directory and where its
// process is. Name and Surfaces come from its manifest, so they are the
// extension's own words; Text holds their translations, by the tag of each
// folder of the extension's locales. Source is the repository or folder it
// was installed from and Release the repository's tag, both empty for one
// placed by hand.
type Extension struct {
	ID       string                   `json:"id"`
	Name     string                   `json:"name"`
	Version  string                   `json:"version"`
	Status   string                   `json:"status"`
	Error    string                   `json:"error,omitempty"`
	Dir      string                   `json:"dir"`
	Surfaces []ExtensionSurface       `json:"surfaces"`
	Source   string                   `json:"source,omitempty"`
	Release  string                   `json:"release,omitempty"`
	Text     map[string]ExtensionText `json:"text,omitempty"`
	// Permissions is what the manifest lists, the extension.Permission names
	// its calls and subscriptions may use.
	Permissions []string `json:"permissions"`
}

// ExtensionSendOptions writes Message, one line of the extension contract, to
// the process of extension ID. Only event, surface.opened, surface.closed and
// language are accepted.
type ExtensionSendOptions struct {
	ID      string          `json:"id"`
	Message json.RawMessage `json:"message"`
}

// ExtensionMessage is a line extension ExtensionID wrote and lich accepted.
type ExtensionMessage struct {
	ExtensionID string          `json:"extensionId"`
	Message     json.RawMessage `json:"message"`
}
