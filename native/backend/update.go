package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type AppUpdateStatus = api.AppUpdateStatus

type PluginStatus = api.PluginStatus

// PluginIncompatibleEvent is raised when a session's hooks report from a
// lich plugin release outside the range this lich speaks.
const PluginIncompatibleEvent = "plugin-incompatible"

// PluginProfilesEvent is the event a pass over a provider's profiles reports
// on once Personal's lich plugin moved (agentplugin.ProfilesEventName).
const PluginProfilesEvent = "lich-plugin-profiles"

// PluginProfiles mirrors agentplugin.ProfilesEvent: the release a pass put
// into a provider's profiles and the ones it could not. Name is the provider's
// display name.
type PluginProfiles struct {
	Provider string                 `json:"provider"`
	Name     string                 `json:"name"`
	Version  string                 `json:"version"`
	Failures []PluginProfileFailure `json:"failures"`
}

// PluginProfileFailure mirrors agentplugin.ProfileFailure.
type PluginProfileFailure struct {
	Profile string `json:"profile"`
	Name    string `json:"name"`
	Error   string `json:"error"`
}

type PatchNotesGroup = api.PatchNotesGroup

type PatchNotesHighlight = api.PatchNotesHighlight

type PatchNotes = api.PatchNotes

// AppUpdateStatus looks up the latest release.
func (c *Client) AppUpdateStatus(ctx context.Context) (AppUpdateStatus, error) {
	return callMethod(ctx, c, api.AppUpdateGetStatus, api.None{})
}

// PatchNotes returns the running build's changelog section in locale, the
// interface language's tag; the backend falls back to English when the release
// has no translation into it.
func (c *Client) PatchNotes(ctx context.Context, locale string) (PatchNotes, error) {
	return callMethod(ctx, c, api.PatchNotesCurrent, api.PatchNotesOptions{Locale: locale})
}

// PluginStatuses lists the lich plugin's state in every provider that can run
// it, in display order.
func (c *Client) PluginStatuses(ctx context.Context) ([]PluginStatus, error) {
	return callMethod(ctx, c, api.AgentPluginStatus, api.None{})
}

// InstallPlugin installs the lich plugin into provider's CLI.
func (c *Client) InstallPlugin(ctx context.Context, provider string) error {
	_, err := callMethod(ctx, c, api.AgentPluginInstall, api.PluginProviderOptions{Provider: provider})
	return err
}

// UpdatePlugin moves provider's installed lich plugin to the release this lich
// is compatible with.
func (c *Client) UpdatePlugin(ctx context.Context, provider string) error {
	_, err := callMethod(ctx, c, api.AgentPluginUpdate, api.PluginProviderOptions{Provider: provider})
	return err
}
