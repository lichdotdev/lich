package api

var (
	ProvidersDetect      = define[None, []DetectedProvider]("providers.Detect")
	ProvidersOnPath      = define[ProviderIDOptions, []ProviderBinary]("providers.OnPath")
	ProvidersRefreshPath = define[None, None]("providers.RefreshPath")
	ProvidersVerify      = define[BinaryOptions, BinaryCheck]("providers.Verify")
	ProvidersVersion     = define[BinaryOptions, string]("providers.Version")
)

// The Status of a BinaryCheck that passed, and of one with nothing to check;
// every other value is a failure.
const (
	CheckOK    = "ok"
	CheckEmpty = "empty"
)

// Failed reports a check to show as a problem.
func (b BinaryCheck) Failed() bool { return b.Status != CheckOK && b.Status != CheckEmpty }

// ProviderIDOptions names a provider by its id ("claude", "codex").
type ProviderIDOptions struct {
	ID string `json:"id"`
}

// BinaryOptions names an executable as a session's spawn would resolve it.
type BinaryOptions struct {
	Bin string `json:"bin"`
}

// DetectedProvider reports a provider and whether a binary a session could spawn was
// found — on PATH, or at the path the user configured.
// Binary is the executable name a session spawns (DefaultBinary), which the
// settings screen needs even when nothing was found: a provider id is not its
// command — Antigravity's is `agy`.
// Docs is carried on every entry, installed or not: the row that has somewhere
// to send the user is exactly the one that found nothing.
// Source is SourcePath or SourceSetting, empty when nothing was found.
type DetectedProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Binary    string `json:"binary"`
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	Source    string `json:"source"`
	Docs      string `json:"docs"`
}

// ProviderBinary is one executable a provider could be started from, and the version it
// reports, empty when it printed none lich could read.
type ProviderBinary struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// BinaryCheck is what a configured binary resolves to. Path is the absolute
// executable a session would spawn, empty for every status but CheckOK.
type BinaryCheck struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}
