package relay

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// fakePRs answers the merge watch's reads with what each checkout's branch has
// at the moment, and counts the reads.
type fakePRs struct {
	mu    sync.Mutex
	prs   map[string]*api.PRDetail
	err   error
	reads map[string]int
}

func (f *fakePRs) set(checkout string, pr *api.PRDetail) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prs[checkout] = pr
}

func (f *fakePRs) read(path string) (*api.PRDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads[path]++
	return f.prs[path], f.err
}

func newFakePRs() *fakePRs {
	return &fakePRs{prs: map[string]*api.PRDetail{}, reads: map[string]int{}}
}

func featPR(state string) *api.PRDetail {
	return &api.PRDetail{Number: 42, Title: "Add merge notice", State: state, HeadRefName: "feat/merge", BaseRefName: "main"}
}

func TestMergeWatchAnnouncesAMergeMadeOutsideLich(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("agent")
	svc := newRelay(merging(), term, nil)
	prs := newFakePRs()
	prs.set("/wt/feat", featPR(prOpen))

	svc.watchMerges(prs.read)
	prs.set("/wt/feat", featPR(prMerged))
	svc.watchMerges(prs.read)

	if !awaitWritten(term, "agent", "#42") {
		t.Fatalf("typed at agent = %q, want the merge notice", term.written("agent"))
	}
	svc.watchMerges(prs.read)
	if got := strings.Count(term.written("agent"), "#42"); got != 1 {
		t.Fatalf("agent heard the notice %d times, want once", got)
	}
}

func TestMergeWatchReadsOnlyCheckoutsAnAgentRunsIn(t *testing.T) {
	t.Parallel()
	// "shell" and "parked" are in /wt/feat too; only "other" makes /wt/fix live.
	term := newFakeTerminal("shell", "other")
	svc := newRelay(merging(), term, nil)
	prs := newFakePRs()

	svc.watchMerges(prs.read)

	if prs.reads["/wt/fix"] != 1 || prs.reads["/wt/feat"] != 0 || prs.reads["/src/lich"] != 0 {
		t.Fatalf("reads = %v, want /wt/fix alone", prs.reads)
	}
}

func TestMergeWatchLeavesAPullRequestFirstSeenMerged(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("agent")
	svc := newRelay(merging(), term, nil)
	prs := newFakePRs()
	prs.set("/wt/feat", featPR(prMerged))

	svc.watchMerges(prs.read)

	if awaitWritten(term, "agent", "#42") {
		t.Fatalf("typed at agent = %q, want nothing: it merged before the watch saw it open", term.written("agent"))
	}
}

func TestMergeWatchLeavesAClosedPullRequest(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("agent")
	svc := newRelay(merging(), term, nil)
	prs := newFakePRs()
	prs.set("/wt/feat", featPR(prOpen))
	svc.watchMerges(prs.read)
	prs.set("/wt/feat", featPR("CLOSED"))
	svc.watchMerges(prs.read)
	prs.set("/wt/feat", featPR(prMerged))
	svc.watchMerges(prs.read)

	if awaitWritten(term, "agent", "#42") {
		t.Fatalf("typed at agent = %q, want nothing for a closed PR", term.written("agent"))
	}
}

func TestMergeWatchKeepsWhatItSawThroughAFailedRead(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("agent")
	svc := newRelay(merging(), term, nil)
	prs := newFakePRs()
	prs.set("/wt/feat", featPR(prOpen))
	svc.watchMerges(prs.read)
	prs.err = errors.New("gh is not signed in to GitHub.")
	svc.watchMerges(prs.read)
	prs.err = nil
	prs.set("/wt/feat", featPR(prMerged))
	svc.watchMerges(prs.read)

	if !awaitWritten(term, "agent", "#42") {
		t.Fatalf("typed at agent = %q, want the merge notice", term.written("agent"))
	}
}

func TestMergeWatchAndThePullsMergeAnnounceOnce(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("agent")
	svc := newRelay(merging(), term, nil)
	prs := newFakePRs()
	prs.set("/wt/feat", featPR(prOpen))
	svc.watchMerges(prs.read)

	if _, err := svc.AnnounceMerge(t.Context(), api.AnnounceMergeOptions{Checkout: "/wt/feat", Number: 42, Title: "Add merge notice", Branch: "feat/merge", Base: "main"}); err != nil {
		t.Fatalf("AnnounceMerge: %v", err)
	}
	prs.set("/wt/feat", featPR(prMerged))
	svc.watchMerges(prs.read)

	if !awaitWritten(term, "agent", "#42") {
		t.Fatalf("typed at agent = %q, want the merge notice", term.written("agent"))
	}
	if got := strings.Count(term.written("agent"), "#42"); got != 1 {
		t.Fatalf("agent heard the notice %d times, want once", got)
	}
}
