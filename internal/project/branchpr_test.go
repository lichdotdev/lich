package project

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// forgePRs answers gh's branch pull request read with what each checkout's
// branch has at the moment, and counts the reads.
type forgePRs struct {
	mu    sync.Mutex
	out   map[string]string
	err   error
	reads map[string]int
	hold  chan struct{}
}

func newForgePRs() *forgePRs { return &forgePRs{out: map[string]string{}, reads: map[string]int{}} }

func (f *forgePRs) set(path, out string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.out[path] = out
}

func (f *forgePRs) run(_ time.Duration, dir, _ string, _ ...string) ([]byte, error) {
	f.mu.Lock()
	hold := f.hold
	f.mu.Unlock()
	if hold != nil {
		<-hold
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads[dir]++
	if f.err != nil {
		return nil, f.err
	}
	if f.out[dir] == "" {
		return nil, errNoPullRequest
	}
	return []byte(f.out[dir]), nil
}

func (f *forgePRs) readsOf(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads[path]
}

const (
	featOpen   = `{"number":42,"url":"u","title":"Add merge notice","state":"OPEN","headRefName":"feat/merge","baseRefName":"main"}`
	featMerged = `{"number":42,"url":"u","title":"Add merge notice","state":"MERGED","headRefName":"feat/merge","baseRefName":"main"}`
	featClosed = `{"number":42,"url":"u","title":"Add merge notice","state":"CLOSED","headRefName":"feat/merge","baseRefName":"main"}`
)

// watching is a Service on forge that records every BranchPullRequestEvent and
// every merge it announces.
type watching struct {
	*Service
	mu      sync.Mutex
	events  []api.BranchPullRequest
	merges  []api.AnnounceMergeOptions
	mergeOK error
}

func newWatching(forge *forgePRs) *watching {
	w := &watching{Service: &Service{runner: forge.run}}
	w.SetEmit(func(name string, data any) {
		if name == api.BranchPullRequestEvent.Name {
			w.mu.Lock()
			w.events = append(w.events, data.(api.BranchPullRequest))
			w.mu.Unlock()
		}
	})
	w.SetAnnounceMerge(func(_ context.Context, opts api.AnnounceMergeOptions) (api.None, error) {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.merges = append(w.merges, opts)
		return api.None{}, w.mergeOK
	})
	return w
}

func (w *watching) merged() []api.AnnounceMergeOptions {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.merges)
}

func TestBranchPullRequestKeepsAMergedOne(t *testing.T) {
	t.Parallel()
	gh := &fakeGH{out: []byte(featMerged)}
	pr, err := withGH(gh).BranchPullRequest("/repo")
	if err != nil {
		t.Fatalf("BranchPullRequest: %v", err)
	}
	want := PullRequest{Number: 42, URL: "u", Title: "Add merge notice", State: "MERGED", HeadRefName: "feat/merge", BaseRefName: "main"}
	if pr == nil || *pr != want {
		t.Fatalf("BranchPullRequest = %+v, want %+v", pr, want)
	}
	if want := []string{"pr", "view", "--json", "number,url,title,state,headRefName,baseRefName"}; !slices.Equal(gh.args, want) {
		t.Errorf("args = %v, want %v", gh.args, want)
	}
	if gh.dir != "/repo" || gh.timeout != prLookupTimeout {
		t.Errorf("wrong scope: dir %q timeout %v", gh.dir, gh.timeout)
	}

	for name, gh := range map[string]*fakeGH{"no PR": {err: errNoPullRequest}, "empty object": {out: []byte(`{}`)}} {
		if pr, err := withGH(gh).BranchPullRequest("/repo"); err != nil || pr != nil {
			t.Errorf("%s = %+v, %v, want nil, nil", name, pr, err)
		}
	}
	if _, err := withGH(&fakeGH{err: errTest}).BranchPullRequest("/repo"); !errors.Is(err, errTest) {
		t.Errorf("failed lookup err = %v, want %v", err, errTest)
	}
	if _, err := withGH(&fakeGH{out: []byte(`not json`)}).BranchPullRequest("/repo"); err == nil {
		t.Error("malformed answer: want a decode error")
	}
}

// The badge read runs as the project's account, like every other GitHub call.
func TestBranchPullRequestRunsAsTheProjectsAccount(t *testing.T) {
	t.Parallel()
	account := &accountGH{token: "gho_secret", pr: featOpen}
	svc := &Service{runner: account.run}
	svc.SetAccounts(func(string) string { return "octocat" })
	if pr, err := svc.BranchPullRequest("/repo"); err != nil || pr == nil {
		t.Fatalf("BranchPullRequest = %+v, %v, want the open PR", pr, err)
	}
	if account.viewToken != "GH_TOKEN=gho_secret" {
		t.Errorf("read ran with token %q, want the configured account's", account.viewToken)
	}
}

func TestEveryReadIsReported(t *testing.T) {
	t.Parallel()
	forge := newForgePRs()
	w := newWatching(forge)
	w.readBranchPR("/wt/feat")
	forge.set("/wt/feat", featOpen)
	w.readBranchPR("/wt/feat")
	w.readBranchPR("/wt/feat")

	if len(w.events) != 3 || w.events[0].PR != nil || w.events[1].PR == nil || w.events[2].PR == nil {
		t.Fatalf("events = %+v, want none, then the open PR twice", w.events)
	}
	if w.events[1].Path != "/wt/feat" || w.events[1].PR.Number != 42 {
		t.Fatalf("event = %+v, want #42 at /wt/feat", w.events[1])
	}
}

