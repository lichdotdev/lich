package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func packTheme(id, url string, latest *backend.ThemeLatest) themeEntry {
	return themeEntry{def: backend.Theme{ID: id, Source: &backend.ThemeSource{URL: url, Version: "1.0.0"}, Latest: latest}}
}

func TestPendingThemeUpdatesAreOnePerRepository(t *testing.T) {
	newer := &backend.ThemeLatest{Pack: "Tokyo", Version: "1.1.0"}
	got := pendingThemeUpdates([]themeEntry{
		{def: backend.Theme{ID: "light"}},
		packTheme("night", "https://a/tokyo", newer),
		packTheme("day", "https://a/tokyo", newer),
		packTheme("nord", "https://a/nord", nil),
	})
	want := themePackUpdate{url: "https://a/tokyo", pack: "Tokyo", version: "1.1.0", theme: "night"}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("pending = %#v, want only %#v", got, want)
	}
}

func TestAThemeUpdateIsPromptedOncePerVersion(t *testing.T) {
	tokyo := themePackUpdate{url: "https://a/tokyo", version: "1.1.0"}
	nord := themePackUpdate{url: "https://a/nord", version: "2.0.0"}
	pending := []themePackUpdate{tokyo, nord}
	if got := unpromptedThemeUpdates(pending, nil, ""); len(got) != 2 {
		t.Fatalf("nothing prompted yet: got %#v", got)
	}
	if got := unpromptedThemeUpdates(pending, map[string]bool{tokyo.key(): true}, ""); len(got) != 1 || got[0] != nord {
		t.Fatalf("tokyo prompted this run: got %#v", got)
	}
	if got := unpromptedThemeUpdates(pending, nil, tokyo.key()+"\n"+nord.key()); len(got) != 0 {
		t.Fatalf("Later clicked for both: got %#v", got)
	}
	tokyo.version = "1.2.0"
	if got := unpromptedThemeUpdates([]themePackUpdate{tokyo}, nil, "https://a/tokyo 1.1.0"); len(got) != 1 {
		t.Fatalf("a version after the one put off: got %#v", got)
	}
}

func TestThemeUpdateToastLeadsToSettingsForSeveral(t *testing.T) {
	var updated string
	opened := false
	one := themeUpdateToast([]themePackUpdate{{pack: "Tokyo", version: "1.1.0", theme: "night"}}, func() {}, func(id string) { updated = id }, func() { opened = true })
	if one.Title != "Theme pack Tokyo v1.1.0 is available" {
		t.Fatalf("title = %q", one.Title)
	}
	one.Action.OnClick()
	if updated != "night" || opened {
		t.Fatalf("Update asked for %q, opened Settings %v", updated, opened)
	}
	several := themeUpdateToast(make([]themePackUpdate, 2), func() {}, func(string) {}, func() { opened = true })
	if several.Title != "2 theme packs have a new version" {
		t.Fatalf("title = %q", several.Title)
	}
	several.Action.OnClick()
	if !opened {
		t.Fatal("the toast for several packs did not open Settings")
	}
}
