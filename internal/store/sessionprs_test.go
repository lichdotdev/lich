package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func linkTestStore(t *testing.T) *Service {
	t.Helper()
	svc := newTestStore(t)
	if _, err := svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"}); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"keep", "s1"} {
		if _, err := svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: id, Kind: "claude", NextSeq: i + 2}); err != nil {
			t.Fatal(err)
		}
	}
	return svc
}

func mustLinks(t *testing.T, svc *Service) []api.SessionPullRequest {
	t.Helper()
	got, err := svc.SessionPullRequests()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

const linkURL = "https://github.com/o/r/pull/7"

// The latest word on a URL wins, and an unlink keeps the read a link made.
func TestSessionPullRequestLatestWordWins(t *testing.T) {
	svc := linkTestStore(t)
	pr := &api.PullRequest{Number: 7, URL: linkURL, Title: "t", State: "OPEN"}
	words := []api.SessionPullRequest{
		{Session: "s1", URL: linkURL, Linked: true, At: 1, Forge: api.ForgeGitHub, Here: true, PR: pr},
		{Session: "s1", URL: linkURL, At: 2},
	}
	for _, w := range words {
		if err := svc.SaveSessionPullRequest(w); err != nil {
			t.Fatal(err)
		}
	}
	got := mustLinks(t, svc)
	if len(got) != 1 || got[0].Linked || got[0].At != 2 || got[0].PR == nil || *got[0].PR != *pr {
		t.Fatalf("after unlink = %+v, want one unlinked row keeping the read", got)
	}
	if err := svc.SaveSessionPullRequest(api.SessionPullRequest{Session: "s1", URL: linkURL, Linked: true, At: 3, PR: pr}); err != nil {
		t.Fatal(err)
	}
	if got := mustLinks(t, svc); len(got) != 1 || !got[0].Linked {
		t.Fatalf("after linking again = %+v, want it linked", got)
	}
}

func TestSessionPullRequestsNewestFirst(t *testing.T) {
	svc := linkTestStore(t)
	for i, url := range []string{"a", "b", "c"} {
		if err := svc.SaveSessionPullRequest(api.SessionPullRequest{Session: "s1", URL: url, Linked: true, At: int64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	got := mustLinks(t, svc)
	if len(got) != 3 || got[0].URL != "c" || got[2].URL != "a" {
		t.Errorf("order = %+v, want c, b, a", got)
	}
}

// A resume re-keys the session; its links go with it, and a session deleted
// for good takes them along.
func TestSessionPullRequestsFollowTheSession(t *testing.T) {
	svc := linkTestStore(t)
	_ = svc.SetProviderSession("s1", "conv")
	if err := svc.SaveSessionPullRequest(api.SessionPullRequest{Session: "s1", URL: linkURL, Linked: true, At: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "s1", ActiveID: "keep"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReopenSession(t.Context(), api.ReopenSessionOptions{SessionID: "s1", NewSessionID: "fresh"}); err != nil {
		t.Fatal(err)
	}
	if got := mustLinks(t, svc); len(got) != 1 || got[0].Session != "fresh" {
		t.Fatalf("after resume = %+v, want the link on the fresh id", got)
	}
	if err := svc.DeleteSession("p1", "fresh", "keep"); err != nil {
		t.Fatal(err)
	}
	if got := mustLinks(t, svc); len(got) != 0 {
		t.Errorf("after delete = %+v, want none", got)
	}
}