func TestAMergeAfterAnOpenReadIsAnnounced(t *testing.T) {
	t.Parallel()
	forge := newForgePRs()
	forge.set("/wt/feat", featOpen)
	w := newWatching(forge)
	w.readBranchPR("/wt/feat")
	forge.set("/wt/feat", featMerged)
	w.readBranchPR("/wt/feat")
	w.readBranchPR("/wt/feat")

	want := []api.AnnounceMergeOptions{{Checkout: "/wt/feat", Number: 42, Title: "Add merge notice", Branch: "feat/merge", Base: "main"}}
	if got := w.merged(); !slices.Equal(got, want) {
		t.Fatalf("merges = %+v, want %+v", got, want)
	}
}

func TestAMergeFirstReadMergedOrAfterACloseIsNotAnnounced(t *testing.T) {
	t.Parallel()
	forge := newForgePRs()
	forge.set("/wt/a", featMerged)
	forge.set("/wt/b", featOpen)
	w := newWatching(forge)
	w.readBranchPR("/wt/a")
	w.readBranchPR("/wt/b")
	forge.set("/wt/b", featClosed)
	w.readBranchPR("/wt/b")
	forge.set("/wt/b", featMerged)
	w.readBranchPR("/wt/b")

	if got := w.merged(); len(got) != 0 {
		t.Fatalf("merges = %+v, want none", got)
	}
}

func TestAFailedReadKeepsWhatTheLastOneSaw(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	forge := newForgePRs()
	forge.set(dir, featOpen)
	w := newWatching(forge)
	// The announce failing is only logged: the merge already happened.
	w.mergeOK = errors.New("disk gone")
	w.readBranchPR(dir)
	forge.mu.Lock()
	forge.err = errors.New("gh is not signed in to GitHub.")
	forge.mu.Unlock()
	w.readBranchPR(dir)
	if len(w.events) != 1 {
		t.Fatalf("events = %+v, want only the read that answered", w.events)
	}
	forge.mu.Lock()
	forge.err = nil
	forge.mu.Unlock()
	forge.set(dir, featMerged)
	w.readBranchPR(dir)

	if got := w.merged(); len(got) != 1 {
		t.Fatalf("merges = %+v, want the one merge", got)
	}
}

func TestTheWatchFollowsAgentsAndOpenPullRequests(t *testing.T) {
	t.Parallel()
	forge := newForgePRs()
	forge.set("/wt/open", featOpen)
	forge.set("/wt/merged", featMerged)
	w := newWatching(forge)
	w.SetAgentCheckouts(func() []string { return []string{"/wt/agent", "/wt/open"} })
	for _, path := range []string{"/wt/open", "/wt/merged", "/wt/none"} {
		w.readBranchPR(path)
	}

	got := w.followedCheckouts()
	slices.Sort(got)
	if want := []string{"/wt/agent", "/wt/open"}; !slices.Equal(got, want) {
		t.Fatalf("followed = %v, want %v", got, want)
	}

	w.SetAgentCheckouts(nil)
	if got := w.followedCheckouts(); !slices.Equal(got, []string{"/wt/open"}) {
		t.Fatalf("followed with no agent = %v, want the open PR's checkout", got)
	}
}

func TestAGoneCheckoutIsNoLongerFollowed(t *testing.T) {
	t.Parallel()
	here, gone := t.TempDir(), t.TempDir()+"/removed"
	forge := newForgePRs()
	forge.set(here, featOpen)
	forge.set(gone, featOpen)
	w := newWatching(forge)
	w.readBranchPR(here)
	w.readBranchPR(gone)
	forge.mu.Lock()
	forge.err = errors.New("gh failed")
	forge.mu.Unlock()
	w.readBranchPR(here)
	w.readBranchPR(gone)

	if got := w.followedCheckouts(); !slices.Equal(got, []string{here}) {
		t.Fatalf("followed = %v, want %s alone: the other checkout is gone", got, here)
	}
}

func TestARecheckDuringAReadReadsAgainAfterIt(t *testing.T) {
	t.Parallel()
	forge := newForgePRs()
	forge.hold = make(chan struct{})
	w := newWatching(forge)
	done := make(chan struct{})
	go func() { w.readBranchPR("/wt/feat"); close(done) }()
	waitClaimed(t, w, "/wt/feat")

	if _, err := w.RecheckPullRequest(t.Context(), api.PathOptions{Path: "/wt/feat"}); err != nil {
		t.Fatalf("RecheckPullRequest: %v", err)
	}
	waitAgain(t, w, "/wt/feat")
	forge.mu.Lock()
	close(forge.hold)
	forge.hold = nil
	forge.mu.Unlock()
	<-done

	if got := forge.readsOf("/wt/feat"); got != 2 {
		t.Fatalf("reads = %d, want 2: the one running and the one asked meanwhile", got)
	}
}

func waitClaimed(t *testing.T, w *watching, path string) {
	t.Helper()
	waitPRs(t, w, func(b *branchPRs) bool { return b.reading[path] })
}

func waitAgain(t *testing.T, w *watching, path string) {
	t.Helper()
	waitPRs(t, w, func(b *branchPRs) bool { return b.again[path] })
}

func waitPRs(t *testing.T, w *watching, ok func(*branchPRs) bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		w.prs.mu.Lock()
		done := ok(&w.prs)
		w.prs.mu.Unlock()
		if done {
			return
		}
	}
	t.Fatal("the watch never reached the state")
}
