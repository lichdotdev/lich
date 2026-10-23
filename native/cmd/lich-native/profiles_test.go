package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestProfileChoiceValueNamesPersonalOnlyInAProject(t *testing.T) {
	cases := []struct{ slug, scope, want string }{
		{personalProfile, "", ""},
		{personalProfile, "p1", personalProfile},
		{"work", "", "work"},
		{"work", "p1", "work"},
	}
	for _, c := range cases {
		if got := profileChoiceValue(c.slug, c.scope); got != c.want {
			t.Errorf("profileChoiceValue(%q, %q) = %q, want %q", c.slug, c.scope, got, c.want)
		}
	}
}

func TestProfileAccountLine(t *testing.T) {
	cases := []struct {
		p    backend.Profile
		want string
	}{
		{backend.Profile{LoggedIn: true, Email: "a@b.c", Plan: "max", Plugins: 3}, "a@b.c · Max · 3 plugins"},
		{backend.Profile{LoggedIn: true, Email: "a@b.c", Plugins: 1}, "a@b.c · 1 plugin"},
		{backend.Profile{LoggedIn: true, Email: "a@b.c"}, "a@b.c · no plugins"},
		{backend.Profile{Email: "left@over"}, ""},
	}
	for _, c := range cases {
		if got := profileAccountLine(claudeProfileProvider, c.p); got != c.want {
			t.Errorf("profileAccountLine(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}

func TestProfileUsageQuotesTheFiveHourWindow(t *testing.T) {
	plan := backend.QuotaPlan{Status: "ok", Windows: []backend.QuotaWindow{
		{Seconds: 7 * day, Percent: 90},
		{Seconds: 5 * hour, Percent: 12},
	}}
	if text, limited := profileUsage(plan, true); text != "12% used" || limited {
		t.Errorf("profileUsage = %q, %v; want 12%% used", text, limited)
	}
	plan.Windows[1].Percent = 100
	if text, limited := profileUsage(plan, true); text != "limit" || !limited {
		t.Errorf("a spent window = %q, %v; want \"limit\", limited", text, limited)
	}
	if text, _ := profileUsage(backend.QuotaPlan{Status: "signed-out"}, true); text != "" {
		t.Errorf("a signed-out plan quoted %q", text)
	}
	if text, _ := profileUsage(plan, false); text != "" {
		t.Errorf("a missing reading quoted %q", text)
	}
}

func TestProfileBookNamesWhatItDoesNotKnow(t *testing.T) {
	b := newProfileBook(claudeProfileProvider, nil, func() {}, newPlanQuota(quotaReader(nil), func() {}))
	b.list = []backend.Profile{{Slug: personalProfile, Name: "Personal"}, {Slug: "work", Name: "Work"}}
	for stored, want := range map[string]string{"": "Personal", personalProfile: "Personal", "work": "Work", "gone": "gone"} {
		if got := b.nameOf(stored); got != want {
			t.Errorf("nameOf(%q) = %q, want %q", stored, got, want)
		}
	}
}

// The words name their keys at runtime, where the catalog scan cannot see
// them: every provider's must read in every language, as the dialog and the
// rows do.
func TestEveryProfileSentenceReadsInEveryLanguage(t *testing.T) {
	t.Cleanup(func() { i18n.SetCurrent(i18n.EN) })
	providers := append([]profileProvider{claudeProfileProvider, codexProfileProvider, ompProfileProvider}, xdgProfileProviders...)
	for _, locale := range i18n.Locales {
		i18n.SetCurrent(locale)
		for _, prov := range providers {
			w := prov.words
			for _, key := range []string{w.profileDescKey, w.profilesDescKey, w.copiedNoteKey, w.searchAlsoKey, w.profilesAlsoKey, w.newDescKey, w.renameDescKey, w.deleteDescKey, w.loginDescKey, w.addressDescKey} {
				if key != "" && profileSentence(prov, key) == "" {
					t.Errorf("%s %s: %q reads empty", locale, prov.kind, key)
				}
			}
		}
	}
}
