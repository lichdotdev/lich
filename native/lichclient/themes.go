package lichclient

import (
	"context"
	"encoding/json"
)

// Theme mirrors themes.Theme: a color theme for the app tokens (CSS colors,
// keyed by the index.css names without "--") and the terminal (hex colors,
// keyed by xterm's ITheme names).
type Theme struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Scheme   string            `json:"scheme"`
	Origin   string            `json:"origin"`
	Source   *ThemeSource      `json:"source,omitempty"`
	App      map[string]string `json:"app"`
	Terminal map[string]string `json:"terminal"`
	// Syntax styles the code views by highlighting tag; nil keeps the
	// default palette of the theme's scheme.
	Syntax map[string]SyntaxStyle `json:"syntax,omitempty"`
	// Pair is the id of this theme's variant for the other scheme, "" for
	// none.
	Pair string `json:"pair,omitempty"`
}

// ThemeSelectionEvent is themes.SelectionEventName: the stored selection
// changed, from this window, another, or `lich theme switch`.
const ThemeSelectionEvent = "theme-selection"

// ThemeSelection mirrors themes.Selection.
type ThemeSelection struct {
	Theme        string `json:"theme"`
	FollowSystem bool   `json:"followSystem"`
}

// ThemeSelection reads the stored selection; a theme never chosen reads
// "system".
func (c *Client) ThemeSelection(ctx context.Context) (ThemeSelection, error) {
	var out ThemeSelection
	return out, c.Call(ctx, "themes.Selection", &out)
}

// SelectTheme stores id, "system" or an installed theme, as the selection.
func (c *Client) SelectTheme(ctx context.Context, id string) error {
	return c.Call(ctx, "themes.Select", nil, struct {
		ID string `json:"id"`
	}{id})
}

// SetFollowSystem stores whether the window draws the selected theme's pair
// while the OS asks for the pair's scheme.
func (c *Client) SetFollowSystem(ctx context.Context, on bool) error {
	return c.Call(ctx, "themes.SetFollowSystem", nil, on)
}

// SyntaxStyle mirrors themes.SyntaxStyle: one tag's color and font style.
type SyntaxStyle struct {
	Color         string `json:"color,omitempty"`
	Bold          bool   `json:"bold,omitempty"`
	Italic        bool   `json:"italic,omitempty"`
	Underline     bool   `json:"underline,omitempty"`
	Strikethrough bool   `json:"strikethrough,omitempty"`
}

// UnmarshalJSON also reads the bare color a theme file may write: the
// backend sends the object, the bundled theme files keep the short form.
func (s *SyntaxStyle) UnmarshalJSON(data []byte) error {
	if json.Unmarshal(data, &s.Color) == nil {
		return nil
	}
	type plain SyntaxStyle
	return json.Unmarshal(data, (*plain)(s))
}

// ThemeSource mirrors themes.Source: the repository a theme was installed
// from.
type ThemeSource struct {
	URL     string `json:"url"`
	Version string `json:"version"`
}

// BrokenTheme mirrors themes.BrokenTheme: a stored theme that cannot load.
type BrokenTheme struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// ThemeImportResult mirrors themes.ImportResult: NeedsOverwrite asks before
// replacing a theme of the same id.
type ThemeImportResult struct {
	Theme          Theme `json:"theme"`
	NeedsOverwrite bool  `json:"needsOverwrite"`
}

// ThemeGitResult mirrors themes.GitInstallResult. Conflicts names the ids an
// install refused until it is repeated with overwrite.
type ThemeGitResult struct {
	Pack      string   `json:"pack"`
	Version   string   `json:"version"`
	Themes    []Theme  `json:"themes"`
	Conflicts []string `json:"conflicts"`
	UpToDate  bool     `json:"upToDate"`
}

// Themes returns the bundled themes and the imported ones that load.
func (c *Client) Themes(ctx context.Context) ([]Theme, error) {
	var out []Theme
	return out, c.Call(ctx, "themes.List", &out)
}

// BrokenThemes returns the stored themes Themes leaves out, with why.
func (c *Client) BrokenThemes(ctx context.Context) ([]BrokenTheme, error) {
	var out []BrokenTheme
	return out, c.Call(ctx, "themes.ListBroken", &out)
}

// ImportTheme imports the theme file at path.
func (c *Client) ImportTheme(ctx context.Context, path string, overwrite bool) (ThemeImportResult, error) {
	var out ThemeImportResult
	return out, c.Call(ctx, "themes.Import", &out, path, overwrite)
}

// InstallThemes clones url and installs every theme its manifest lists.
func (c *Client) InstallThemes(ctx context.Context, url string, overwrite bool) (ThemeGitResult, error) {
	var out ThemeGitResult
	return out, c.Call(ctx, "themes.InstallFromGit", &out, url, overwrite)
}

// UpdateTheme re-clones the repository theme id came from.
func (c *Client) UpdateTheme(ctx context.Context, id string) (ThemeGitResult, error) {
	var out ThemeGitResult
	return out, c.Call(ctx, "themes.UpdateFromGit", &out, id)
}

// RemoveTheme deletes an imported theme; the backend refuses a bundled one.
func (c *Client) RemoveTheme(ctx context.Context, id string) error {
	return c.Call(ctx, "themes.Remove", nil, id)
}

// SaveThemeTemplate writes the starter theme to path.
func (c *Client) SaveThemeTemplate(ctx context.Context, path string) error {
	return c.Call(ctx, "themes.SaveTemplate", nil, path)
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
