package main

import (
	"fmt"
	"image/color"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/godemirror/tags"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// themes.ts's ids and origins.
const (
	themeSystem         = "system"
	themeOriginBundled  = "bundled"
	themeOriginCustom   = "custom"
	themeSchemeLight    = "light"
	themeTemplateName   = "lich-theme-template.json"
	terminalForeground  = "foreground"
	terminalBackground  = "background"
	terminalCursorToken = "cursor"
)

// The canvas behind the islands is index.css's --canvas, the background
// mixed toward black: 8% in :root (light), 35% in .dark.
const (
	canvasDarkenLight = 0.08
	canvasDarkenDark  = 0.35
)

// bundledOrder is themes.ts BUNDLED: light before dark.
var bundledOrder = []string{themeLight, themeDark}

// ansiTokens are xterm's ITheme names for palette entries 0-15.
var ansiTokens = [16]string{
	"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
	"brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightMagenta", "brightCyan", "brightWhite",
}

// themeEntry is a theme this window can draw: its definition, and the
// palette and terminal colors read out of it.
type themeEntry struct {
	def      lichclient.Theme
	palette  ui.Palette
	terminal gioterm.Colors
	code     godemirror.Theme
}

func (e themeEntry) light() bool { return e.def.Scheme == themeSchemeLight }

// readThemes reads the backend's list in mergeThemes' order. A theme whose
// colors this window cannot parse joins broken with the reason, rather than
// drawing in colors it does not have.
func readThemes(defs []lichclient.Theme, broken []lichclient.BrokenTheme) ([]themeEntry, []lichclient.BrokenTheme) {
	sorted := slices.Clone(defs)
	slices.SortStableFunc(sorted, compareThemes)
	entries := make([]themeEntry, 0, len(sorted))
	broken = slices.Clone(broken)
	for _, def := range sorted {
		e, err := readTheme(def)
		if err != nil {
			broken = append(broken, lichclient.BrokenTheme{ID: def.ID, Reason: err.Error()})
			continue
		}
		entries = append(entries, e)
	}
	return entries, broken
}

// compareThemes is mergeThemes' sort: the bundled pair first in its own
// order, then the imported ones by name.
func compareThemes(a, b lichclient.Theme) int {
	aBundled, bBundled := a.Origin == themeOriginBundled, b.Origin == themeOriginBundled
	switch {
	case aBundled != bBundled && aBundled:
		return -1
	case aBundled != bBundled:
		return 1
	case aBundled:
		return slices.Index(bundledOrder, a.ID) - slices.Index(bundledOrder, b.ID)
	}
	return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

func readTheme(def lichclient.Theme) (themeEntry, error) {
	p, err := paletteOf(def)
	if err != nil {
		return themeEntry{}, err
	}
	term, err := terminalOf(def)
	if err != nil {
		return themeEntry{}, err
	}
	code, err := codeThemeOf(def, p)
	if err != nil {
		return themeEntry{}, err
	}
	return themeEntry{def: def, palette: p, terminal: term, code: code}, nil
}

// codeThemeOf is the code views' palette: godemirror's for the scheme, with
// the text and the gutter in the theme's foreground and muted foreground, as
// the web's editor draws them, and the theme's syntax highlighting when it
// has one. Each view lays its own ground over Background.
func codeThemeOf(def lichclient.Theme, p ui.Palette) (godemirror.Theme, error) {
	t := godemirror.DarkTheme
	if def.Scheme == themeSchemeLight {
		t = godemirror.LightTheme
	}
	t.Foreground, t.GutterForeground = p.Foreground, p.MutedForeground
	if len(def.Syntax) == 0 {
		return t, nil
	}
	h, err := highlightOf(def.Syntax)
	if err != nil {
		return t, err
	}
	t.Highlight = h
	return t, nil
}

// highlightOf builds the HighlightStyle of a theme's syntax section. Two keys
// can name one tag ("function(definition(x))" and "definition(function(x))");
// specs go in key order, so which one wins does not change between runs.
func highlightOf(syntax map[string]lichclient.SyntaxStyle) (*godemirror.HighlightStyle, error) {
	specs := make([]godemirror.TagStyle, 0, len(syntax))
	for _, key := range slices.Sorted(maps.Keys(syntax)) {
		tag, err := tags.Parse(key)
		if err != nil {
			return nil, fmt.Errorf("syntax: %w", err)
		}
		s := syntax[key]
		spec := godemirror.TagStyle{Tags: []*tags.Tag{tag}, Bold: s.Bold, Italic: s.Italic, Underline: s.Underline, Strikethrough: s.Strikethrough}
		if s.Color != "" {
			c, err := ui.ParseColor(s.Color)
			if err != nil {
				return nil, fmt.Errorf("syntax.%s: %w", key, err)
			}
			spec.Color = c
		}
		specs = append(specs, spec)
	}
	return godemirror.NewHighlightStyle(specs...), nil
}

// paletteOf reads the app tokens the native window draws with.
func paletteOf(def lichclient.Theme) (ui.Palette, error) {
	var p ui.Palette
	fields := map[string]*color.NRGBA{
		"background": &p.Background, "foreground": &p.Foreground,
		"card": &p.Card, "card-foreground": &p.CardForeground,
		"popover": &p.Popover, "popover-foreground": &p.PopoverForeground,
		"primary": &p.Primary, "primary-foreground": &p.PrimaryForeground,
		"secondary": &p.Secondary, "secondary-foreground": &p.SecondaryForeground,
		"muted": &p.Muted, "muted-foreground": &p.MutedForeground,
		"accent": &p.Accent, "accent-foreground": &p.AccentForeground,
		"destructive": &p.Destructive, "tone-pass": &p.TonePass, "tone-wait": &p.ToneWait,
		"border": &p.Border, "input": &p.Input, "ring": &p.Ring,
		"sidebar": &p.Sidebar, "sidebar-foreground": &p.SidebarForeground,
		"sidebar-accent": &p.SidebarAccent, "sidebar-accent-foreground": &p.SidebarAccentForeground,
		"sidebar-border": &p.SidebarBorder,
	}
	for token, field := range fields {
		css, ok := def.App[token]
		if !ok {
			return p, fmt.Errorf("app.%s is missing", token)
		}
		c, err := ui.ParseColor(css)
		if err != nil {
			return p, fmt.Errorf("app.%s: %w", token, err)
		}
		*field = c
	}
	darken := canvasDarkenDark
	if def.Scheme == themeSchemeLight {
		darken = canvasDarkenLight
	}
	p.Canvas = ui.MixOKLab(p.Background, color.NRGBA{A: 255}, darken)
	return p, nil
}

// terminalOf reads the terminal's defaults. Only the foreground and the
// background are required; a missing cursor or ANSI color stays libghostty's.
func terminalOf(def lichclient.Theme) (gioterm.Colors, error) {
	var c gioterm.Colors
	read := func(token string, into *color.NRGBA, required bool) error {
		css, ok := def.Terminal[token]
		if !ok {
			if required {
				return fmt.Errorf("terminal.%s is missing", token)
			}
			return nil
		}
		v, err := ui.ParseColor(css)
		if err != nil {
			return fmt.Errorf("terminal.%s: %w", token, err)
		}
		v.A = 255
		*into = v
		return nil
	}
	if err := read(terminalForeground, &c.Foreground, true); err != nil {
		return c, err
	}
	if err := read(terminalBackground, &c.Background, true); err != nil {
		return c, err
	}
	if err := read(terminalCursorToken, &c.Cursor, false); err != nil {
		return c, err
	}
	for i, token := range ansiTokens {
		if err := read(token, &c.ANSI[i], false); err != nil {
			return c, err
		}
	}
	return c, nil
}

// resolveTheme is themes.ts resolveTheme: System is the bundled theme the
// OS asks for, and a selection no theme answers to falls back to it. Following
// the system, a theme of the scheme the OS does not ask for gives way to its
// pair.
func resolveTheme(selected string, themes []themeEntry, systemDark, followSystem bool) themeEntry {
	systemID := themeLight
	if systemDark {
		systemID = themeDark
	}
	system, found := themeByID(themes, systemID)
	if !found && len(themes) > 0 {
		system = themes[0]
	}
	if selected == themeSystem {
		return system
	}
	e, ok := themeByID(themes, selected)
	if !ok {
		return system
	}
	if pair, ok := pairOf(e, themes); ok && followSystem && e.light() == systemDark {
		return pair
	}
	return e
}

// pairOf is e's variant for the other scheme. The backend lists a theme whose
// pair has its own scheme as broken; a pair no theme answers to is none.
func pairOf(e themeEntry, themes []themeEntry) (themeEntry, bool) {
	if e.def.Pair == "" {
		return themeEntry{}, false
	}
	return themeByID(themes, e.def.Pair)
}

func themeByID(themes []themeEntry, id string) (themeEntry, bool) {
	i := slices.IndexFunc(themes, func(e themeEntry) bool { return e.def.ID == id })
	if i < 0 {
		return themeEntry{}, false
	}
	return themes[i], true
}

// bundledPair is the light and the dark bundled theme, System's two halves.
func bundledPair(themes []themeEntry) (light, dark themeEntry) {
	light, _ = themeByID(themes, themeLight)
	dark, _ = themeByID(themes, themeDark)
	return light, dark
}

// themeAfterRemoval is themes.ts themeAfterRemoval: removing the theme in
// use hands the window back to System.
func themeAfterRemoval(removedID, selected string) string {
	if selected == removedID {
		return themeSystem
	}
	return selected
}

// themeCaption is the line beside a card's name: "in use" on the selected
// one, else where it came from.
func themeCaption(def lichclient.Theme, selected string) string {
	switch {
	case def.ID == selected:
		return i18n.T("settings.themePicker.inUse")
	case def.Source != nil:
		return repoLabel(def.Source.URL) + " v" + def.Source.Version
	}
	return i18n.T("settings.themePicker.bundled")
}

var repoSeparators = regexp.MustCompile(`[/\\:]+`)

// repoLabel is themes.ts repoLabel: a clone URL cut to the owner/repo a
// user recognizes.
func repoLabel(url string) string {
	trimmed := strings.TrimRight(strings.TrimSuffix(strings.TrimSpace(url), ".git"), `/\`)
	var segments []string
	for _, s := range repoSeparators.Split(trimmed, -1) {
		if s != "" {
			segments = append(segments, s)
		}
	}
	if len(segments) == 0 {
		return trimmed
	}
	return strings.Join(segments[max(0, len(segments)-2):], "/")
}

// themeCount is the strip's count line, System included.
func themeCount(themes int) string {
	return i18n.T("settings.themePicker.count", i18n.Args{"count": themes + 1})
}
