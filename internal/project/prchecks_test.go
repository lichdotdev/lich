package project

import (
	"errors"
	"maps"
	"slices"
	"testing"
)

// TestIsFailureConclusion pins every conclusion gh reports, both halves of the
// list. It is an allow-list of failures, so a literal dropped from it does not
// break a build — it turns a broken check green on the panel, and a merge
// affordance opens on a red pull request.
func TestIsFailureConclusion(t *testing.T) {
	t.Parallel()
	tests := map[string]bool{
		"FAILURE":         true,
		"TIMED_OUT":       true,
		"CANCELLED":       true,
		"ACTION_REQUIRED": true,
		"STARTUP_FAILURE": true,
		"STALE":           true,
		"SUCCESS":         false,
		"NEUTRAL":         false,
		"SKIPPED":         false,
		// gh shouts its conclusions; a lowercase one is not one of them, and
		// matching it would be matching something gh never sends.
		"failure": false,
		"":        false,
	}
	for conclusion, want := range tests {
		t.Run(conclusion, func(t *testing.T) {
			if got := isFailureConclusion(conclusion); got != want {
				t.Errorf("isFailureConclusion(%q) = %v, want %v", conclusion, got, want)
			}
		})
	}
}

// TestPullRequestDetailMarksRequiredChecks pins Required to GitHub's isRequired,
// matched per run: "test" runs in two workflows and only the URL tells them
// apart.
func TestPullRequestDetailMarksRequiredChecks(t *testing.T) {
	t.Parallel()
	view := []byte(`{"number":818,"state":"OPEN","statusCheckRollup":[
		{"__typename":"CheckRun","name":"test","status":"COMPLETED","conclusion":"SUCCESS","workflowName":"ci","detailsUrl":"https://x/ci"},
		{"__typename":"CheckRun","name":"test","status":"COMPLETED","conclusion":"SUCCESS","workflowName":"native","detailsUrl":"https://x/native"},
		{"__typename":"StatusContext","context":"deploy","state":"SUCCESS","targetUrl":"https://x/deploy"}
	]}`)
	required := []byte(`{"data":{"repository":{"pullRequest":{"commits":{"nodes":[{"commit":{"statusCheckRollup":{"contexts":{"nodes":[
		{"name":"test","detailsUrl":"https://x/ci","isRequired":true},
		{"name":"test","detailsUrl":"https://x/native","isRequired":false},
		{"name":"deploy","detailsUrl":"https://x/deploy","isRequired":true}
	]}}}}]}}}}}`)
	gh := &fakeGH{outs: [][]byte{view, required}}
	pr, err := withGH(gh).PullRequestDetail("/repo", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := map[string]bool{}
	for _, c := range pr.CheckRuns {
		got[c.URL] = c.Required
	}
	want := map[string]bool{"https://x/ci": true, "https://x/native": false, "https://x/deploy": true}
	if !maps.Equal(got, want) {
		t.Errorf("required = %v, want %v", got, want)
	}
	if !slices.Contains(gh.history[1], "number=818") {
		t.Errorf("lookup args = %v, want the PR number gh resolved", gh.history[1])
	}
}

func TestPullRequestDetailRequiredLookupFailure(t *testing.T) {
	t.Parallel()
	view := []byte(`{"number":7,"state":"OPEN","statusCheckRollup":[{"name":"test","status":"IN_PROGRESS"}]}`)
	gh := &fakeGH{outs: [][]byte{view, nil}, errs: []error{nil, errors.New("gh is not signed in to GitHub.")}}
	if _, err := withGH(gh).PullRequestDetail("/repo", 0); err == nil {
		t.Fatal("expected the lookup's error")
	}
}

func TestPullRequestDetailWithoutChecksSkipsTheLookup(t *testing.T) {
	t.Parallel()
	gh := &fakeGH{out: []byte(`{"number":7,"state":"OPEN"}`)}
	if _, err := withGH(gh).PullRequestDetail("/repo", 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gh.calls != 1 {
		t.Errorf("gh calls = %d, want 1", gh.calls)
	}
}
