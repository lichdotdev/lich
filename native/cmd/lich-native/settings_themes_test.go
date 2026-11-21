package main

import (
	"image"
	"slices"
	"strings"
	"testing"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func testThemeCatalog() backend.ThemeCatalog {
	night := customTheme("moss-night", "Moss Night")
	night.Pair = "moss-day"
	day := customTheme("moss-day", "Moss Day")
	day.Scheme, day.Pair = themeSchemeLight, "moss-night"
	ember := customTheme("ember", "Ember")
	return backend.ThemeCatalog{Packs: []backend.ThemeCatalogPack{
		{ID: "moss", Name: "Moss", Publisher: "lichdotdev", Description: "Green and soft", Stars: 3, Version: "1.0.0",
			Updated: "2026-10-01T00:00:00Z", Repo: "https://github.com/lichdotdev/moss-lich", Themes: []backend.Theme{day, night}},
		{ID: "ember", Name: "Ember", Publisher: "someone", Description: "Warm, for the late hours", Stars: 9, Version: "2.0.0",
			Updated: "2026-11-01T00:00:00Z", Repo: "https://github.com/someone/ember", Themes: []backend.Theme{ember}},
	}}
}

func packIDs(packs []themePack) []string {
	out := make([]string, len(packs))
	for i, p := range packs {
		out[i] = p.def.ID
	}
	return out
}

func TestAThemeWhoseColorsDoNotParseIsLeftOutOfTheCatalog(t *testing.T) {
	cat := testThemeCatalog()
	broken := customTheme("bad", "Bad")
	delete(broken.App, "primary")
	cat.Packs = append(cat.Packs, backend.ThemeCatalogPack{ID: "bad", Name: "Bad", Themes: []backend.Theme{broken}})
	read := readThemeCatalog(cat)
	if got := packIDs(read.packs); !slices.Equal(got, []string{"moss", "ember"}) {
		t.Errorf("packs %v, want the two that draw", got)
	}
	if moss, _ := read.pack("moss"); moss.cover().def.ID != "moss-night" {
		t.Errorf("cover %q, want the pack's first dark theme", moss.cover().def.ID)
	}
}

func TestTheThemeFilterNarrowsBySchemeAndQueryAndOrders(t *testing.T) {
	packs := readThemeCatalog(testThemeCatalog()).packs
	cases := []struct {
		scheme int
		query  string
		order  catalogSort
		want   []string
	}{
		{themeTabAll, "", sortRecent, []string{"ember", "moss"}},
		{themeTabAll, "", sortName, []string{"ember", "moss"}},
		{themeTabAll, "", sortStars, []string{"ember", "moss"}},
		{themeTabLight, "", sortRecent, []string{"moss"}},
		{themeTabDark, "", sortRecent, []string{"ember", "moss"}},
		{themeTabAll, "LATE", sortRecent, []string{"ember"}},
		{themeTabAll, "lichdotdev", sortRecent, []string{"moss"}},
		{themeTabAll, "zzz", sortRecent, nil},
	}
	for _, c := range cases {
		if got := packIDs(themesShown(packs, c.scheme, c.query, c.order)); !slices.Equal(got, c.want) {
			t.Errorf("scheme %d, %q, order %d: %v, want %v", c.scheme, c.query, c.order, got, c.want)
		}
	}
}

func TestAPackSaysInUseOverInstalled(t *testing.T) {
	moss, _ := readThemeCatalog(testThemeCatalog()).pack("moss")
	installed := map[string]bool{"moss-day": true}
	for _, c := range []struct {
		selected string
		want     int
	}{{"moss-night", packInUse}, {themeSystem, packInstalled}} {
		if got := packState(moss, c.selected, installed); got != c.want {
			t.Errorf("%s selected: %d, want %d", c.selected, got, c.want)
		}
	}
	if got := packState(moss, themeSystem, nil); got != packListed {
		t.Errorf("nothing installed: %d", got)
	}
}

func TestVariantNamesAndPairs(t *testing.T) {
	moss, _ := readThemeCatalog(testThemeCatalog()).pack("moss")
	if got := packPairs(moss); got != "Day ↔ Night" {
		t.Errorf("pairs %q", got)
	}
	ember, _ := readThemeCatalog(testThemeCatalog()).pack("ember")
	if got := ember.variantName(ember.themes[0]); got != "Ember" {
		t.Errorf("a theme named as its pack is %q", got)
	}
}

func TestOnlyAnOlderReleaseIsRefused(t *testing.T) {
	for _, c := range []struct {
		lich, need string
		older      bool
	}{
		{"v0.52.9", "0.53.0", true},
		{"0.53.0", "0.53.0", false},
		{"v1.0.0", "0.53.0", false},
		{"", "0.53.0", false},
		{"v0.60.0-dev", "0.53.0", false},
		{"v0.10.0", "", false},
	} {
		if got := olderRelease(c.lich, c.need); got != c.older {
			t.Errorf("olderRelease(%q, %q) = %v", c.lich, c.need, got)
		}
	}
}

func TestEveryThemeCardAndTheSkeletonStandAsTall(t *testing.T) {
	th := shapingTheme()
	s := newSettingsScreen(t.Context(), nil, new(ui.Root), func() {})
	s.theme = &themePicker{look: &appearance{}}
	cat := readThemeCatalog(testThemeCatalog())
	height := func(w func(gtx C) D) int {
		gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Constraints{Max: image.Pt(340, 2000)}}
		return w(gtx).Size.Y
	}
	inUse := height(func(gtx C) D { return s.themeCard(gtx, th, cat.packs[0], packInUse) })
	listed := height(func(gtx C) D { return s.themeCard(gtx, th, cat.packs[1], packListed) })
	skeleton := height(func(gtx C) D { return themeSkeletonCard(gtx, th) })
	if inUse != listed || skeleton != listed {
		t.Errorf("heights: in use %d, listed %d, skeleton %d; want one height", inUse, listed, skeleton)
	}
}

