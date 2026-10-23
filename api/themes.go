package api

import (
	"encoding/json"
	"fmt"
)

var (
	ThemesList            = define[None, []Theme]("themes.List")
	ThemesListBroken      = define[None, []BrokenTheme]("themes.ListBroken")
	ThemesImport          = define[ImportThemeOptions, ThemeImportResult]("themes.Import")
	ThemesInstallFromGit  = define[InstallThemesOptions, ThemeGitResult]("themes.InstallFromGit")
	ThemesUpdateFromGit   = define[UpdateThemeOptions, ThemeGitResult]("themes.UpdateFromGit")
	ThemesRemove          = define[RemoveThemeOptions, None]("themes.Remove")
	ThemesSaveTemplate    = define[SaveThemeTemplateOptions, None]("themes.SaveTemplate")
	ThemesSelection       = define[None, ThemeSelection]("themes.Selection")
	ThemesSelect          = define[SelectThemeOptions, None]("themes.Select")
	ThemesSetFollowSystem = define[FollowSystemOptions, None]("themes.SetFollowSystem")
)

// Theme describes every color value the window needs to paint the app and
// the terminal.
type Theme struct {
	FormatVersion int               `json:"formatVersion,omitempty"`
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Scheme        string            `json:"scheme"`
	Origin        string            `json:"origin"`
	Source        *ThemeSource      `json:"source,omitempty"`
	App           map[string]string `json:"app"`
	Terminal      map[string]string `json:"terminal"`
	// Syntax is optional: a theme without it is highlighted in the window's
	// default palette for its scheme.
	Syntax Syntax `json:"syntax,omitempty"`
	// Pair is optional: the id of this theme's variant for the other scheme,
	// which the window draws while the OS asks for that scheme and the user
	// turned on following it.
	Pair string `json:"pair,omitempty"`
}

// BrokenTheme is a stored custom theme this lich cannot load. It is listed
// rather than dropped, so the user sees why and can remove it by id.
type BrokenTheme struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// ThemeSource records the repository a theme was installed from and the
// manifest version it came in at, so lich can clone it again to look for a
// newer one. It is written by lich, never by a picked file: a standalone import
// has no source and stays unversioned.
type ThemeSource struct {
	URL     string `json:"url"`
	Version string `json:"version"`
}

// SyntaxStyle is how the code under one highlighting tag is drawn: a
// CodeMirror HighlightStyle spec. A theme file writes it as a bare color or
// as an object; Color empty leaves the code its foreground color.
type SyntaxStyle struct {
	Color         string `json:"color,omitempty"`
	Bold          bool   `json:"bold,omitempty"`
	Italic        bool   `json:"italic,omitempty"`
	Underline     bool   `json:"underline,omitempty"`
	Strikethrough bool   `json:"strikethrough,omitempty"`
}

// Syntax is a theme's syntax highlighting, keyed by the tag each style is
// for, as @lezer/highlight prints it: "keyword", "function(variableName)".
type Syntax map[string]SyntaxStyle

// UnmarshalJSON reads each entry as a bare color or a style object, naming
// the entry that is neither.
func (s *Syntax) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("syntax must be an object of tag styles: %w", err)
	}
	out := make(Syntax, len(raw))
	for tag, entry := range raw {
		var style SyntaxStyle
		if err := json.Unmarshal(entry, &style.Color); err != nil {
			if err := json.Unmarshal(entry, &style); err != nil {
				return fmt.Errorf("syntax.%s must be a color or a style object", tag)
			}
		}
		out[tag] = style
	}
	*s = out
	return nil
}

// ThemeImportResult either reports the installed theme or asks the UI to
// confirm replacing the existing custom theme with the same id.
type ThemeImportResult struct {
	Theme          Theme `json:"theme"`
	NeedsOverwrite bool  `json:"needsOverwrite"`
}

// ThemeGitResult reports what installing a repository produced. Conflicts
// names the theme ids that already exist locally: found by the scan that runs
// before any write, nothing was installed at all; found by the write itself — a
// theme that appeared in between — that one id was left alone and Themes says
// what did go in.
type ThemeGitResult struct {
	Pack      string   `json:"pack"`
	Version   string   `json:"version"`
	Themes    []Theme  `json:"themes"`
	Conflicts []string `json:"conflicts"`
	UpToDate  bool     `json:"upToDate"`
}

// ThemeSelection is the theme the user chose and whether the window draws
// that theme's pair while the OS asks for the pair's scheme.
type ThemeSelection struct {
	Theme        string `json:"theme"`
	FollowSystem bool   `json:"followSystem"`
}

// ImportThemeOptions names the theme file to import; Overwrite replaces a
// custom theme of the same id instead of asking.
type ImportThemeOptions struct {
	Path      string `json:"path"`
	Overwrite bool   `json:"overwrite"`
}

// InstallThemesOptions names the theme repository to clone.
type InstallThemesOptions struct {
	URL       string `json:"url"`
	Overwrite bool   `json:"overwrite"`
}

// UpdateThemeOptions names a theme whose repository is cloned again.
type UpdateThemeOptions struct {
	ID string `json:"id"`
}

// RemoveThemeOptions names the custom theme to delete.
type RemoveThemeOptions struct {
	ID string `json:"id"`
}

// SaveThemeTemplateOptions is where the starter theme is written.
type SaveThemeTemplateOptions struct {
	Path string `json:"path"`
}

// SelectThemeOptions names the theme to store as the selection: "system" or
// an installed theme's id.
type SelectThemeOptions struct {
	ID string `json:"id"`
}

// FollowSystemOptions is whether the window follows the OS to the selected
// theme's pair.
type FollowSystemOptions struct {
	On bool `json:"on"`
}
