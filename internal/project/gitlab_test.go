package project

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// The fixtures under testdata/gitlab are glab 1.131's own answers for
// gitlab.com/gitlab-org/cli: !4527 open from a fork, !4516 merged, !4524
// closed. The job lists keep only the fields lich reads; the trace is a failed
// job's, read from the job's raw log page.

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "gitlab", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fakeGLab answers each glab call by its arguments, joined by spaces, and
// records the calls. A call it has no answer for fails the test's read.
type fakeGLab struct {
	mu    sync.Mutex
	out   map[string][]byte
	errs  map[string]error
	calls []string
	dirs  []string
}

func newFakeGLab() *fakeGLab {
	return &fakeGLab{out: map[string][]byte{}, errs: map[string]error{}}
}

func (f *fakeGLab) on(call string, out []byte) *fakeGLab {
	f.out[call] = out
	return f
}

func (f *fakeGLab) fail(call string, err error) *fakeGLab {
	f.errs[call] = err
	return f
}

func (f *fakeGLab) run(_ time.Duration, dir string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := strings.Join(args, " ")
	f.calls = append(f.calls, call)
	f.dirs = append(f.dirs, dir)
	if err, ok := f.errs[call]; ok {
		return nil, err
	}
	if out, ok := f.out[call]; ok {
		return out, nil
	}
	return nil, errors.New("unexpected glab " + call)
}

func (f *fakeGLab) called(call string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.calls, call)
}

const (
	view4527      = "mr view 4527 -F json"
	commits4527   = "api projects/34675721/merge_requests/4527/commits?per_page=100"
	approvals4527 = "api projects/34675721/merge_requests/4527/approvals"
	jobs4527      = "api projects/45049979/pipelines/3099714877/jobs?per_page=100"
	diff4527      = "mr diff 4527 --raw"
)

func detailGLab(t *testing.T) *fakeGLab {
	return newFakeGLab().
		on(view4527, fixture(t, "mr4527.json")).
		on(commits4527, fixture(t, "commits4527.json")).
		on(approvals4527, fixture(t, "approvals4527.json")).
		on(jobs4527, fixture(t, "jobs4527.json")).
		on(diff4527, fixture(t, "diff4527.patch"))
}

func TestGitLabDetailReadsAMergeRequestInGitHubsWords(t *testing.T) {
	t.Parallel()
	d, err := gitlabForge{run: detailGLab(t).run}.pullRequestDetail("/w", 4527, false)
	if err != nil || d == nil {
		t.Fatalf("detail = %+v, %v", d, err)
	}
	want := PRDetail{
		Number: 4527, State: prOpen, Author: "Spiral-Memory", AuthorLogin: "Spiral-Memory",
		Mergeable: prMergeable, MergeStateStatus: prBlocked, ReviewDecision: prReviewRequired,
		Reviewers:   []PRReviewer{{Login: "jay_mccure"}, {Login: "GitLabDuo", State: prApproved}},
		BaseRefName: "main", HeadRefName: "feat/8154-ci-compile-job-script", ChangedFiles: 3,
		IsCrossRepository: true, MaintainerCanModify: true,
	}
	got := *d
	got.URL, got.Title, got.Body, got.Commits, got.CheckRuns = "", "", "", nil, nil
	got.Checks, got.Additions, got.Deletions = ChecksRollup{}, 0, 0
	if !reflect.DeepEqual(got, want) {
		t.Errorf("detail =\n%+v\nwant\n%+v", got, want)
	}
	if d.URL != "https://gitlab.com/gitlab-org/cli/-/merge_requests/4527" {
		t.Errorf("URL = %q", d.URL)
	}
	if d.Checks != (ChecksRollup{Passed: 13, Total: 13}) {
		t.Errorf("checks = %+v, want 13 passed: the failed-allowed manual job does not hold the merge", d.Checks)
	}
	if wantA, wantD := diffSize(string(fixture(t, "diff4527.patch"))); d.Additions != wantA || d.Deletions != wantD {
		t.Errorf("size = +%d -%d, want the diff's +%d -%d", d.Additions, d.Deletions, wantA, wantD)
	}
}