func TestATrialLetsGoOnASelectionFromElsewhereAndOnTheSystemsSwitch(t *testing.T) {
	f, client := newSidebarBackend(t)
	f.themes = []backend.Theme{bundledTheme(t, themeLight), bundledTheme(t, themeDark)}
	a := &appearance{ctx: t.Context(), client: client, invalidate: func() {}}
	if err := a.load(); err != nil {
		t.Fatal(err)
	}
	a.setSystemDark(true)
	moss, _ := readThemeCatalog(testThemeCatalog()).pack("moss")
	a.try(moss.themes[0].themeEntry)
	if e, ok := a.take(); !ok || e.def.ID != "moss-day" {
		t.Fatalf("a trial took %q, %v", e.def.ID, ok)
	}
	a.setSystemDark(true)
	if _, ok := a.trying(); !ok {
		t.Fatal("the same scheme again let the trial go")
	}
	a.setSystemDark(false)
	if _, ok := a.trying(); ok {
		t.Fatal("the system's switch kept the trial")
	}
	a.try(moss.themes[0].themeEntry)
	a.applySelection(backend.ThemeSelection{Theme: themeDark})
	if e, ok := a.take(); !ok || e.def.ID != themeDark {
		t.Errorf("a selection from elsewhere took %q, %v", e.def.ID, ok)
	}
}

