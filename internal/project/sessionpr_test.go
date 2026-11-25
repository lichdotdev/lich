package project

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestParsePRRef(t *testing.T) {
	t.Parallel()
	good := map[string]prRef{
		"#1214":                            {number: 1214},
		" !12 ":                            {number: 12},
		"https://github.com/o/r/pull/1214": {forge: api.ForgeGitHub, number: 1214, url: "https://github.com/o/r/pull/1214"},
		"https://github.com/o/r/pull/1214/files?w=1#diff":    {forge: api.ForgeGitHub, number: 1214, url: "https://github.com/o/r/pull/1214"},
		"https://ghe.corp/o/r/pull/3":                        {forge: api.ForgeGitHub, number: 3, url: "https://ghe.corp/o/r/pull/3"},
		"https://gitlab.com/g/sub/p/-/merge_requests/9":      {forge: api.ForgeGitLab, number: 9, repo: "https://gitlab.com/g/sub/p"},
		"https://git.corp:8443/g/p/-/merge_requests/9/diffs": {forge: api.ForgeGitLab, number: 9, repo: "https://git.corp:8443/g/p"},
	}
	for in, want := range good {
		got, err := parsePRRef(in)
		if err != nil || got != want {
			t.Errorf("parsePRRef(%q) = %+v, %v, want %+v", in, got, err, want)
		}
	}
	for _, in := range []string{
		"", "1214", "#", "#0", "#-3", "#12a", "feat/branch",
		"https://github.com/o/r/issues/3", "https://github.com/o/r/pull/x", "https://github.com/o/r",
		"ftp://github.com/o/r/pull/3", "github.com/o/r/pull/3", "https:///o/r/pull/3",
		"https://gitlab.com/p/-/merge_requests/9", "https://gitlab.com/g/p/-/issues/9",
	} {
		if got, err := parsePRRef(in); err != errBadPRRef {
			t.Errorf("parsePRRef(%q) = %+v, %v, want refused", in, got, err)
		}
	}
}

// memLinks is the workspace store's side of the links, in memory.
type memLinks struct {
	mu   sync.Mutex
	rows []api.SessionPullRequest
}

func (m *memLinks) SessionPullRequests() ([]api.SessionPullRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := slices.Clone(m.rows)
	slices.SortFunc(out, func(a, b api.SessionPullRequest) int { return int(b.At - a.At) })
	return out, nil
}

func (m *memLinks) SaveSessionPullRequest(l api.SessionPullRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.rows, func(r api.SessionPullRequest) bool { return r.Session == l.Session && r.URL == l.URL })
	if i < 0 {
		m.rows = append(m.rows, l)
		return nil
	}
	if l.PR == nil {
		l.PR = m.rows[i].PR
	}
	m.rows[i] = l
	return nil
}

// linking is a Service on gh that keeps its links in memory and counts the
// lists it announces.
func linking(gh *fakeGH) (*Service, *memLinks, *int) {
	s := withGH(gh)
	links, emitted := &memLinks{}, new(int)
	s.SetSessionPullRequests(links)
	s.SetEmit(func(name string, _ any) {
		if name == api.SessionPullRequestsEvent.Name {
			*emitted++
		}
	})
	return s, links, emitted
}

const linkedOpen = `{"number":7,"url":"https://github.com/o/r/pull/7","title":"Fix","state":"OPEN"}`