func TestGitLabDetailListsCommitsOldestFirst(t *testing.T) {
	t.Parallel()
	d, err := gitlabForge{run: detailGLab(t).run}.pullRequestDetail("/w", 4527, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Commits) != 2 {
		t.Fatalf("commits = %+v", d.Commits)
	}
	first := d.Commits[0]
	if first.Headline != "feat: add --job flag to extract job script in config compile" || first.OID[:8] != "2488e6c0" {
		t.Errorf("first commit = %+v, want the older one", first)
	}
	if first.Login != "" || first.Name == "" || first.Date == "" {
		t.Errorf("attribution = %+v: GitLab names no login", first)
	}
}

func TestGitLabDetailMarksAJobAllowedToFailNotRequired(t *testing.T) {
	t.Parallel()
	d, err := gitlabForge{run: detailGLab(t).run}.pullRequestDetail("/w", 4527, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.CheckRuns {
		if c.Required == (c.Name == "govulncheck") {
			t.Errorf("%s required = %v", c.Name, c.Required)
		}
	}
}

func TestGitLabBranchDetailIsOpenOnly(t *testing.T) {
	t.Parallel()
	g := newFakeGLab().on("mr view 4516 -F json", fixture(t, "mr4516.json"))
	d, err := gitlabForge{run: g.run}.pullRequestDetail("/w", 4516, true)
	if err != nil || d != nil {
		t.Errorf("open-only detail of a merged merge request = %+v, %v; want nil", d, err)
	}
	if len(g.calls) != 1 {
		t.Errorf("calls = %v, want the view alone", g.calls)
	}
}

func TestGitLabStatesReadAsGitHubs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file   string
		number int
		state  string
	}{
		{"mr4516.json", 4516, prMerged},
		{"mr4524.json", 4524, prClosed},
	} {
		g := newFakeGLab().on("mr view "+strconv.Itoa(tc.number)+" -F json", fixture(t, tc.file))
		head, err := gitlabForge{run: g.run}.pullRequestHead("/w", tc.number)
		if err != nil || head.RefName == "" || !head.CrossRepo {
			t.Errorf("%s head = %+v, %v", tc.file, head, err)
		}
		mr, err := gitlabForge{run: g.run}.view("/w", tc.number)
		if err != nil || mr.pullRequest().State != tc.state {
			t.Errorf("%s state = %q, %v; want %s", tc.file, mr.pullRequest().State, err, tc.state)
		}
	}
}

func TestGitLabMergeStatusReadsAsGitHubs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status    string
		conflicts bool
		mergeable string
		state     string
	}{
		{"mergeable", false, prMergeable, "CLEAN"},
		{"conflict", true, prConflicting, "DIRTY"},
		{"mergeable", true, prConflicting, "DIRTY"},
		{"need_rebase", false, prMergeable, "BEHIND"},
		{"draft_status", false, prMergeable, "DRAFT"},
		{"checking", false, prUnknown, prUnknown},
		{"unchecked", false, prUnknown, prUnknown},
		{"not_approved", false, prMergeable, prBlocked},
		{"ci_still_running", false, prMergeable, prBlocked},
		{"discussions_not_resolved", false, prMergeable, prBlocked},
	} {
		mr := glabMR{DetailedMergeStatus: tc.status, HasConflicts: tc.conflicts}
		if got := glabMergeable(mr); got != tc.mergeable {
			t.Errorf("%s/%v mergeable = %s, want %s", tc.status, tc.conflicts, got, tc.mergeable)
		}
		if got := glabMergeState(mr); got != tc.state {
			t.Errorf("%s/%v merge state = %s, want %s", tc.status, tc.conflicts, got, tc.state)
		}
	}
}

func TestGitLabApprovalsReadAsAReviewDecision(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		a    glabApprovals
		want string
	}{
		{"approvals still owed", glabApprovals{ApprovalsLeft: 2}, prReviewRequired},
		{"approved by someone", approvedBy(true, "ana"), prApproved},
		{"approved with nobody asked", glabApprovals{Approved: true}, ""},
	} {
		if got := approvalDecision(tc.a); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, got, tc.want)
		}
	}
	g := newFakeGLab().on("api projects/34675721/merge_requests/4516/approvals", fixture(t, "approvals4516.json"))
	if got := decision(gitlabForge{run: g.run}.review("/w", glabMR{IID: 4516, ProjectID: 34675721})); got != prApproved {
		t.Errorf("merged !4516 = %q, want APPROVED", got)
	}
	if got := decision(gitlabForge{run: newFakeGLab().run}.review("/w", glabMR{IID: 1})); got != "" {
		t.Errorf("a failed approvals read = %q, want none", got)
	}
}

