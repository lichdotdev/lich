package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

func TestPullsStoreReadsOnceUntilAChangeIsAnnounced(t *testing.T) {
	stub := &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		return []lichclient.PRSummary{{Number: 7}}, http.StatusOK
	}}
	s := newPullsStore(context.Background(), stub.client(t), func() {})
	s.list("/repo", pullsOpen)
	waitFor(t, "the first read", func() bool { return s.list("/repo", pullsOpen).loaded })
	s.list("/repo", pullsOpen)
	if got := stub.calls("project.ListPullRequests"); got != 1 {
		t.Fatalf("calls before a change = %d, want 1", got)
	}
	s.stale()
	if got := s.list("/repo", pullsOpen); len(got.val) != 1 {
		t.Errorf("a stale read keeps its rows while the next comes: %v", got.val)
	}
	waitFor(t, "the read the change owes", func() bool { return stub.calls("project.ListPullRequests") == 2 })
}

func TestPullsStoreKeepsGhsRefusalAsText(t *testing.T) {
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) { return failing("gh is not installed") }}
	s := newPullsStore(context.Background(), stub.client(t), func() {})
	s.list("/repo", pullsOpen)
	waitFor(t, "the refusal", func() bool { return s.list("/repo", pullsOpen).loaded })
	if got := s.list("/repo", pullsOpen).err; got != "gh is not installed" {
		t.Errorf("err = %q, want gh's sentence", got)
	}
}

func TestPullsStoreRereadsADetailWhileAChecksRuns(t *testing.T) {
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) {
		return lichclient.PRDetail{Number: 7, Checks: lichclient.ChecksRollup{Pending: 1, Total: 1}}, http.StatusOK
	}}
	s := newPullsStore(context.Background(), stub.client(t), func() {})
	s.detail("/repo", 7)
	waitFor(t, "the first read", func() bool { return s.detail("/repo", 7).loaded })
	s.mu.Lock()
	for _, e := range s.details {
		e.at = time.Now().Add(-pendingRecheck)
	}
	s.mu.Unlock()
	s.detail("/repo", 7)
	waitFor(t, "the recheck", func() bool { return stub.calls("project.PullRequestDetail") == 2 })
}

func TestPullsStoreRereadsADetailWhileGitHubWorksOutMergeability(t *testing.T) {
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) {
		return lichclient.PRDetail{Number: 7, State: "OPEN", Mergeable: "UNKNOWN", MergeStateStatus: "UNKNOWN"}, http.StatusOK
	}}
	s := newPullsStore(context.Background(), stub.client(t), func() {})
	s.detail("/repo", 7)
	waitFor(t, "the first read", func() bool { return s.detail("/repo", 7).loaded })
	s.mu.Lock()
	for _, e := range s.details {
		e.at = time.Now().Add(-pendingRecheck)
	}
	s.mu.Unlock()
	s.detail("/repo", 7)
	waitFor(t, "the recheck", func() bool { return stub.calls("project.PullRequestDetail") == 2 })
}

func pullsScreenOnAList(t *testing.T) (*pullsScreen, *rpcStub) {
	t.Helper()
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) {
		return []lichclient.PRSummary{{Number: 7}}, http.StatusOK
	}}
	p := newPullsScreen(newPullsStore(context.Background(), stub.client(t), func() {}), new(ui.Root))
	p.show(lichclient.Project{ID: "p", Path: "/repo"}, "main", 0)
	waitFor(t, "the first read", func() bool { return p.store.list("/repo", pullsOpen).loaded })
	return p, stub
}

func TestPullsScreenRereadsWhenTheWindowGetsFocusBack(t *testing.T) {
	p, stub := pullsScreenOnAList(t)
	p.setFocused(true)
	p.setFocused(true)
	p.store.list("/repo", pullsOpen)
	waitFor(t, "the read focus owes", func() bool { return stub.calls("project.ListPullRequests") == 2 })
	p.setFocused(false)
	p.store.list("/repo", pullsOpen)
	if got := stub.calls("project.ListPullRequests"); got != 2 {
		t.Errorf("calls after losing focus = %d, want 2", got)
	}
}

func TestPullsScreenRereadsWhatItShowsEveryRecheck(t *testing.T) {
	p, stub := pullsScreenOnAList(t)
	now := time.Now()
	if due := p.recheck(now); !due.After(now) {
		t.Fatalf("a fresh read is owed again at %v, want after %v", due, now)
	}
	p.store.list("/repo", pullsOpen)
	if got := stub.calls("project.ListPullRequests"); got != 1 {
		t.Fatalf("calls before the recheck = %d, want 1", got)
	}
	p.recheck(now.Add(prRecheck))
	p.store.list("/repo", pullsOpen)
	waitFor(t, "the recheck", func() bool { return stub.calls("project.ListPullRequests") == 2 })
}

func TestPullRequestCommentsGoOnlyToASessionInItsBranchCheckout(t *testing.T) {
	stub := &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		if method == "project.ListCheckouts" {
			return []lichclient.Worktree{{Name: "main", Path: "/repo"}, {Name: "feat/x", Path: "/wt/feat-x"}}, http.StatusOK
		}
		return []lichclient.PRSummary{}, http.StatusOK
	}}
	p := newPullsScreen(newPullsStore(context.Background(), stub.client(t), func() {}), new(ui.Root))
	p.show(lichclient.Project{ID: "p", Path: "/repo"}, "main", 0)
	p.sessionsIn = func(projectID, path string) []string {
		if projectID == "p" && path == "/wt/feat-x" {
			return []string{"in-branch", "second"}
		}
		return []string{"elsewhere"}
	}
	waitFor(t, "the checkouts", func() bool { return len(p.store.checkoutsOf("/repo")) == 2 })

	if id, target := p.reviewSession(lichclient.PRDetail{HeadRefName: "feat/x", URL: "https://pr/1"}); id != "in-branch" || target != "/wt/feat-x" {
		t.Errorf("checked-out branch: session %q, target %q; want in-branch, /wt/feat-x", id, target)
	}
	p.files.activeID = "second"
	if id, _ := p.reviewSession(lichclient.PRDetail{HeadRefName: "feat/x"}); id != "second" {
		t.Errorf("active session in the checkout: session %q, want second", id)
	}
	p.files.activeID = "elsewhere"
	if id, _ := p.reviewSession(lichclient.PRDetail{HeadRefName: "feat/x"}); id != "in-branch" {
		t.Errorf("active session in another checkout: session %q, want in-branch", id)
	}
	if id, target := p.reviewSession(lichclient.PRDetail{HeadRefName: "feat/y", URL: "https://pr/2"}); id != "" || target != "https://pr/2" {
		t.Errorf("branch with no checkout: session %q, target %q; want none, the PR's page", id, target)
	}
}
