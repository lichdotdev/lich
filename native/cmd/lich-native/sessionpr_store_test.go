package main

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestPickSessionPR(t *testing.T) {
	branch := &backend.PullRequest{Number: 1, URL: "u1"}
	two := &backend.PullRequest{Number: 2, URL: "u2"}
	three := &backend.PullRequest{Number: 3, URL: "u3"}
	link := func(pr *backend.PullRequest) backend.SessionPullRequest {
		return backend.SessionPullRequest{URL: pr.URL, Linked: true, PR: pr}
	}
	unlink := func(url string) backend.SessionPullRequest { return backend.SessionPullRequest{URL: url} }
	cases := []struct {
		name   string
		branch *backend.PullRequest
		links  []backend.SessionPullRequest
		shown  *backend.PullRequest
		linked bool
		others []*backend.PullRequest
	}{
		{name: "the branch's alone", branch: branch, shown: branch},
		{name: "none", shown: nil},
		{name: "the newest link wins over the branch's", branch: branch, links: []backend.SessionPullRequest{link(three), link(two)},
			shown: three, linked: true, others: []*backend.PullRequest{two, branch}},
		{name: "an unlinked branch PR is never brought back", branch: branch, links: []backend.SessionPullRequest{unlink("u1")}, shown: nil},
		{name: "an unlink of another leaves the branch's", branch: branch, links: []backend.SessionPullRequest{unlink("u9")}, shown: branch},
		{name: "unlinked branch PR stays out of the others", branch: branch, links: []backend.SessionPullRequest{link(two), unlink("u1")},
			shown: two, linked: true},
		{name: "the branch's linked by hand is not listed twice", branch: branch, links: []backend.SessionPullRequest{link(two), link(branch)},
			shown: two, linked: true, others: []*backend.PullRequest{branch}},
		{name: "a link with no read is skipped", links: []backend.SessionPullRequest{{URL: "u4", Linked: true}, link(two)}, shown: two, linked: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shown, l, others := pickSessionPR(c.branch, c.links)
			if shown != c.shown || (l != nil) != c.linked || !slices.Equal(others, c.others) {
				t.Errorf("= %+v, link %v, others %+v; want %+v, link %v, others %+v", shown, l != nil, others, c.shown, c.linked, c.others)
			}
		})
	}
}

// A hand link reaches the composer's line through the window's store, and an
// unlink of the branch's takes it off.
func TestTheComposerShowsWhatTheSessionLinked(t *testing.T) {
	u, r, _ := composerWithPR(t, rigWindow, backend.PullRequest{Number: 1187, URL: "https://github.com/o/r/pull/1187", State: "OPEN"}, api.ForgeGitHub, "s1")
	id := u.model.projects[0].Sessions[0].ID
	linked := &backend.PullRequest{Number: 1214, URL: "https://gitlab.com/g/p/-/merge_requests/1214", State: "OPEN", Title: "Fix"}
	u.model.setSessionPRs([]backend.SessionPullRequest{{Session: id, URL: linked.URL, Linked: true, Forge: api.ForgeGitLab, PR: linked}})
	r.settle(t)
	if !r.shows("!1214") || r.shows("#1187") {
		t.Fatal("the composer did not swap the branch's PR for the linked merge request")
	}
	u.model.setSessionPRs([]backend.SessionPullRequest{{Session: id, URL: "https://github.com/o/r/pull/1187"}})
	r.settle(t)
	if r.shows("#1187") {
		t.Error("the composer drew the branch's PR the session unlinked")
	}
}

func TestLinkPRItems(t *testing.T) {
	var act sidebarAction
	sv := &sessionView{ID: "s1", forge: api.ForgeGitHub}
	items := linkPRItems(sv, &act)
	if len(items) != 1 {
		t.Fatalf("items = %v, want Link alone without a pull request", labels(items))
	}
	items[0].OnClick()
	if act.linkPR != "s1" {
		t.Errorf("Link asked %q, want s1", act.linkPR)
	}
	sv.pr = &backend.PullRequest{Number: 9, URL: "https://gitlab.com/g/p/-/merge_requests/9"}
	sv.prLink = &backend.SessionPullRequest{Forge: api.ForgeGitLab}
	unlink := entry(t, linkPRItems(sv, &act), i18n.T("sidebar.sessionCard.unlinkMergeRequest"))
	unlink.OnClick()
	if act.unlinkPR != (prUnlink{id: "s1", url: sv.pr.URL}) {
		t.Errorf("Unlink asked %+v, want s1 and its URL", act.unlinkPR)
	}
}

func TestSessionPRTarget(t *testing.T) {
	pr := &backend.PullRequest{Number: 7, URL: "https://github.com/o/r/pull/7"}
	cases := []struct {
		name   string
		link   *backend.SessionPullRequest
		shown  string
		number int
		url    string
	}{
		{"the branch's, in its checkout", nil, "/wt", 7, ""},
		{"linked by number", &backend.SessionPullRequest{Here: true}, "/elsewhere", 7, ""},
		{"linked by URL", &backend.SessionPullRequest{}, "/wt", 0, pr.URL},
	}
	for _, c := range cases {
		sv := sessionView{path: "/wt", shown: c.shown, pr: pr, prLink: c.link}
		if n, url := sv.prTarget(); n != c.number || url != c.url {
			t.Errorf("%s: = %d, %q; want %d, %q", c.name, n, url, c.number, c.url)
		}
	}
}

func TestSessionPRTip(t *testing.T) {
	sv := sessionView{forge: api.ForgeGitHub, pr: &backend.PullRequest{Title: "Fix"},
		prLink:   &backend.SessionPullRequest{Forge: api.ForgeGitHub, Error: "gh: not found"},
		prOthers: []*backend.PullRequest{{Number: 3}, {Number: 4}}}
	want := "Fix\nCould not read it: gh: not found\nAlso on this session: #3, #4"
	if got := sv.prTip(); got != want {
		t.Errorf("prTip = %q, want %q", got, want)
	}
	if got := (&sessionView{pr: &backend.PullRequest{}}).prTip(); got != "" {
		t.Errorf("prTip with no title = %q, want none", got)
	}
}