func TestGitLabDetailFailsWhenAPartFails(t *testing.T) {
	t.Parallel()
	g := detailGLab(t).fail(jobs4527, errors.New("GitLab did not answer in time."))
	if _, err := (gitlabForge{run: g.run}).pullRequestDetail("/w", 4527, false); err == nil {
		t.Error("detail with its jobs unread = no error")
	}
}

func TestDiffSizeCountsInsideHunksOnly(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,2 +1,2 @@\n--- removed dashes\n+++ added pluses\n same\n"
	if a, d := diffSize(diff); a != 1 || d != 1 {
		t.Errorf("size = +%d -%d, want +1 -1", a, d)
	}
	a, d := diffSize(string(fixture(t, "diff4527.patch")))
	if a == 0 {
		t.Errorf("!4527 adds nothing: +%d -%d", a, d)
	}
}

func TestGitLabListMapsEachState(t *testing.T) {
	t.Parallel()
	for state, tc := range map[string]struct{ call, file, want string }{
		"open":   {"mr list --per-page 50 -F json", "list_open.json", prOpen},
		"merged": {"mr list --per-page 50 -F json --merged", "list_merged.json", prMerged},
		"closed": {"mr list --per-page 50 -F json --closed", "list_closed.json", prClosed},
	} {
		g := newFakeGLab().on(tc.call, fixture(t, tc.file))
		list, err := gitlabForge{run: g.run}.listPullRequests("/w", state)
		if err != nil || len(list) != 3 {
			t.Fatalf("%s list = %+v, %v", state, list, err)
		}
		for _, row := range list {
			if row.State != tc.want || row.Number == 0 || row.HeadRefName == "" || row.Checks.Total != 0 {
				t.Errorf("%s row = %+v", state, row)
			}
		}
	}
	g := newFakeGLab().on("mr list --per-page 50 -F json", fixture(t, "list_open.json"))
	list, _ := gitlabForge{run: g.run}.listPullRequests("/w", "open")
	if list[1].Number != 4526 || list[1].ReviewDecision != prReviewRequired {
		t.Errorf("!4526 (not_approved) = %+v, want REVIEW_REQUIRED", list[1])
	}
	g = newFakeGLab().on("mr list --per-page 50 -F json --all", []byte("[]"))
	if list, err := (gitlabForge{run: g.run}).listPullRequests("/w", "all"); list != nil || err != nil {
		t.Errorf("empty list = %v, %v", list, err)
	}
}

func TestGitLabMergeArgs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		opts api.MergePullRequestOptions
		want string
	}{
		{api.MergePullRequestOptions{Number: 9, Method: "squash"}, "mr merge 9 --yes --auto-merge=false --squash"},
		{api.MergePullRequestOptions{Number: 9, Method: "merge"}, "mr merge 9 --yes --auto-merge=false"},
		{api.MergePullRequestOptions{Number: 9, Method: "rebase", Subject: "s"}, "mr merge 9 --yes --auto-merge=false --rebase"},
		{
			api.MergePullRequestOptions{Number: 9, Method: "squash", Subject: "s", Body: "b"},
			"mr merge 9 --yes --auto-merge=false --squash --squash-message s\n\nb",
		},
		{api.MergePullRequestOptions{Number: 9, Method: "merge", Subject: "s"}, "mr merge 9 --yes --auto-merge=false --message s"},
	} {
		g := newFakeGLab().on(tc.want, nil)
		if err := (gitlabForge{run: g.run}).mergePullRequest("/w", tc.opts); err != nil {
			t.Errorf("%+v: %v (calls %q)", tc.opts, err, g.calls)
		}
	}
}

func TestGitLabMergeRefusesWhatItCannotDo(t *testing.T) {
	t.Parallel()
	g := newFakeGLab()
	if err := (gitlabForge{run: g.run}).mergePullRequest("/w", api.MergePullRequestOptions{Number: 9, Method: "ff"}); err == nil {
		t.Error("unknown method merged")
	}
	err := gitlabForge{run: g.run}.mergePullRequest("/w", api.MergePullRequestOptions{Number: 9, Method: "merge", Admin: true})
	if !errors.Is(err, errAdminMergeGitLab) {
		t.Errorf("admin merge = %v, want the refusal", err)
	}
	if len(g.calls) != 0 {
		t.Errorf("refused merges called glab: %v", g.calls)
	}
}

