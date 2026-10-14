package main

import (
	"slices"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// TestXDGProfilesShowOnTheirOwnScreens: opencode, Cursor CLI and Kiro CLI each
// get the Profile and Profiles rows, a book read from their own service, and a
// login that only ever runs in a terminal card.
func TestXDGProfilesShowOnTheirOwnScreens(t *testing.T) {
	for _, kind := range []string{opencodeKind, cursorKind, kiroKind} {
		var rows int
		for _, e := range providerEntries() {
			if slices.Equal(e.onlyFor, []string{kind}) && (e.title == profileTitle || e.title == profilesTitle) {
				rows++
			}
		}
		if rows != 2 {
			t.Errorf("%s has %d profile rows, want 2", kind, rows)
		}
		prov, ok := profileProviderOf(kind)
		if !ok || prov.key != "provider."+kind+".profile" || !prov.terminalLogin || prov.plugins || prov.quotaPrefix != "" {
			t.Errorf("%s provider = %+v", kind, prov)
		}
	}
	if !slices.Equal(xdgProfileKinds(), []string{opencodeKind, cursorKind, kiroKind}) {
		t.Errorf("kinds = %v", xdgProfileKinds())
	}
}

// TestAnXDGProfileLogsInFromATerminal: the dialog goes from the name straight to
// a shell card running the command the provider's own service hands back.
func TestAnXDGProfileLogsInFromATerminal(t *testing.T) {
	b := &profileBackend{}
	c := b.client(t)
	var ran []string
	s := newSettingsScreen(t.Context(), c, new(ui.Root), func() {})
	books := map[string]*profileBook{kiroKind: newProfileBook(kiroProfileProvider, c, func() {}, newPlanQuota(quotaReader(c), func() {}))}
	s.providers.bindXDG(books, func(command string) bool { ran = append(ran, command); return true })
	ps := s.providers.profiles(kiroKind)
	if ps == nil {
		t.Fatal("no Kiro profiles on its screen")
	}
	s.startNewProfile(ps)
	ps.flow.name.SetText("Work")
	s.confirmFlow(ps, ps.flow.view())
	eventually(t, "the terminal card was opened and the dialog closed", func() bool {
		return !ps.flow.view().open && len(ran) == 1
	})
	want := []string{`kiroprofile.CreateProfile["Work"]`, `kiroprofile.LoginCommand["work"]`}
	if got := b.log(); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

// TestContinueInIsOfferedWhereTheConversationCanMove: Cursor CLI and Kiro CLI
// offer the other logged-in profiles; opencode, whose conversation lives in the
// database that holds the login, offers none.
func TestContinueInIsOfferedWhereTheConversationCanMove(t *testing.T) {
	m := newModel(nil, func() {})
	logged := []lichclient.Profile{{Slug: personalProfile, Name: "Personal", LoggedIn: true}, {Slug: "work", Name: "Work", LoggedIn: true}}
	for _, kind := range []string{opencodeKind, cursorKind, kiroKind} {
		m.xdgProfiles[kind].list = logged
	}
	for kind, want := range map[string]int{opencodeKind: 0, cursorKind: 1, kiroKind: 1} {
		if got := len(m.continueTargets(kind, "")); got != want {
			t.Errorf("%s offers %d profiles to continue in, want %d", kind, got, want)
		}
	}
	if got := m.profileNameOf(cursorKind, "work"); got != "Work" {
		t.Errorf("a Cursor card in work names %q", got)
	}
}
