package main

import (
	"testing"

	"github.com/omartelo/lich/native/lichclient"
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
		p    lichclient.Profile
		want string
	}{
		{lichclient.Profile{LoggedIn: true, Email: "a@b.c", Plan: "max", Plugins: 3}, "a@b.c · Max · 3 plugins"},
		{lichclient.Profile{LoggedIn: true, Email: "a@b.c", Plugins: 1}, "a@b.c · 1 plugin"},
		{lichclient.Profile{LoggedIn: true, Email: "a@b.c"}, "a@b.c · no plugins"},
		{lichclient.Profile{Email: "left@over"}, ""},
	}
	for _, c := range cases {
		if got := profileAccountLine(claudeProfileProvider, c.p); got != c.want {
			t.Errorf("profileAccountLine(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}

func TestProfileUsageQuotesTheFiveHourWindow(t *testing.T) {
	plan := lichclient.QuotaPlan{Status: "ok", Windows: []lichclient.QuotaWindow{
		{Seconds: 7 * day, Percent: 90},
		{Seconds: 5 * hour, Percent: 12},
	}}
	if text, limited := profileUsage(plan, true); text != "12% used" || limited {
		t.Errorf("profileUsage = %q, %v; want 12%% used", text, limited)
	}
	plan.Windows[1].Percent = 100
	if text, limited := profileUsage(plan, true); text != profileUsageLimit || !limited {
		t.Errorf("a spent window = %q, %v; want %q, limited", text, limited, profileUsageLimit)
	}
	if text, _ := profileUsage(lichclient.QuotaPlan{Status: "signed-out"}, true); text != "" {
		t.Errorf("a signed-out plan quoted %q", text)
	}
	if text, _ := profileUsage(plan, false); text != "" {
		t.Errorf("a missing reading quoted %q", text)
	}
}

func TestProfileBookNamesWhatItDoesNotKnow(t *testing.T) {
	b := newProfileBook(claudeProfileProvider, nil, func() {}, newPlanQuota(quotaReader(nil), func() {}))
	b.list = []lichclient.Profile{{Slug: personalProfile, Name: "Personal"}, {Slug: "work", Name: "Work"}}
	for stored, want := range map[string]string{"": "Personal", personalProfile: "Personal", "work": "Work", "gone": "gone"} {
		if got := b.nameOf(stored); got != want {
			t.Errorf("nameOf(%q) = %q, want %q", stored, got, want)
		}
	}
}