func TestLinkPullRequestByNumberReadsTheCheckoutsRepo(t *testing.T) {
	t.Parallel()
	gh := &fakeGH{out: []byte(linkedOpen)}
	s, links, emitted := linking(gh)
	if _, err := s.LinkPullRequest(t.Context(), api.LinkPullRequestOptions{Session: "s1", Path: "/repo", Ref: "#7"}); err != nil {
		t.Fatalf("LinkPullRequest: %v", err)
	}
	if want := []string{"pr", "view", "7", "--json", prBadgeFields}; !slices.Equal(gh.args, want) || gh.dir != "/repo" {
		t.Errorf("gh %v in %q, want %v in /repo", gh.args, gh.dir, want)
	}
	got, _ := links.SessionPullRequests()
	if len(got) != 1 || !got[0].Linked || !got[0].Here || got[0].URL != "https://github.com/o/r/pull/7" || got[0].Forge != api.ForgeGitHub || got[0].PR.Title != "Fix" {
		t.Fatalf("links = %+v, want #7 linked here under its forge's URL", got)
	}
	if *emitted != 1 {
		t.Errorf("emitted %d lists, want 1", *emitted)
	}
}

func TestLinkPullRequestByURLIsNotHere(t *testing.T) {
	t.Parallel()
	gh := &fakeGH{out: []byte(linkedOpen)}
	s, links, _ := linking(gh)
	if _, err := s.LinkPullRequest(t.Context(), api.LinkPullRequestOptions{Session: "s1", Path: "/repo", Ref: "https://github.com/o/r/pull/7/files"}); err != nil {
		t.Fatalf("LinkPullRequest: %v", err)
	}
	if gh.args[2] != "https://github.com/o/r/pull/7" {
		t.Errorf("gh read %v, want the pull request's own URL", gh.args)
	}
	if got, _ := links.SessionPullRequests(); len(got) != 1 || got[0].Here {
		t.Errorf("links = %+v, want one not here", got)
	}
}

func TestLinkMergeRequestByURLReadsItsProject(t *testing.T) {
	t.Parallel()
	glab := newFakeGLab().on("mr view 9 -F json -R https://gitlab.com/g/p",
		[]byte(`{"iid":9,"state":"opened","title":"MR","web_url":"https://gitlab.com/g/p/-/merge_requests/9"}`))
	s, links, _ := linking(&fakeGH{})
	s.glab = glab.run
	if _, err := s.LinkPullRequest(t.Context(), api.LinkPullRequestOptions{Session: "s1", Path: "/repo", Ref: "https://gitlab.com/g/p/-/merge_requests/9"}); err != nil {
		t.Fatalf("LinkPullRequest: %v", err)
	}
	got, _ := links.SessionPullRequests()
	if len(got) != 1 || got[0].Forge != api.ForgeGitLab || got[0].PR.State != prOpen || got[0].URL != "https://gitlab.com/g/p/-/merge_requests/9" {
		t.Errorf("links = %+v, want the merge request, open", got)
	}
}

// Nothing is kept until the forge answers for the pull request.
func TestLinkPullRequestRefusals(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		gh  *fakeGH
		ref string
		err string
	}{
		"not a reference": {gh: &fakeGH{}, ref: "feat/x", err: errBadPRRef.Error()},
		"no such PR":      {gh: &fakeGH{err: errNoPullRequest}, ref: "#7", err: errNoLinkedPR.Error()},
		"empty answer":    {gh: &fakeGH{out: []byte(`{}`)}, ref: "#7", err: errNoLinkedPR.Error()},
		"forge fails":     {gh: &fakeGH{err: errTest}, ref: "#7", err: errTest.Error()},
	}
	for name, c := range cases {
		s, links, emitted := linking(c.gh)
		_, err := s.LinkPullRequest(t.Context(), api.LinkPullRequestOptions{Session: "s1", Path: "/repo", Ref: c.ref})
		if err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: err = %v, want %q", name, err, c.err)
		}
		if got, _ := links.SessionPullRequests(); len(got) != 0 || *emitted != 0 {
			t.Errorf("%s: kept %+v and emitted %d, want nothing", name, got, *emitted)
		}
	}
	if _, err := (&Service{}).LinkPullRequest(t.Context(), api.LinkPullRequestOptions{Ref: "#7"}); err != errNoLinks {
		t.Errorf("no store: err = %v, want %v", err, errNoLinks)
	}
	s := withGH(&fakeGH{})
	s.SetSessionPullRequests(&memLinks{})
	if _, err := s.LinkPullRequest(t.Context(), api.LinkPullRequestOptions{Ref: "https://gitlab.com/g/p/-/merge_requests/9"}); err != errNoGLab {
		t.Errorf("no glab: err = %v, want %v", err, errNoGLab)
	}
}

