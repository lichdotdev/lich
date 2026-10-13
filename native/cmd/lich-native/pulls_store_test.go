package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
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