func TestGitLabCreateCheckoutAndDiff(t *testing.T) {
	t.Parallel()
	g := newFakeGLab().on("mr create --fill --web", nil).on("mr checkout 7", nil).on("mr diff 7 --raw", []byte("d"))
	f := gitlabForge{run: g.run}
	if err := f.createPullRequest("/w"); err != nil {
		t.Errorf("create: %v", err)
	}
	if err := f.checkoutPullRequest("/project", "/wt", 7); err != nil || g.dirs[1] != "/wt" {
		t.Errorf("checkout: %v in %q, want inside the worktree", err, g.dirs[1])
	}
	if diff, err := f.pullRequestDiff("/w", 7); diff != "d" || err != nil {
		t.Errorf("diff = %q, %v", diff, err)
	}
}

func TestGitLabAnswersThatCannotBeRead(t *testing.T) {
	t.Parallel()
	bad := []byte(`{not json`)
	mr := glabMR{IID: 4527, ProjectID: 34675721}
	for name, read := range map[string]func(gitlabForge) error{
		"view":    func(f gitlabForge) error { _, err := f.view("/w", 4527); return err },
		"list":    func(f gitlabForge) error { _, err := f.listPullRequests("/w", "open"); return err },
		"commits": func(f gitlabForge) error { _, err := f.commits("/w", mr); return err },
		"jobs": func(f gitlabForge) error {
			_, err := f.jobs("/w", &glabPipeline{ID: 3099714877, ProjectID: 45049979})
			return err
		},
	} {
		g := newFakeGLab().on(view4527, bad).on("mr list --per-page 50 -F json", bad).
			on(commits4527, bad).on(jobs4527, bad)
		if err := read(gitlabForge{run: g.run}); err == nil || !strings.Contains(err.Error(), "GitLab's answer") {
			t.Errorf("%s = %v, want the unreadable answer", name, err)
		}
	}
	g := newFakeGLab().on(approvals4527, bad)
	if got := decision(gitlabForge{run: g.run}.review("/w", mr)); got != "" {
		t.Errorf("unreadable approvals = %q", got)
	}
}

func decision(d string, _ []PRReviewer) string { return d }

func approvedBy(approved bool, logins ...string) glabApprovals {
	a := glabApprovals{Approved: approved}
	for _, l := range logins {
		a.ApprovedBy = append(a.ApprovedBy, struct {
			User glabUser `json:"user"`
		}{glabUser{Username: l}})
	}
	return a
}

func TestGitLabReviewersAreTheRequestedThenTheOtherApprovers(t *testing.T) {
	t.Parallel()
	requested := []glabUser{{Username: "ana"}, {Username: "bo"}}
	got := glabReviewers(requested, approvedBy(true, "bo", "cy"))
	want := []PRReviewer{{Login: "ana"}, {Login: "bo", State: prApproved}, {Login: "cy", State: prApproved}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roster = %+v, want %+v", got, want)
	}
	if got := glabReviewers(nil, glabApprovals{}); got != nil {
		t.Errorf("nobody asked or approved = %+v, want none", got)
	}
	_, roster := gitlabForge{run: newFakeGLab().run}.review("/w", glabMR{IID: 1, Reviewers: requested})
	if !reflect.DeepEqual(roster, []PRReviewer{{Login: "ana"}, {Login: "bo"}}) {
		t.Errorf("a failed approvals read = %+v, want the requested reviewers still owing", roster)
	}
}

func TestGitLabHeadWithoutASourceBranch(t *testing.T) {
	t.Parallel()
	g := newFakeGLab().on("mr view 3 -F json", []byte(`{"iid":3}`)).on("mr view 4 -F json", []byte(`{}`))
	if _, err := (gitlabForge{run: g.run}).pullRequestHead("/w", 3); err == nil || !strings.Contains(err.Error(), "no source branch") {
		t.Errorf("err = %v", err)
	}
	if _, err := (gitlabForge{run: g.run}).pullRequestHead("/w", 4); !errors.Is(err, errNoPullRequest) {
		t.Errorf("err = %v, want none", err)
	}
	if c, err := (gitlabForge{run: newFakeGLab().on(commits4527, []byte("[]")).run}).commits("/w", glabMR{IID: 4527, ProjectID: 34675721}); c != nil || err != nil {
		t.Errorf("no commits = %v, %v", c, err)
	}
}
