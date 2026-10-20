package main

import (
	"context"
	"testing"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
)

// profileFooterRig is a window with Claude and Codex installed, two Claude logins,
// and project p on screen with a Claude, a Codex and a shell card.
func profileFooterRig(t *testing.T) (*sidebarBackend, *model, *settingsScreen) {
	t.Helper()
	f, c := newSidebarBackend(t)
	twoInstalled(f)
	m, s := launchRig(t, f, c)
	s.providers.claude.bind(m.profiles, nil)
	m.profiles.list = []lichclient.Profile{
		{Slug: personalProfile, Name: "Personal", LoggedIn: true},
		{Slug: "work", Name: "Work", LoggedIn: true},
	}
	m.mu.Lock()
	m.projects = []lichclient.Project{{ID: "p", Name: "lich", Sessions: []lichclient.Session{
		{ID: "c", Kind: claudeKind}, {ID: "x", Kind: codexKind}, {ID: "sh", Kind: "shell"},
	}}}
	m.activeProject, m.active = "p", "c"
	m.providers.noteProjects(m.projects)
	m.mu.Unlock()
	s.scope = "p"
	return f, m, s
}

func activate(m *model, id string) {
	m.mu.Lock()
	m.active = id
	m.mu.Unlock()
}

func TestTheFooterNamesTheProfileOfTheActiveCardsProvider(t *testing.T) {
	_, m, _ := profileFooterRig(t)
	got, ok := m.focusedProfile()
	if !ok || got.kind != claudeKind || got.choice.shown != personalProfile {
		t.Fatalf("a Claude card: %+v, %v", got, ok)
	}
	activate(m, "x")
	if _, ok := m.focusedProfile(); ok {
		t.Fatal("a Codex card with one login shows a line")
	}
	activate(m, "sh")
	if got, ok := m.focusedProfile(); !ok || got.kind != claudeKind {
		t.Fatalf("a shell card follows the default provider: %+v, %v", got, ok)
	}
}

func TestTheFooterHidesWithASingleLogin(t *testing.T) {
	_, m, _ := profileFooterRig(t)
	m.profiles.list = m.profiles.list[:1]
	if got, ok := m.focusedProfile(); ok {
		t.Fatalf("one login still shows %+v", got)
	}
}

func TestAProjectProfileIsOneValueTheFooterAndSettingsSee(t *testing.T) {
	f, m, s := profileFooterRig(t)
	s.setProfile(&s.providers.claude, "work")
	if got, _ := m.focusedProfile(); got.choice.shown != "work" || !got.choice.own {
		t.Fatalf("the footer before the backend answered: %+v", got.choice)
	}
	waitForSetting(t, f, "provider.claude.profile@p", "work")
	if err := m.providers.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.focusedProfile(); got.choice.shown != "work" {
		t.Fatalf("the footer after a read: %+v", got.choice)
	}

	s.writeProviderKey(claudeProfileKey, "p", "", nil)
	if got, ok := s.snapshot().set(claudeProfileKey, "p"); ok {
		t.Fatalf("Settings still reads the project's own %q", got)
	}
	if got, _ := m.focusedProfile(); got.choice.shown != personalProfile || got.choice.own {
		t.Fatalf("a project back on Global: %+v", got.choice)
	}
	waitForSetting(t, f, "provider.claude.profile@p", "")
}

func TestAProjectOpenedAfterTheReadBringsItsProfile(t *testing.T) {
	f, m, _ := profileFooterRig(t)
	f.mu.Lock()
	f.settings["provider.claude.profile@late"] = "work"
	f.mu.Unlock()
	m.mu.Lock()
	m.projects = append(m.projects, lichclient.Project{ID: "late", Name: "late"})
	m.activeProject, m.active = "late", ""
	fresh := m.providers.noteProjects(m.projects)
	m.mu.Unlock()
	if len(fresh) != 1 || fresh[0] != "late" {
		t.Fatalf("new projects = %v", fresh)
	}
	m.providers.readProfiles(context.Background(), fresh)
	eventually(t, "the late project's profile", func() bool {
		got, _ := m.focusedProfile()
		return got.choice.shown == "work"
	})
}

func TestTheFooterMenuOffersGlobalOnlyWhileTheProjectSetsItsOwn(t *testing.T) {
	i18n.SetCurrent(i18n.EN)
	_, m, s := profileFooterRig(t)
	th := testTheme()
	var act sidebarAction
	f, _ := m.focusedProfile()
	if n := len(m.profileEntries(th, f, &act)); n != 2 {
		t.Fatalf("a project on Global lists %d entries", n)
	}
	s.setProfile(&s.providers.claude, "work")
	f, _ = m.focusedProfile()
	entries := m.profileEntries(th, f, &act)
	last := entries[len(entries)-1]
	if last.Label != "Use Global (Personal)" {
		t.Fatalf("last entry = %q", last.Label)
	}
	last.OnClick()
	if act.profilePick != (profilePick{key: claudeProfileKey, projectID: "p"}) {
		t.Fatalf("Use Global picked %+v", act.profilePick)
	}
	if !entries[1].Checked || entries[0].Checked {
		t.Fatal("Work is not the checked login")
	}
}
