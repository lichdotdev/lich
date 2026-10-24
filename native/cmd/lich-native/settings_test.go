package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/i18n"
)

var (
	overridable = settingEntry{key: "sandbox.ssh-agent", scope: scopeOverridable}
	globalOnly  = settingEntry{key: diffLayoutKey}
	projectOnly = settingEntry{key: "vcs.account", scope: scopeProjectOnly}
	openTabs    = []settingsProject{{"lich", "lich"}, {"pg", "lich-playground"}, {"ij", "lich-intellij"}}
)

func TestRowStateFollowsTheScope(t *testing.T) {
	values := scopedValues{"sandbox.ssh-agent": {"pg": "true"}, "vcs.account": {"pg": "omartelo"}}
	cases := []struct {
		entry settingEntry
		scope string
		want  rowKind
	}{
		{overridable, "", rowOverridden},
		{overridable, "pg", rowChanged},
		{overridable, "lich", rowFollows},
		{globalOnly, "", rowPlain},
		{globalOnly, "pg", rowGlobalOnly},
		{projectOnly, "", rowHidden},
		{projectOnly, "lich", rowProjectOnly},
	}
	for _, c := range cases {
		if got := rowKindOf(c.entry, c.scope, openTabs, values); got != c.want {
			t.Errorf("%s from %q = %d, want %d", c.entry.key, c.scope, got, c.want)
		}
	}
	if got := rowKindOf(overridable, "", openTabs, scopedValues{}); got != rowPlain {
		t.Errorf("a row nobody overrides = %d, want plain", got)
	}
}

func TestOverriddenInNamesEveryProjectInTabOrder(t *testing.T) {
	values := scopedValues{"sandbox.ssh-agent": {"ij": "false", "pg": "true"}}
	got := overriders(overridable, openTabs, values)
	if !slices.Equal(got, []string{"lich-playground", "lich-intellij"}) {
		t.Fatalf("overriders = %v", got)
	}
}

func TestBadgesCountOverridesNotProjectOnlyRows(t *testing.T) {
	values := scopedValues{
		"sandbox.ssh-agent": {"pg": "true", "ij": "true"},
		"sandbox.gh-token":  {"ij": "true"},
		"vcs.account":       {"pg": "omartelo"},
	}
	sandbox, vcs := sectionByID(sectionSandbox), sectionByID(sectionVersionControl)
	if got := sectionBadge(sandbox, "", openTabs, values); got != 2 {
		t.Errorf("Sandbox from Global = %d, want 2 rows overridden somewhere", got)
	}
	if got := sectionBadge(sandbox, "pg", openTabs, values); got != 1 {
		t.Errorf("Sandbox from lich-playground = %d, want its 1 change", got)
	}
	if got := sectionBadge(vcs, "pg", openTabs, values); got != 0 {
		t.Errorf("Version Control = %d: a project-only value overrides nothing", got)
	}
	if got := projectChanges("ij", values); got != 2 {
		t.Errorf("lich-intellij changes = %d, want 2", got)
	}
	if changesLabel(1) != "1 change" || changesLabel(2) != "2 changes" || changesLabel(0) != "" {
		t.Errorf("labels: %q %q %q", changesLabel(0), changesLabel(1), changesLabel(2))
	}
}

func TestOnlyProjectScopedSectionsStayLitInAProject(t *testing.T) {
	var lit []string
	for _, s := range settingsSections {
		if s.projectScoped() {
			lit = append(lit, s.id)
		}
	}
	if !slices.Equal(lit, []string{sectionProviders, sectionSandbox, sectionVersionControl, sectionServices}) {
		t.Fatalf("lit in a project: %v", lit)
	}
}

func TestSearchRanksPrefixThenContainsThenAlso(t *testing.T) {
	titles := func(q string) []string {
		var out []string
		for _, h := range searchSettings(q) {
			out = append(out, h.title)
		}
		return out
	}
	if got := titles("diff"); !slices.Equal(got, []string{"Diff layout", "Hide whitespace in diffs"}) {
		t.Errorf("diff: %v", got)
	}
	if got := titles("  SPLIT "); !slices.Equal(got, []string{"Diff layout"}) {
		t.Errorf("split, found by its also words: %v", got)
	}
	if got := searchSettings("sand"); len(got) != 1 || got[0].section != sectionSandbox || got[0].path() != "" {
		t.Errorf("sand: %+v; a section is its own result, with no path", got)
	}
	if got := searchSettings("whitespace"); len(got) != 1 || got[0].path() != "Editor" {
		t.Errorf("whitespace: %+v", got)
	}
	if got := searchSettings("finishes"); len(got) != 1 || got[0].path() != "General › Notifications" {
		t.Errorf("finishes: %+v; a grouped row's path names its group", got)
	}
	if got := searchSettings("notifications"); len(got) != 2 || got[0].title != "Notify me when a session needs input" || got[1].title != "Notify me when a session finishes working" {
		t.Errorf("notifications: %+v; a group's name finds its rows", got)
	}
	if searchSettings("  ") != nil {
		t.Error("a blank query has results")
	}
}