// themesFrames is Settings on the Themes page over a fake backend holding the
// bundled themes, with the test catalog held, framed with the page's dialogs
// and the trial's bar the way the window frames them.
func themesFrames(t *testing.T) (*settingsScreen, *frameRig, *sidebarBackend) {
	t.Helper()
	s, r, f := settingsFramesOn(t, sectionAppearance+pageSeparator+themesPage)
	f.themes = []backend.Theme{bundledTheme(t, themeLight), bundledTheme(t, themeDark)}
	a := &appearance{ctx: t.Context(), client: s.client, invalidate: r.invalidate}
	if err := a.load(); err != nil {
		t.Fatal(err)
	}
	s.theme = newThemePicker(t.Context(), s.client, a, new(ui.Toaster), s.root)
	s.theme.catalog = newThemeCatalogStore(nil, r.invalidate)
	s.theme.catalog.set(testThemeCatalog())
	r.draw = func(gtx C, th *ui.Theme) {
		s.root.Layout(gtx, func(gtx C) D {
			s.layoutThemesDialogs(gtx, th)
			if s.open {
				s.layout(gtx, th, openTabs)
			}
			s.layoutThemeTrial(gtx, th)
			return D{Size: gtx.Constraints.Max}
		})
	}
	return s, r, f
}

// tryMossNight opens Moss's detail on its night theme and tries it.
func tryMossNight(t *testing.T, r *frameRig) {
	t.Helper()
	r.clickLabel(t, "Moss")
	r.clickLabel(t, i18n.T("settings.themesPage.try"))
	r.settle(t)
	if !r.shows(i18n.T("settings.themesPage.trying", i18n.Args{"name": "Moss Night"})) {
		t.Fatalf("no bar after Try:\n%s", r.drawn())
	}
}

func TestTryWearsTheThemeAndRevertGoesBackWithoutStoringAnything(t *testing.T) {
	s, r, f := themesFrames(t)
	tryMossNight(t, r)
	if e, ok := s.theme.look.trying(); !ok || e.def.ID != "moss-night" {
		t.Fatalf("trying %q, %v", e.def.ID, ok)
	}
	r.clickLabel(t, i18n.T("settings.themesPage.revert"))
	if _, ok := s.theme.look.trying(); ok {
		t.Fatal("Revert kept the trial")
	}
	if r.shows(i18n.T("settings.themesPage.revert")) {
		t.Error("the bar stayed after Revert")
	}
	if got := f.get(themeSettingKey + "@"); got != "" {
		t.Errorf("Try and Revert stored %q", got)
	}
}

func TestKeepInstallsTheListedPackThenChoosesTheTheme(t *testing.T) {
	s, r, f := themesFrames(t)
	tryMossNight(t, r)
	r.clickLabel(t, i18n.T("settings.themesPage.keep"))
	waitForSetting(t, f, themeSettingKey+"@", "moss-night")
	f.mu.Lock()
	installed := slices.ContainsFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, "themes.InstallFromCatalog(") })
	f.mu.Unlock()
	if !installed {
		t.Error("Keep chose a theme it never installed")
	}
	if _, ok := s.theme.look.trying(); ok {
		t.Error("the trial outlived Keep")
	}
}

func TestClosingSettingsLetsTheTrialGo(t *testing.T) {
	s, r, f := themesFrames(t)
	tryMossNight(t, r)
	r.click(t, r.smallestClickIn(t, image.Rect(rigWindow.X-60, 0, rigWindow.X, 60)))
	if s.open {
		t.Fatal("the × left Settings open")
	}
	if _, ok := s.theme.look.trying(); ok {
		t.Error("closing Settings kept the trial")
	}
	if got := f.get(themeSettingKey + "@"); got != "" {
		t.Errorf("closing stored %q", got)
	}
}

func TestTheThemesTabsSwitchInTheFrameThatTookTheClick(t *testing.T) {
	s, r, _ := themesFrames(t)
	r.clickLabel(t, i18n.T("settings.themesPage.installedTab"))
	if s.theme.view.Selected != themesViewInstalled {
		t.Fatal("still on the catalog")
	}
	if !r.shows(strings.ToUpper(i18n.T("settings.themesPage.bundledGroup"))) {
		t.Errorf("no Bundled group:\n%s", r.drawn())
	}
}

func TestEscapeRevertsATrial(t *testing.T) {
	s, r, _ := themesFrames(t)
	tryMossNight(t, r)
	r.router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	r.frame()
	r.frame()
	if _, ok := s.theme.look.trying(); ok {
		t.Fatal("Escape kept the trial")
	}
}
