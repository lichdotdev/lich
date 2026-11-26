package api

var (
	AgentPluginStatus  = define[None, []PluginStatus]("agentplugin.Status")
	AgentPluginProblem = define[None, PluginProblem]("agentplugin.Problem")
	AgentPluginRetry   = define[None, None]("agentplugin.Retry")
)

// PluginProblem is what lich could not do to give its sessions their plugin,
// the zero value when nothing failed. Bundle is the release lich could not
// unpack, which leaves Claude Code, Cursor CLI, opencode and oh-my-pi sessions
// without its hooks; Providers names each provider whose install failed. Error
// is the first failure's text.
type PluginProblem struct {
	Bundle    bool     `json:"bundle"`
	Providers []string `json:"providers"`
	Error     string   `json:"error"`
}

// PluginStatus is one provider's plugin state, reported to the window.
type PluginStatus struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	// Available is whether the provider's CLI is on PATH at all. A provider the
	// machine does not have is still listed — with nothing to install — so the
	// UI can say why it is not on offer.
	Available        bool   `json:"available"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	// LatestVersion is the newest release this lich is compatible with, not the
	// newest the plugin has published: it is the one an install or update writes.
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	// Compatible is false when the installed release is outside the range this
	// lich speaks (PluginVersionFloor, PluginVersionCeiling). Its reports still
	// land where they parse; UpdateAvailable then offers LatestVersion, a
	// downgrade included.
	Compatible bool `json:"compatible"`
	// Reregister is an update that moves no version: the hooks lich wrote for
	// this CLI predate the release of it installed now (a Crush upgraded past
	// the lifecycle events), and only writing them again picks the new events
	// up. UpdateAvailable is true beside it.
	Reregister bool `json:"reregister"`
}
