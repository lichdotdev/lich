package main

import (
	"context"
	"encoding/json"
	"image/color"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// bundledTheme reads themes/<id>.json, the file the backend serves the
// bundled themes from.
func bundledTheme(t *testing.T, id string) lichclient.Theme {
	t.Helper()
	data, err := os.ReadFile("../../../themes/" + id + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var th lichclient.Theme
	if err := json.Unmarshal(data, &th); err != nil {
		t.Fatal(err)
	}
	return th
}

func customTheme(id, name string) lichclient.Theme {
	th := lichclient.Theme{ID: id, Name: name, Scheme: themeDark, Origin: themeOriginCustom,
		App: map[string]string{}, Terminal: map[string]string{"background": "#191724", "foreground": "#e0def4"}}
	for _, token := range []string{"background", "foreground", "card", "card-foreground", "popover", "popover-foreground",
		"primary", "primary-foreground", "secondary", "secondary-foreground", "muted", "muted-foreground", "accent",
		"accent-foreground", "destructive", "tone-pass", "tone-wait", "border", "input", "ring", "sidebar",
		"sidebar-foreground", "sidebar-accent", "sidebar-accent-foreground", "sidebar-border"} {
		th.App[token] = "#191724"
	}
	return th
}

// The bundled files read into the palettes ported by hand from index.css,
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
	defs := []lichclient.Theme{customTheme("zeta", "zeta"), bundledTheme(t, themeDark), unreadable,
		customTheme("alpha", "Alpha"), bundledTheme(t, themeLight)}
	themes, broken := readThemes(defs, []lichclient.BrokenTheme{{ID: "old", Reason: "bad json"}})
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
	themes, _ := readThemes([]lichclient.Theme{bundledTheme(t, themeLight), bundledTheme(t, themeDark), customTheme("rose-pine", "Rosé Pine")}, nil)
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
		if got := resolveTheme(c.selected, themes, c.systemDark).def.ID; got != c.want {
			t.Errorf("resolveTheme(%q, dark %v) = %q, want %q", c.selected, c.systemDark, got, c.want)
		}
	}
}

func TestThemeCaption(t *testing.T) {
	repo := customTheme("rose-pine", "Rosé Pine")
	repo.Source = &lichclient.ThemeSource{URL: "https://github.com/lichdotdev/rose-pine-lich.git", Version: "1.2.0"}
	cases := []struct {
		def      lichclient.Theme
		selected string
		want     string
	}{
		{repo, "rose-pine", themeInUse},
		{repo, themeSystem, "lichdotdev/rose-pine-lich v1.2.0"},
		{customTheme("file", "File"), themeSystem, themeBundled},
		{bundledTheme(t, themeDark), themeSystem, themeBundled},
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
		if got := searchSettings(q); len(got) == 0 || got[0].title != themeTitle || got[0].section != sectionAppearance {
			t.Errorf("%q: %+v", q, got)
		}
	}
	// "palette" is also the title of a Hotkeys row, a prefix match that
	// outranks the theme's also words, so the theme is found but not first.
	if got := searchSettings("palette"); !slices.ContainsFunc(got, func(h settingHit) bool {
		return h.title == themeTitle && h.section == sectionAppearance
	}) {
		t.Errorf("palette: %+v", got)
	}
}

// The window opens on System when nothing was stored, takes the theme a
// choice resolves to once, and stores the choice where the web reads it.
func TestAppearanceChoosesAndStores(t *testing.T) {
	f, client := newSidebarBackend(t)
	f.themes = []lichclient.Theme{bundledTheme(t, themeDark), bundledTheme(t, themeLight), customTheme("rose-pine", "Rosé Pine")}
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
