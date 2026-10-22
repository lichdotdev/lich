package project

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestListPullRequestsTalliesRequestedThreads(t *testing.T) {
	t.Parallel()
	list := []byte(`[
		{"number":39,"title":"a","reviewDecision":"CHANGES_REQUESTED"},
		{"number":41,"title":"b","reviewDecision":"CHANGES_REQUESTED"},
		{"number":37,"title":"c","reviewDecision":"APPROVED"}
	]`)
	threads := []byte(`{"data":{"repository":{
		"pr39":{"number":39,"reviewThreads":{"nodes":[{"isResolved":true},{"isResolved":true}]}},
		"pr41":{"number":41,"reviewThreads":{"nodes":[{"isResolved":false},{"isResolved":true},{"isResolved":false}]}}
	}}}`)

	t.Run("each row with changes requested carries its threads", func(t *testing.T) {
		gh := &fakeGH{outs: [][]byte{list, threads}}
		got, err := withGH(gh).ListPullRequests("/repo", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []ThreadTally{{Open: 0, Total: 2}, {Open: 2, Total: 3}, {}}
		for i, w := range want {
			if got[i].Threads != w {
				t.Errorf("#%d threads = %+v, want %+v", got[i].Number, got[i].Threads, w)
			}
		}
		query := gh.history[1][len(gh.history[1])-1]
		if !strings.Contains(query, "pullRequest(number:39)") || !strings.Contains(query, "pullRequest(number:41)") || strings.Contains(query, "number:37") {
			t.Errorf("query asks for the wrong pull requests: %s", query)
		}
		if gh.dir != "/repo" || gh.timeout != prReadTimeout {
			t.Errorf("wrong scope: dir %q timeout %v", gh.dir, gh.timeout)
		}
	})

	t.Run("no changes requested asks nothing more", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`[{"number":37,"title":"c","reviewDecision":"APPROVED"}]`)}
		if _, err := withGH(gh).ListPullRequests("/repo", ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gh.calls != 1 {
			t.Errorf("gh was called %d times, want 1", gh.calls)
		}
	})

	t.Run("a failed thread query fails the list", func(t *testing.T) {
		gh := &fakeGH{outs: [][]byte{list, nil}, errs: []error{nil, errors.New("graphql: rate limited")}}
		if _, err := withGH(gh).ListPullRequests("/repo", ""); err == nil || err.Error() != "graphql: rate limited" {
			t.Errorf("error = %v, want the runner's message", err)
		}
	})

	t.Run("an unreadable answer is an error", func(t *testing.T) {
		gh := &fakeGH{outs: [][]byte{list, []byte(`{not json`)}}
		if _, err := withGH(gh).ListPullRequests("/repo", ""); err == nil {
			t.Error("expected a decode error")
		}
	})

	t.Run("the query is read-only graphql in the checkout's repository", func(t *testing.T) {
		gh := &fakeGH{outs: [][]byte{list, threads}}
		if _, err := withGH(gh).ListPullRequests("/repo", ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		head := gh.history[1][:6]
		want := []string{"api", "graphql", "-F", "owner={owner}", "-F", "repo={repo}"}
		if !slices.Equal(head, want) {
			t.Errorf("args = %v, want %v", head, want)
		}
	})
}
