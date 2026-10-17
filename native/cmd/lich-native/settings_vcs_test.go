package main

import (
	"slices"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
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
		id   lichclient.CommitIdentity
		want identityLine
	}{
		{lichclient.CommitIdentity{}, identityLine{"No git identity in this checkout.", "", "Commits will be refused until user.email is set."}},
		{lichclient.CommitIdentity{Name: "Ana", Email: "a@x.io"}, identityLine{"Commits land as Ana", "<a@x.io>", "— git user.email, not this account."}},
		{lichclient.CommitIdentity{Email: "a@x.io", Local: true}, identityLine{"Commits land as", "<a@x.io>", "— set in this repository, overriding your global one."}},
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
