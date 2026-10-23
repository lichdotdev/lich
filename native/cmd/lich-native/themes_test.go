package main

import (
	"context"
	"encoding/json"
	"image/color"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/godemirror/tags"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// bundledTheme reads themes/<id>.json, the file the backend serves the
// bundled themes from.
func bundledTheme(t *testing.T, id string) backend.Theme {
	t.Helper()
	data, err := os.ReadFile("../../../themes/" + id + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var th backend.Theme
	if err := json.Unmarshal(data, &th); err != nil {
		t.Fatal(err)
	}
	return th
}

func customTheme(id, name string) backend.Theme {
	th := backend.Theme{ID: id, Name: name, Scheme: themeDark, Origin: themeOriginCustom,
		App: map[string]string{}, Terminal: map[string]string{"background": "#191724", "foreground": "#e0def4"}}
	for _, token := range []string{"background", "foreground", "card", "card-foreground", "popover", "popover-foreground",
		"primary", "primary-foreground", "secondary", "secondary-foreground", "muted", "muted-foreground", "accent",
		"accent-foreground", "destructive", "tone-pass", "tone-wait", "border", "input", "ring", "sidebar",
		"sidebar-foreground", "sidebar-accent", "sidebar-accent-foreground", "sidebar-border"} {
		th.App[token] = "#191724"
	}
	return th
}

// The bundled files read into the hand-written palettes (ui.Light, ui.Dark),
// so a token wired to the wrong field shows here.
func TestBundledThemesReadIntoTheIndexCSSPalettes(t *testing.T) {
	for id, want := range map[string]ui.Palette{themeLight: ui.Light, themeDark: ui.Dark} {
		got, err := paletteOf(bundledTheme(t, id))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s:\n got %+v\nwant %+v", id, got, want)
		}
	}
}

func TestTerminalColorsFromTheTemplate(t *testing.T) {
	c, err := terminalOf(bundledTheme(t, "template"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Background != rgb(0x15111f) || c.Cursor != rgb(0xc4a7ff) || c.ANSI[1] != rgb(0xff5c8a) {
		t.Errorf("background %v cursor %v red %v", c.Background, c.Cursor, c.ANSI[1])
	}
	dark, _ := terminalOf(bundledTheme(t, themeDark))
	if dark.Cursor.A != 0 || dark.ANSI[1].A != 0 {
		t.Errorf("a theme naming no cursor or ANSI colors set some: %+v", dark)
	}
}

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

func TestReadThemesSortsAndSetsAsideWhatItCannotDraw(t *testing.T) {
	unreadable := customTheme("labbed", "Labbed")
	unreadable.App["background"] = "lab(50 20 30)"
	defs := []backend.Theme{customTheme("zeta", "zeta"), bundledTheme(t, themeDark), unreadable,
		customTheme("alpha", "Alpha"), bundledTheme(t, themeLight)}
	themes, broken := readThemes(defs, []backend.BrokenTheme{{ID: "old", Reason: "bad json"}})
	var ids []string
	for _, e := range themes {
		ids = append(ids, e.def.ID)
	}
	if !slices.Equal(ids, []string{themeLight, themeDark, "alpha", "zeta"}) {
		t.Errorf("order %v, want the bundled pair, then by name", ids)
	}
	if len(broken) != 2 || broken[1].ID != "labbed" || !strings.Contains(broken[1].Reason, "app.background") {
		t.Errorf("broken %+v, want the backend's and labbed's with its token", broken)
	}
	if got := themeCount(len(themes)); got != "5 themes" {
		t.Errorf("count %q, want System counted too", got)
	}
}

func TestResolveTheme(t *testing.T) {
	themes, _ := readThemes([]backend.Theme{bundledTheme(t, themeLight), bundledTheme(t, themeDark), customTheme("rose-pine", "Rosé Pine")}, nil)
	cases := []struct {
		selected   string
		systemDark bool
		want       string
	}{
		{themeSystem, true, themeDark},
		{themeSystem, false, themeLight},
		{"rose-pine", false, "rose-pine"},
		{"removed", true, themeDark},
		{themeLight, true, themeLight},
	}
	for _, c := range cases {
		if got := resolveTheme(c.selected, themes, c.systemDark, false).def.ID; got != c.want {
			t.Errorf("resolveTheme(%q, dark %v) = %q, want %q", c.selected, c.systemDark, got, c.want)
		}
	}
}

// Following the system, a theme the OS asks the other scheme of gives way
// to its pair; without a pair, or not following, it stays.
func TestResolveThemeFollowsTheSystemToThePair(t *testing.T) {
	night := customTheme("forest-night", "Forest Night")
	night.Scheme, night.Pair = "dark", "forest-day"
	day := customTheme("forest-day", "Forest Day")
	day.Scheme, day.Pair = "light", "forest-night"
	lone := customTheme("lone", "Lone")
	lone.Scheme = "dark"
	themes, _ := readThemes([]backend.Theme{bundledTheme(t, themeLight), bundledTheme(t, themeDark), night, day, lone}, nil)
	cases := []struct {
		selected             string
		systemDark, followOS bool
		want                 string
	}{
		{"forest-night", false, true, "forest-day"},
		{"forest-night", true, true, "forest-night"},
		{"forest-day", true, true, "forest-night"},
		{"forest-night", false, false, "forest-night"},
		{"lone", false, true, "lone"},
		{themeSystem, false, true, themeLight},
	}
	for _, c := range cases {
		if got := resolveTheme(c.selected, themes, c.systemDark, c.followOS).def.ID; got != c.want {
			t.Errorf("resolveTheme(%q, dark %v, follow %v) = %q, want %q", c.selected, c.systemDark, c.followOS, got, c.want)
		}
	}
}

// A selection stored elsewhere, `lich theme switch` or another window,
// repaints this one.
func TestAppearanceTakesASelectionStoredElsewhere(t *testing.T) {
	f, client := newSidebarBackend(t)
	night := customTheme("forest-night", "Forest Night")
	night.Scheme, night.Pair = "dark", "forest-day"
	day := customTheme("forest-day", "Forest Day")
	day.Scheme, day.Pair = "light", "forest-night"
	f.themes = []backend.Theme{bundledTheme(t, themeLight), bundledTheme(t, themeDark), night, day}
	a := &appearance{ctx: context.Background(), client: client, invalidate: func() {}}
	if err := a.load(); err != nil {
		t.Fatal(err)
	}
	a.setSystemDark(false)
	a.take()
	a.applySelection(backend.ThemeSelection{Theme: "forest-night", FollowSystem: true})
	if e, ok := a.take(); !ok || e.def.ID != "forest-day" {
		t.Errorf("following a light desktop from forest-night took %q, %v", e.def.ID, ok)
	}
	a.setFollowSystem(false)
	if e, ok := a.take(); !ok || e.def.ID != "forest-night" {
		t.Errorf("no longer following took %q, %v", e.def.ID, ok)
	}
	waitForSetting(t, f, "appearance.followSystem@", "false")
}

func TestThemeCaption(t *testing.T) {
	repo := customTheme("rose-pine", "Rosé Pine")
	repo.Source = &backend.ThemeSource{URL: "https://github.com/lichdotdev/rose-pine-lich.git", Version: "1.2.0"}
	cases := []struct {
		def      backend.Theme
		selected string
		want     string
	}{
		{repo, "rose-pine", "in use"},
		{repo, themeSystem, "lichdotdev/rose-pine-lich v1.2.0"},
		{customTheme("file", "File"), themeSystem, "bundled"},
		{bundledTheme(t, themeDark), themeSystem, "bundled"},
	}
	for _, c := range cases {
		if got := themeCaption(c.def, c.selected); got != c.want {
			t.Errorf("%s with %s selected: %q, want %q", c.def.ID, c.selected, got, c.want)
		}
	}
}

func TestRepoLabel(t *testing.T) {
	for url, want := range map[string]string{
		"https://github.com/lichdotdev/rose-pine-lich.git": "lichdotdev/rose-pine-lich",
		" git@github.com:me/themes/ ":                      "me/themes",
		`C:\themes\mine`:                                   "themes/mine",
		"solo":                                             "solo",
	} {
		if got := repoLabel(url); got != want {
			t.Errorf("repoLabel(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestRemovingTheThemeInUseFallsBackToSystem(t *testing.T) {
	if got := themeAfterRemoval("rose-pine", "rose-pine"); got != themeSystem {
		t.Errorf("removed the selection: %q", got)
	}
	if got := themeAfterRemoval("rose-pine", themeDark); got != themeDark {
		t.Errorf("removed another theme: %q", got)
	}
}

func TestThemeIsFoundBySearch(t *testing.T) {
	for _, q := range []string{"theme", "DARK"} {
		if got := searchSettings(q); len(got) == 0 || got[0].title != "Theme" || got[0].section != sectionAppearance {
			t.Errorf("%q: %+v", q, got)
		}
	}
	// "palette" is also the title of a Hotkeys row, a prefix match that
	// outranks the theme's also words, so the theme is found but not first.
	if got := searchSettings("palette"); !slices.ContainsFunc(got, func(h settingHit) bool {
		return h.title == "Theme" && h.section == sectionAppearance
	}) {
		t.Errorf("palette: %+v", got)
	}
}

// The window opens on System when nothing was stored, takes the theme a
// choice resolves to once, and stores the choice.
func TestAppearanceChoosesAndStores(t *testing.T) {
	f, client := newSidebarBackend(t)
	f.themes = []backend.Theme{bundledTheme(t, themeDark), bundledTheme(t, themeLight), customTheme("rose-pine", "Rosé Pine")}
	a := &appearance{ctx: context.Background(), client: client, invalidate: func() {}}
	if err := a.load(); err != nil {
		t.Fatal(err)
	}
	if a.view().selected != themeSystem {
		t.Errorf("nothing stored opens on %q", a.view().selected)
	}
	a.setSystemDark(false)
	if e, ok := a.take(); !ok || e.def.ID != themeLight || !e.light() {
		t.Errorf("System on a light desktop: %q, %v", e.def.ID, ok)
	}
	if _, ok := a.take(); ok {
		t.Error("an unchanged appearance repainted again")
	}
	a.choose("rose-pine")
	if e, ok := a.take(); !ok || e.def.ID != "rose-pine" || e.terminal.Background != rgb(0x191724) {
		t.Errorf("chose rose-pine, took %q", e.def.ID)
	}
	waitForSetting(t, f, themeSettingKey+"@", "rose-pine")
}

// resolved is a HighlightStyle as the style each tag gets, the specs that
// name one tag folded into one entry.
func resolved(h *godemirror.HighlightStyle) map[*tags.Tag]godemirror.TagStyle {
	out := map[*tags.Tag]godemirror.TagStyle{}
	for _, s := range h.Specs() {
		for _, tag := range s.Tags {
			s := s
			s.Tags = nil
			out[tag] = s
		}
	}
	return out
}

// The bundled themes write godemirror's own palettes out tag by tag, so the
// code views look as they did before themes carried syntax.
func TestBundledThemesHighlightAsGodemirrorsDefaults(t *testing.T) {
	for id, want := range map[string]godemirror.Theme{themeLight: godemirror.LightTheme, themeDark: godemirror.DarkTheme} {
		e, err := readTheme(bundledTheme(t, id))
		if err != nil {
			t.Fatal(err)
		}
		got, exp := resolved(e.code.Highlight), resolved(want.Highlight)
		if len(got) != len(exp) {
			t.Fatalf("%s: %d tags styled, godemirror styles %d", id, len(got), len(exp))
		}
		for tag, style := range exp {
			if !reflect.DeepEqual(got[tag], style) {
				t.Errorf("%s: %s = %+v, want %+v", id, tag, got[tag], style)
			}
		}
	}
}

func TestThemeSyntaxReachesTheCodeViews(t *testing.T) {
	th := customTheme("pine", "Pine")
	th.Syntax = map[string]backend.SyntaxStyle{
		"keyword":                {Color: "#c4a7e7"},
		"function(variableName)": {Color: "#ebbcba", Bold: true},
	}
	e, err := readTheme(th)
	if err != nil {
		t.Fatal(err)
	}
	got := resolved(e.code.Highlight)
	if c := got[tags.Keyword].Color; c != (color.NRGBA{R: 0xc4, G: 0xa7, B: 0xe7, A: 0xff}) {
		t.Errorf("keyword color = %v", c)
	}
	if s := got[tags.Function(tags.VariableName)]; !s.Bold {
		t.Errorf("function(variableName) = %+v, want bold", s)
	}
	if e.code.Foreground != e.palette.Foreground {
		t.Errorf("code foreground = %v, want the theme's %v", e.code.Foreground, e.palette.Foreground)
	}
}

// A syntax entry this window cannot read sets the theme aside as broken,
// rather than drawing code in colors the author did not write.
func TestABadSyntaxEntryBreaksTheTheme(t *testing.T) {
	th := customTheme("pine", "Pine")
	th.Syntax = map[string]backend.SyntaxStyle{"keywords": {Color: "#fff"}}
	if _, err := readTheme(th); err == nil || !strings.Contains(err.Error(), `"keywords" is not a highlighting tag`) {
		t.Fatalf("readTheme error = %v", err)
	}
}
