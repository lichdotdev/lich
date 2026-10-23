package api

var (
	AgentPluginStatus  = define[None, []PluginStatus]("agentplugin.Status")
	AgentPluginInstall = define[PluginProviderOptions, None]("agentplugin.Install")
	AgentPluginUpdate  = define[PluginProviderOptions, None]("agentplugin.Update")
)

// PluginProviderOptions names the provider whose lich plugin is installed or
// updated.
type PluginProviderOptions struct {
	Provider string `json:"provider"`
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
}
