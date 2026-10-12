package lichclient

import "context"

// AppUpdateStatus mirrors appupdate.Status: whether a newer lich exists. A
// failed release lookup is an answer with everything empty, not an error.
type AppUpdateStatus struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	CanSelfApply    bool   `json:"canSelfApply"`
	ReleaseURL      string `json:"releaseUrl"`
	InstallCommand  string `json:"installCommand"`
}

// PluginStatus mirrors agentplugin.Status: one provider CLI's lich plugin.
type PluginStatus struct {
	Provider         string `json:"provider"`
	Name             string `json:"name"`
	Available        bool   `json:"available"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	LatestVersion    string `json:"latestVersion"`
	UpdateAvailable  bool   `json:"updateAvailable"`
	Compatible       bool   `json:"compatible"`
}

// PatchNotesGroup mirrors patchnotes.Group; its items keep their markdown
// bold and code markers.
type PatchNotesGroup struct {
	Label string   `json:"label"`
	Items []string `json:"items"`
}

// PatchNotesHighlight mirrors patchnotes.Highlight: Kind is "important",
// "warning" or "note".
type PatchNotesHighlight struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// PatchNotes mirrors patchnotes.Notes. Groups is nil, not empty, when no
// changelog section matches the running build.
type PatchNotes struct {
	Version    string                `json:"version"`
	Highlights []PatchNotesHighlight `json:"highlights"`
	Groups     []PatchNotesGroup     `json:"groups"`
}

// AppUpdateStatus looks up the latest release.
func (c *Client) AppUpdateStatus(ctx context.Context) (AppUpdateStatus, error) {
	var s AppUpdateStatus
	return s, c.Call(ctx, "appupdate.Status", &s)
}

// PatchNotes returns the running build's changelog section.
func (c *Client) PatchNotes(ctx context.Context) (PatchNotes, error) {
	var n PatchNotes
	return n, c.Call(ctx, "patchnotes.Current", &n)
}

// PluginStatuses lists the lich plugin's state in every provider that can run
// it, in display order.
func (c *Client) PluginStatuses(ctx context.Context) ([]PluginStatus, error) {
	var statuses []PluginStatus
	return statuses, c.Call(ctx, "agentplugin.Status", &statuses)
}

// InstallPlugin installs the lich plugin into provider's CLI.
func (c *Client) InstallPlugin(ctx context.Context, provider string) error {
	return c.Call(ctx, "agentplugin.Install", nil, provider)
}

// UpdatePlugin moves provider's installed lich plugin to the release this lich
// is compatible with.
func (c *Client) UpdatePlugin(ctx context.Context, provider string) error {
	return c.Call(ctx, "agentplugin.Update", nil, provider)
}
