package main

import (
	"image/color"
	"regexp"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const internalNewTabURL = "http://127.0.0.1:27000/new-tab/abc"

func TestTheNewTabPageShowsNoInternalAddress(t *testing.T) {
	st := api.BrowserState{URL: internalNewTabURL, Title: "New tab", NewTab: true}
	if got := pageURL(st); got != "about:blank" {
		t.Fatalf("pageURL = %q", got)
	}
	if displayURL(pageURL(st)) != "" || shortURL(pageURL(st)) != "" {
		t.Fatal("the address field is not empty on the new-tab page")
	}
	if pageTitle(st) != "" {
		t.Fatalf("pageTitle = %q", pageTitle(st))
	}
	other := api.BrowserState{URL: "https://example.com/a", Title: "A"}
	if pageURL(other) != other.URL || pageTitle(other) != "A" {
		t.Fatal("another page lost its address or title")
	}
}

func TestTheNewTabPageIsNamedByTheCatalogWhateverItsTitle(t *testing.T) {
	tab := api.BrowserTab{URL: internalNewTabURL, Title: "Nova aba", NewTab: true}
	if got := tabLabel(tab); got != "New tab" {
		t.Fatalf("tabLabel = %q", got)
	}
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	if got := tabLabel(tab); got != "Nova aba" {
		t.Fatalf("tabLabel in pt-BR = %q", got)
	}
}

func TestTheNewTabPageHasNoSelectAndDrawBar(t *testing.T) {
	page := api.BrowserState{URL: "https://example.com"}
	if !notesOnPage(page) {
		t.Fatal("a page has no bar")
	}
	if notesOnPage(api.BrowserState{URL: internalNewTabURL, NewTab: true}) {
		t.Fatal("the new-tab page has the bar")
	}
	devtools := api.BrowserState{Tabs: []api.BrowserTab{{ID: "a"}, {ID: "d", DevTools: true, Front: true}}}
	if notesOnPage(devtools) {
		t.Fatal("DevTools has the bar")
	}
}

func TestTheNewTabLookIsTheThemesAndTheLanguages(t *testing.T) {
	hex := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	for _, p := range []ui.Palette{ui.Dark, ui.Light} {
		look := newTabLook(&ui.Theme{Palette: p})
		c, f := look.Colors, look.Flame
		for _, v := range []string{c.Background, c.Card, c.Foreground, c.Muted, c.Border, c.Accent, c.Pass, f.Core, f.Body, f.Edge, f.Base} {
			if !hex.MatchString(v) {
				t.Errorf("%q is no opaque hex color", v)
			}
		}
		if c.Background != cssHex(p.Background) || c.Muted != cssHex(p.MutedForeground) {
			t.Errorf("colors = %+v, not the palette's", c)
		}
		if f.Rows != chatFlameFrames[0] {
			t.Errorf("the flame is not the chat's first frame")
		}
	}
	if newTabLook(&ui.Theme{Palette: ui.Dark}).Colors == newTabLook(&ui.Theme{Palette: ui.Light}).Colors {
		t.Fatal("light and dark are drawn alike")
	}
}

func TestTheNewTabLookFollowsTheInterfaceLanguage(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	en := newTabLook(th)
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	pt := newTabLook(th)
	if en.Lang != "en" || pt.Lang != "pt-BR" {
		t.Fatalf("langs = %q, %q", en.Lang, pt.Lang)
	}
	if en.Text == pt.Text || pt.Text.Recent != "Recentes" {
		t.Fatalf("text = %+v", pt.Text)
	}
	for _, s := range []string{pt.Text.Title, pt.Text.Address, pt.Text.AddressError, pt.Text.Running, pt.Text.Worktree, pt.Text.Artifacts, pt.Text.Recent} {
		if s == "" || strings.Contains(s, "layouts.browser") {
			t.Errorf("text %q is unresolved", s)
		}
	}
}

func TestTheAddressFollowsAPageThatChangedUnderAnUntouchedEdit(t *testing.T) {
	if !movedUnderEdit("https://a.test/", "about:blank", "https://a.test/") {
		t.Fatal("the new tab did not take the field")
	}
	if movedUnderEdit("https://a.test/", "about:blank", "typed") {
		t.Fatal("a typed address was replaced")
	}
	if movedUnderEdit("https://a.test/", "https://a.test/", "https://a.test/") {
		t.Fatal("an unchanged page took the field")
	}
}

// A custom theme may make a token translucent, which a page cannot be told in
// a hex color without its alpha.
func TestTranslucentTokensAreComposedOnTheBackground(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	th.Background = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	th.Accent = color.NRGBA{R: 255, G: 255, B: 255, A: 128}
	th.Card = color.NRGBA{R: 255, G: 255, B: 255, A: 0}
	look := newTabLook(th)
	if look.Colors.Accent == "#ffffff" || look.Colors.Accent != cssHex(ui.Over(th.Background, th.Accent)) {
		t.Errorf("accent = %s, want it over the background", look.Colors.Accent)
	}
	if look.Colors.Card != "#000000" {
		t.Errorf("a clear card = %s, want the background", look.Colors.Card)
	}
}
