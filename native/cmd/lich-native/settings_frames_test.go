package main

import (
	"image"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// settingsFrames is the Settings screen open on section, a "section/page"
// one opening that page, over a fake
// backend with Claude Code and Codex installed and on, framed the way the
// window frames it: its dialogs first, then the screen.
func settingsFrames(t *testing.T, section string) (*settingsScreen, *frameRig) {
	t.Helper()
	s, r, _ := settingsFramesOn(t, section)
	return s, r
}

// settingsFramesOn is settingsFrames with the fake backend it draws over, for
// a test that reads what a row stored.
func settingsFramesOn(t *testing.T, section string) (*settingsScreen, *frameRig, *sidebarBackend) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	f, client := newSidebarBackend(t)
	twoInstalled(f)
	root := new(ui.Root)
	s := newSettingsScreen(t.Context(), client, root, r.invalidate)
	s.providers.book = newModel(client, r.invalidate).providers
	if err := s.providers.book.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.theme = &themePicker{toaster: new(ui.Toaster)}
	s.notify = newNotifier(t.Context(), client, nil, r.invalidate)
	s.open, s.section, s.page = true, parseSettingsSection(section), settingsPageOf(section)
	r.draw = func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layoutPatchNotesDialog(gtx, th)
			if s.open {
				s.layout(gtx, th, openTabs)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return s, r, f
}

// TestAProjectLeavesOutWhatOnlyGlobalSets: a project's provider screen draws
// the rows it can change, Model among them, and leaves the global-only ones to
// one line at the foot, whose button opens Global where they are.
func TestAProjectLeavesOutWhatOnlyGlobalSets(t *testing.T) {
	s, r := settingsFrames(t, sectionProviders)
	s.providers.open, s.scope = claudeKind, "pg"
	r.settle(t)
	skip := i18n.T("settings.providerBinSettings.skipTitle")
	if r.shows(skip) || r.shows(i18n.T("settings.ultracodeSetting.title")) {
		t.Error("a project's screen drew a row only Global sets")
	}
	if !r.shows(i18n.T("settings.chatPrefSetting.modelTitle")) || !r.shows(i18n.T("settings.settingsRow.restGlobal")) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	r.clickLabel(t, i18n.T("settings.settingsRow.openGlobal"))
	r.settle(t)
	if s.scope != "" || !r.shows(skip) {
		t.Errorf("Open Global left scope %q", s.scope)
	}
}

// TestAPageWithNothingAProjectSetsSaysSo: a page whose every row is global
// draws only the foot from a project.
func TestAPageWithNothingAProjectSetsSaysSo(t *testing.T) {
	s, r := settingsFrames(t, sectionGeneral+"/window")
	s.scope = "pg"
	r.settle(t)
	if r.shows(i18n.T("settings.closeSetting.title")) || !r.shows(i18n.T("settings.settingsRow.allGlobal")) {
		t.Errorf("drawn: %s", r.drawn())
	}
}

// chatPrefFrames is the Claude Code screen over two profiles, Personal on Opus
// high and Work on Sonnet medium, with what the last chat offered.
func chatPrefFrames(t *testing.T) (*settingsScreen, *frameRig) {
	t.Helper()
	s, r, f := settingsFramesOn(t, sectionProviders)
	f.mu.Lock()
	f.settings["provider.claude.model.choices@"] = `[{"id":"opus","label":"Opus"},{"id":"sonnet","label":"Sonnet"},{"id":"haiku","label":"Haiku"}]`
	f.settings["provider.claude.model@"] = "opus"
	f.settings["provider.claude.thought_level.choices@"] = `[{"id":"low","label":"Low"},{"id":"medium","label":"Medium"},{"id":"high","label":"High"}]`
	f.settings["provider.claude.effort@"] = "high"
	f.mu.Unlock()
	if err := s.providers.book.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	book := newProfileBook(claudeProfileProvider, nil, r.invalidate, newPlanQuota(quotaReader(nil), r.invalidate))
	book.list = []backend.Profile{
		{Slug: personalProfile, Name: "Personal", Personal: true, LoggedIn: true, Email: "me@x.y", Plan: "max"},
		{Slug: "work", Name: "Work", LoggedIn: true, Email: "w@x.y", Plan: "team"},
	}
	book.loaded = true
	s.providers.claude.bind(book, nil)
	s.providers.claude.asked = true
	s.providers.claude.prefs = map[string]string{"provider.claude.model@work": "sonnet", "provider.claude.effort@work": "medium"}
	s.providers.open = claudeKind
	return s, r
}

// TestEachProfileShowsTheModelItsChatsStartOn: Global lists each profile with
// its own model and effort, and a project born in Work follows Work's, not
// Personal's.
func TestEachProfileShowsTheModelItsChatsStartOn(t *testing.T) {
	s, r := chatPrefFrames(t)
	r.settle(t)
	writeShot(t, r, "providers-global.png")
	for _, label := range []string{"Opus", "Sonnet", "High", "Medium"} {
		if !r.shows(label) {
			t.Fatalf("no %q in: %s", label, r.drawn())
		}
	}
	s.scope = "pg"
	s.values = scopedValues{claudeProfileKey: {"pg": "work"}}
	s.providers.book.setProjectProfile("pg", claudeProfileKey, "work")
	r.settle(t)
	writeShot(t, r, "providers-project.png")
	for _, value := range []string{"Sonnet", "Medium"} {
		follows := i18n.T("settings.settingsRow.followsFrom", i18n.Args{"name": "Work", "value": value})
		if !r.shows(follows) {
			t.Errorf("no %q in: %s", follows, r.drawn())
		}
	}
}

// TestAProfileRowsMenuOpensFromItsDots: the ⋯ a profile row shows under the
// pointer stays while the pointer is on it, and a click opens the row's menu.
// The row once lost its hover to the button over it, hid the button, found the
// pointer again and drew it back, every frame: the cursor flickered and no
// click landed.
func TestAProfileRowsMenuOpensFromItsDots(t *testing.T) {
	_, r := chatPrefFrames(t)
	r.settle(t)
	name := r.at(t, "Work")
	r.hover(name)
	y := int(name.Y)
	dots := r.smallestClickIn(t, image.Rect(rigWindow.X/2, y-12, rigWindow.X, y+48))
	r.hover(dots)
	r.frames(4)
	if _, ok := r.smallestClickAt(dots.Round()); !ok {
		t.Fatal("the ⋯ went away under the pointer")
	}
	r.click(t, dots)
	r.settle(t)
	if rename := i18n.T("settings.settingsProfilesView.rename"); !r.shows(rename) {
		t.Errorf("no %q after clicking the ⋯:\n%s", rename, r.drawn())
	}
}
