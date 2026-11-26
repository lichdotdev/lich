package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type AppUpdateStatus = api.AppUpdateStatus

type PluginStatus = api.PluginStatus

type PluginProblem = api.PluginProblem

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

// PluginProblem is what lich could not do to give its sessions their plugin,
// the zero value when nothing failed.
func (c *Client) PluginProblem(ctx context.Context) (PluginProblem, error) {
	return callMethod(ctx, c, api.AgentPluginProblem, api.None{})
}

// RetryPlugin runs lich's plugin setup again and returns once it ran.
func (c *Client) RetryPlugin(ctx context.Context) error {
	_, err := callMethod(ctx, c, api.AgentPluginRetry, api.None{})
	return err
}