func TestRememberedScopeFallsBackToGlobalOnceItsProjectCloses(t *testing.T) {
	if got := resolveScope("pg", openTabs); got != "pg" {
		t.Errorf("open project: %q", got)
	}
	if got := resolveScope("gone", openTabs); got != "" {
		t.Errorf("closed project: %q, want Global", got)
	}
}

func TestSettingsRemembersTheSectionInTheBackend(t *testing.T) {
	f, client := newSidebarBackend(t)
	s := newSettingsScreen(context.Background(), client, nil, func() {})
	if err := s.load(); err != nil || s.section != defaultSection {
		t.Fatalf("nothing stored: %q, %v", s.section, err)
	}
	f.settings[settingsSectionKey+"@"] = "provider-claude"
	if err := s.load(); err != nil || s.section != defaultSection {
		t.Fatalf("an id this build has no section for: %q, %v", s.section, err)
	}
	s.openSection(sectionSandbox)
	waitForSetting(t, f, settingsSectionKey+"@", sectionSandbox)
	again := newSettingsScreen(context.Background(), client, nil, func() {})
	if err := again.load(); err != nil || again.section != sectionSandbox {
		t.Fatalf("reopened on %q, %v; want the section left open", again.section, err)
	}
}

func TestUseGlobalClearsTheProjectValue(t *testing.T) {
	f, client := newSidebarBackend(t)
	f.settings["sandbox.ssh-agent@pg"] = "true"
	s := newSettingsScreen(context.Background(), client, nil, func() {})
	values, err := loadScopedValues(context.Background(), client, openTabs)
	if err != nil {
		t.Fatal(err)
	}
	s.values = values
	if _, ok := s.snapshot().set("sandbox.ssh-agent", "pg"); !ok {
		t.Fatal("the seeded override was not read")
	}
	s.useGlobal("sandbox.ssh-agent", "pg")
	if _, ok := s.snapshot().set("sandbox.ssh-agent", "pg"); ok {
		t.Error("the row still reads as changed")
	}
	waitForSetting(t, f, "sandbox.ssh-agent@pg", "")
}

func TestEditorPrefsDefaultToUnifiedAndShownWhitespace(t *testing.T) {
	f, client := newSidebarBackend(t)
	p, err := loadEditorPrefs(context.Background(), client)
	if err != nil || p.hideWhitespace || p.split() {
		t.Fatalf("nothing stored: %+v, %v", p, err)
	}
	f.settings[diffLayoutKey+"@"], f.settings[hideWhitespaceKey+"@"] = diffSplit, "true"
	if p, err = loadEditorPrefs(context.Background(), client); err != nil || !p.hideWhitespace || !p.split() {
		t.Fatalf("stored split and hidden: %+v, %v", p, err)
	}
	f.settings[diffLayoutKey+"@"] = "columns"
	if p, _ = loadEditorPrefs(context.Background(), client); p.split() {
		t.Error("a foreign layout value draws split")
	}
}

func waitForSetting(t *testing.T, f *sidebarBackend, key, want string) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		f.mu.Lock()
		got, ok := f.settings[key]
		f.mu.Unlock()
		if ok && got == want {
			return
		}
	}
	t.Fatalf("setting %s never became %q", key, want)
}

func TestSearchAndLabelsFollowTheInterfaceLanguage(t *testing.T) {
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	hits := searchSettings("indentação")
	if len(hits) != 1 || hits[0].title != "Ocultar espaços em branco nos diffs" || hits[0].path() != "Editor" {
		t.Errorf("indentação: %+v; the also words are read in the interface language", hits)
	}
	if got := searchSettings("notificações"); len(got) != 2 || got[0].path() != "Geral › Notificações" {
		t.Errorf("notificações: %+v", got)
	}
	if changesLabel(1) != "1 mudança" || changesLabel(2) != "2 mudanças" {
		t.Errorf("labels: %q %q", changesLabel(1), changesLabel(2))
	}
}