// An unlink needs no forge, and a link of the same URL undoes it.
func TestUnlinkThenLinkAgain(t *testing.T) {
	t.Parallel()
	gh := &fakeGH{out: []byte(linkedOpen)}
	s, links, emitted := linking(gh)
	if _, err := s.UnlinkPullRequest(t.Context(), api.UnlinkPullRequestOptions{Session: "s1", URL: "https://github.com/o/r/pull/7"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := links.SessionPullRequests(); len(got) != 1 || got[0].Linked || gh.calls != 0 {
		t.Fatalf("after unlink = %+v (gh %d calls), want one unlink and no read", got, gh.calls)
	}
	if _, err := s.LinkPullRequest(t.Context(), api.LinkPullRequestOptions{Session: "s1", Path: "/repo", Ref: "#7"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := links.SessionPullRequests(); len(got) != 1 || !got[0].Linked {
		t.Errorf("after linking again = %+v, want it linked", got)
	}
	if *emitted != 2 {
		t.Errorf("emitted %d lists, want 2", *emitted)
	}
	if _, err := (&Service{}).UnlinkPullRequest(t.Context(), api.UnlinkPullRequestOptions{}); err != errNoLinks {
		t.Errorf("no store: err = %v, want %v", err, errNoLinks)
	}
}

func TestRefreshLinkedPRs(t *testing.T) {
	t.Parallel()
	url := "https://github.com/o/r/pull/7"
	open := &PullRequest{Number: 7, URL: url, Title: "Fix", State: prOpen}
	gh := &fakeGH{outs: [][]byte{
		[]byte(`{"number":7,"url":"` + url + `","title":"Fix","state":"MERGED"}`),
		nil,
	}, errs: []error{nil, errTest}}
	s, links, emitted := linking(gh)
	s.SetSessionCheckout(func(id string) string {
		if id == "gone" {
			return ""
		}
		return "/repo"
	})
	rows := []api.SessionPullRequest{
		{Session: "s1", URL: url, Linked: true, At: 3, PR: open},
		{Session: "s2", URL: url, Linked: true, At: 2, PR: open},
		{Session: "gone", URL: url, Linked: true, At: 1, PR: open},
		{Session: "s3", URL: url, At: 0},
	}
	for _, r := range rows {
		_ = links.SaveSessionPullRequest(r)
	}
	s.refreshLinkedPRs()
	got, _ := links.SessionPullRequests()
	if got[0].PR.State != prMerged || got[0].Error != "" {
		t.Errorf("s1 = %+v, want its merge read", got[0])
	}
	if got[1].PR.State != prOpen || got[1].Error != errTest.Error() {
		t.Errorf("s2 = %+v, want the last read kept beside the failure", got[1])
	}
	if gh.calls != 2 || *emitted != 1 {
		t.Errorf("gh %d calls, %d lists, want 2 reads (no closed session, no unlink) and 1 list", gh.calls, *emitted)
	}
	// Nothing reads differently: no list.
	gh.outs, gh.errs, gh.calls, gh.err = nil, nil, 0, errTest
	s.refreshLinkedPRs()
	if gh.calls != 1 || *emitted != 1 {
		t.Errorf("second pass: gh %d calls, %d lists, want 1 read of the failed one, no list", gh.calls, *emitted)
	}
}

func TestSessionPullRequestsWithoutAStore(t *testing.T) {
	t.Parallel()
	got, err := (&Service{}).SessionPullRequests(t.Context(), api.None{})
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("SessionPullRequests = %v, %v, want an empty list", got, err)
	}
}
