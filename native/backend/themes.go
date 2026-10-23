package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// The theme shapes are api's, the backend's own definition.
type (
	Theme             = api.Theme
	ThemeSelection    = api.ThemeSelection
	SyntaxStyle       = api.SyntaxStyle
	ThemeSource       = api.ThemeSource
	BrokenTheme       = api.BrokenTheme
	ThemeImportResult = api.ThemeImportResult
	ThemeGitResult    = api.ThemeGitResult
)

// ThemeSelectionEvent is themes.SelectionEventName: the stored selection
// changed, from this window, another, or `lich theme switch`.
const ThemeSelectionEvent = "theme-selection"

// ThemeSelection reads the stored selection; a theme never chosen reads
// "system".
func (c *Client) ThemeSelection(ctx context.Context) (ThemeSelection, error) {
	return callMethod(ctx, c, api.ThemesSelection, api.None{})
}

// SelectTheme stores id, "system" or an installed theme, as the selection.
func (c *Client) SelectTheme(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ThemesSelect, api.SelectThemeOptions{ID: id})
	return err
}

// SetFollowSystem stores whether the window draws the selected theme's pair
// while the OS asks for the pair's scheme.
func (c *Client) SetFollowSystem(ctx context.Context, on bool) error {
	_, err := callMethod(ctx, c, api.ThemesSetFollowSystem, api.FollowSystemOptions{On: on})
	return err
}

// Themes returns the bundled themes and the imported ones that load.
func (c *Client) Themes(ctx context.Context) ([]Theme, error) {
	return callMethod(ctx, c, api.ThemesList, api.None{})
}

// BrokenThemes returns the stored themes Themes leaves out, with why.
func (c *Client) BrokenThemes(ctx context.Context) ([]BrokenTheme, error) {
	return callMethod(ctx, c, api.ThemesListBroken, api.None{})
}

// ImportTheme imports the theme file at path.
func (c *Client) ImportTheme(ctx context.Context, path string, overwrite bool) (ThemeImportResult, error) {
	return callMethod(ctx, c, api.ThemesImport, api.ImportThemeOptions{Path: path, Overwrite: overwrite})
}

// InstallThemes clones url and installs every theme its manifest lists.
func (c *Client) InstallThemes(ctx context.Context, url string, overwrite bool) (ThemeGitResult, error) {
	return callMethod(ctx, c, api.ThemesInstallFromGit, api.InstallThemesOptions{URL: url, Overwrite: overwrite})
}

// UpdateTheme re-clones the repository theme id came from.
func (c *Client) UpdateTheme(ctx context.Context, id string) (ThemeGitResult, error) {
	return callMethod(ctx, c, api.ThemesUpdateFromGit, api.UpdateThemeOptions{ID: id})
}

// RemoveTheme deletes an imported theme; the backend refuses a bundled one.
func (c *Client) RemoveTheme(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ThemesRemove, api.RemoveThemeOptions{ID: id})
	return err
}

// SaveThemeTemplate writes the starter theme to path.
func (c *Client) SaveThemeTemplate(ctx context.Context, path string) error {
	_, err := callMethod(ctx, c, api.ThemesSaveTemplate, api.SaveThemeTemplateOptions{Path: path})
	return err
}

// PickFile opens the backend's file picker; "" is a cancelled one.
func (c *Client) PickFile(ctx context.Context, title string) (string, error) {
	var path string
	return path, c.Call(ctx, "project.PickFile", &path, title)
}

// PickSaveFile opens the backend's save picker on defaultName; "" is a
// cancelled one.
func (c *Client) PickSaveFile(ctx context.Context, title, defaultName string) (string, error) {
	var path string
	return path, c.Call(ctx, "project.PickSaveFile", &path, title, defaultName)
}
