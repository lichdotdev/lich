package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type AppUpdateStatus = api.AppUpdateStatus

type PluginStatus = api.PluginStatus

type PluginProfiles = api.PluginProfiles

type PluginProfileFailure = api.PluginProfileFailure

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
