package main

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestUpgradeAccountOnlyWithoutGuessing(t *testing.T) {
	accounts := []string{"github.com/ana", "ghe.corp/bob", "github.com/bob"}
	cases := []struct{ stored, want string }{
		{"ana", "github.com/ana"},
		{"bob", ""},            // on two hosts: only the user can say which
		{"carol", ""},          // gh knows no such login
		{"github.com/ana", ""}, // already carries its host
		{"", ""},
	}
	for _, c := range cases {
		if got := upgradeAccount(c.stored, accounts); got != c.want {
			t.Errorf("upgradeAccount(%q) = %q, want %q", c.stored, got, c.want)
		}
	}
}

func TestAccountLabelNamesTheHostOnlyWhenItTellsApart(t *testing.T) {
	single := []string{"github.com/ana", "github.com/bob"}
	mixed := []string{"github.com/ana", "ghe.corp/bob"}
	cases := []struct {
		account  string
		accounts []string
		want     string
	}{
		{"github.com/ana", single, "ana"},
		{"github.com/ana", mixed, "ana — github.com"},
		{"ghe.corp/bob", []string{"ghe.corp/bob"}, "bob — ghe.corp"},
		{"ana", single, "ana"},
	}
	for _, c := range cases {
		if got := accountLabel(c.account, c.accounts); got != c.want {
			t.Errorf("accountLabel(%q) = %q, want %q", c.account, got, c.want)
		}
	}
}

func TestAccountOptionsKeepAStaleStoredAccount(t *testing.T) {
	gh := []string{"github.com/ana"}
	if got := accountOptions(gh, "github.com/gone"); !slices.Equal(got, []string{"github.com/ana", "github.com/gone"}) {
		t.Errorf("stale account: %v", got)
	}
	if got := accountOptions(gh, "github.com/ana"); !slices.Equal(got, gh) {
		t.Errorf("listed account repeated: %v", got)
	}
	if got := accountOptions(nil, ""); len(got) != 0 {
		t.Errorf("nothing stored, nothing listed: %v", got)
	}
}

func TestIdentityLineSaysWhereTheEmailComesFrom(t *testing.T) {
	cases := []struct {
		id   backend.CommitIdentity
		want identityLine
	}{
		{backend.CommitIdentity{}, identityLine{"No git identity in this checkout.", "", "Commits will be refused until user.email is set."}},
		{backend.CommitIdentity{Name: "Ana", Email: "a@x.io"}, identityLine{"Commits land as Ana", "<a@x.io>", "· git user.email, not this account."}},
		{backend.CommitIdentity{Email: "a@x.io", Local: true}, identityLine{"Commits land as", "<a@x.io>", "· set in this repository, overriding your global one."}},
	}
	for _, c := range cases {
		if got := identityLineOf(c.id); got != c.want {
			t.Errorf("identityLineOf(%+v) = %+v, want %+v", c.id, got, c.want)
		}
	}
}

func TestVersionControlRowsFollowTheScope(t *testing.T) {
	sec := sectionByID(sectionVersionControl)
	if len(sec.rows) != 2 {
		t.Fatalf("rows = %v", sec.rows)
	}
	tools, account := sec.rows[0], sec.rows[1]
	if got := rowKindOf(tools, "pg", openTabs, nil); got != rowGlobalOnly {
		t.Errorf("tools from a project = %d, want global only", got)
	}
	if got := rowKindOf(account, "", openTabs, nil); got != rowHidden {
		t.Errorf("account from Global = %d, want hidden", got)
	}
	if got := rowKindOf(account, "pg", openTabs, nil); got != rowProjectOnly {
		t.Errorf("account from a project = %d, want project only", got)
	}
	if hits := searchSettings("gh"); !slices.ContainsFunc(hits, func(h settingHit) bool { return h.title == "GitHub account" }) {
		t.Errorf("searching gh misses the account: %v", hits)
	}
}

// Each tool's row says what it lacks and offers the command that fixes it:
// the package manager's for a missing one, the sign-in for a CLI with no
// account, the form for git without a global name or email.
func TestToolRowsOfferWhatEachToolLacks(t *testing.T) {
	v := &vcsSettings{checks: map[string]backend.BinaryCheck{
		vcsGitBin:  {Path: "/usr/bin/git", Status: backend.CheckOK},
		vcsGhBin:   {Status: "notFound"},
		vcsGlabBin: {Path: "/usr/bin/glab", Status: backend.CheckOK},
	}}
	v.setup.installs = map[string]string{vcsGhBin: "sudo pacman -S --needed github-cli"}
	v.setup.glabAccounts = []string{}
	v.setup.global = &backend.CommitIdentity{Name: "Ana"}
	if st := v.toolState(vcsGh); !st.gone || st.install != "sudo pacman -S --needed github-cli" {
		t.Errorf("missing gh: %+v", st)
	}
	if st := v.toolState(vcsGlab); st.signIn != vcsGlabSignIn || !st.alarm {
		t.Errorf("glab with no account: %+v", st)
	}
	if st := v.toolState(vcsGit); !st.identity || st.resolved != "/usr/bin/git" {
		t.Errorf("git with no email: %+v", st)
	}
	v.setup.glabAccounts = []string{"gitlab.com/ana.souza"}
	v.setup.global = &backend.CommitIdentity{Name: "Ana", Email: "ana@example.dev"}
	if st := v.toolState(vcsGlab); st.status != "ana.souza" || st.signIn != "" || st.alarm {
		t.Errorf("glab signed in: %+v", st)
	}
	if st := v.toolState(vcsGit); st.identity || st.status != "Ana · ana@example.dev" {
		t.Errorf("git with an identity: %+v", st)
	}
}
