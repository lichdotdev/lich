package main

import (
	"testing"

	"gioui.org/io/key"
)

func TestANavEntryOpensItsPageInTheFrameThatTookTheClick(t *testing.T) {
	s, r := settingsFrames(t, sectionProviders)
	r.settle(t)
	r.clickLabel(t, "Notifications")
	if s.section != sectionGeneral || s.page != "notifications" {
		t.Fatalf("on %q/%q, want general/notifications", s.section, s.page)
	}
	r.settle(t)
	if !r.shows("Notify me when a session needs input") || r.shows("Interface language") {
		t.Fatal("the Notifications page does not show its rows alone")
	}
	r.clickLabel(t, "Hotkeys")
	if s.section != sectionHotkeys {
		t.Fatalf("on %q, want hotkeys", s.section)
	}
}

func TestASearchResultOpensThePageThatHoldsItsRow(t *testing.T) {
	for q, want := range map[string]struct{ title, section, page string }{
		"finishes":      {"Notify me when a session finishes working", sectionGeneral, "notifications"},
		"copy on":       {"Copy on select", sectionGeneral, "terminal"},
		"terminal font": {"Terminal font", sectionAppearance, "terminal"},
	} {
		t.Run(q, func(t *testing.T) {
			s, r := settingsFrames(t, sectionProviders)
			s.nav.query.SetText(q)
			r.settle(t)
			r.clickLabel(t, want.title)
			if s.section != want.section || s.page != want.page {
				t.Fatalf("on %q/%q, want %s/%s", s.section, s.page, want.section, want.page)
			}
		})
	}
}

func TestTwoTerminalPagesAreToldApartByTheirPath(t *testing.T) {
	for q, want := range map[string]string{"copy on select": "General › Terminal", "terminal font": "Appearance › Terminal"} {
		if got := searchSettings(q); len(got) == 0 || got[0].path() != want {
			t.Errorf("%q: %+v, want the path %q first", q, got, want)
		}
	}
}

func TestCtrlFPutsTheCursorInTheSearchField(t *testing.T) {
	s, r := settingsFrames(t, sectionGeneral)
	r.settle(t)
	if r.router.Source().Focused(&s.nav.query) {
		t.Fatal("the field had the focus before the key")
	}
	r.router.Queue(key.Event{Name: "F", Modifiers: key.ModShortcut, State: key.Press})
	r.frame()
	r.frame()
	if !r.router.Source().Focused(&s.nav.query) {
		t.Fatal("Ctrl+F left the search field without the focus")
	}
}

func TestARememberedPageReopensAndAStaleOneFallsToTheFirst(t *testing.T) {
	general := sectionByID(sectionGeneral)
	if parseSettingsSection("general/window") != sectionGeneral || settingsPageOf("general/window") != "window" {
		t.Error("general/window does not read as the Window page")
	}
	if p, _ := general.page("gone"); p.id != "language" {
		t.Errorf("a stale page opens %q, want language", p.id)
	}
	if _, split := sectionByID(sectionEditor).page(""); split {
		t.Error("Editor reads as split")
	}
}
