package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

func TestStandingOf(t *testing.T) {
	cases := []struct {
		in   *backend.BaseStatus
		want baseStanding
	}{
		{nil, baseStanding{}},
		{&backend.BaseStatus{Behind: 0}, baseStanding{}},
		{&backend.BaseStatus{Behind: 3}, baseStanding{count: 3}},
		{&backend.BaseStatus{Behind: 3, Conflicts: []string{"a", "b"}}, baseStanding{conflict: true, count: 2}},
	}
	for _, c := range cases {
		if got := standingOf(c.in); got != c.want {
			t.Errorf("standingOf(%+v) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// fakeForge serves a checkout whose branch and head the test moves, and plays
// the backend's pull request watch: a recheck is answered with a
// BranchPullRequestEvent carrying the PR at the head when the read lands, so
// an answer never depends on when the recheck was asked.
type fakeForge struct {
	mu           sync.Mutex
	branch, head string
	prs          map[string]*backend.PullRequest // head -> PR
	hold         chan struct{}                   // while set, reads do not land
	answered     int                             // reads landed
	m            *model
}

func (f *fakeForge) set(branch, head string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.branch, f.head = branch, head
}

// watch is the model the reads land at.
func (f *fakeForge) watch(m *model) *model {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m = m
	return m
}

// read lands a recheck of path once hold lets it.
func (f *fakeForge) read(path string, hold chan struct{}) {
	if hold != nil {
		<-hold
	}
	f.mu.Lock()
	pr, m := f.prs[f.head], f.m
	f.answered++
	f.mu.Unlock()
	send(m, api.BranchPullRequestEvent.Name, api.BranchPullRequest{Path: path, PR: pr})
}

func (f *fakeForge) client(t *testing.T) *backend.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/rpc/project.Diff":
			_ = json.NewEncoder(w).Encode(backend.DiffStats{Branch: f.branch, Head: f.head})
		case "/rpc/project.BaseStatus":
			_ = json.NewEncoder(w).Encode(backend.BaseStatus{Behind: 2})
		case "/rpc/project.RecheckPullRequest":
			var args []api.PathOptions
			_ = json.NewDecoder(r.Body).Decode(&args)
			go f.read(args[0].Path, f.hold)
			_ = json.NewEncoder(w).Encode(nil)
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return backend.New(backend.Runtime{Port: port, Token: "tok"})
}

// prOnCard refreshes git and waits for the card's PR badge to read want.
func prOnCard(t *testing.T, m *model, want int) {
	t.Helper()
	m.refreshGit(context.Background())
	awaitPROnCard(t, m, want)
}

// awaitPROnCard waits for the card's PR badge to read want, with no git tick.
func awaitPROnCard(t *testing.T, m *model, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := 0
		if pr := cardOf(t, m).pr; pr != nil {
			got = pr.Number
		}
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("PR on card %d, want %d", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCardFollowsThePullRequestOfItsBranch(t *testing.T) {
	forge := &fakeForge{prs: map[string]*backend.PullRequest{
		"c2": {Number: 7, State: "OPEN"},
		"c4": {Number: 9, State: "OPEN"},
	}}
	forge.set("feat", "c1")
	m := forge.watch(newTestModel(forge.client(t)))
	prOnCard(t, m, 0)
	if b := standingOf(cardOf(t, m).base); b != (baseStanding{count: 2}) {
		t.Fatalf("base %+v, want 2 behind", b)
	}

	forge.set("feat", "c2") // the session opened its PR
	prOnCard(t, m, 7)

	forge.mu.Lock()
	forge.hold = make(chan struct{})
	forge.mu.Unlock()
	forge.set("feat", "c3") // a new commit: the badge stays while the PR is read
	m.refreshGit(context.Background())
	if pr := cardOf(t, m).pr; pr == nil || pr.Number != 7 {
		t.Fatalf("new commit on the branch: PR %+v before the read lands, want #7 kept", pr)
	}
	forge.mu.Lock()
	close(forge.hold)
	forge.hold = nil
	forge.mu.Unlock()
	prOnCard(t, m, 0) // c3 has no open PR: merged

	forge.prs["c3"] = &backend.PullRequest{Number: 8, State: "OPEN"}
	m.mu.Lock()
	m.stalePullRequests() // the window came back: opened in the browser
	m.mu.Unlock()
	prOnCard(t, m, 8)

	forge.mu.Lock()
	forge.hold = make(chan struct{})
	forge.mu.Unlock()
	forge.set("other", "c4")
	m.refreshGit(context.Background())
	if pr := cardOf(t, m).pr; pr != nil {
		t.Fatalf("new branch: PR #%d before the read lands, want the old branch's dropped", pr.Number)
	}
	forge.mu.Lock()
	close(forge.hold)
	forge.hold = nil
	forge.mu.Unlock()
	prOnCard(t, m, 9)
}

// gh pr create moves neither branch nor head, so the turn that ran it ending is
// what makes the card ask again.
func TestCardFindsThePullRequestItsTurnOpened(t *testing.T) {
	forge := &fakeForge{prs: map[string]*backend.PullRequest{}}
	forge.set("feat", "c1")
	m := forge.watch(newTestModel(forge.client(t)))
	prOnCard(t, m, 0)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		forge.mu.Lock()
		answered := forge.answered
		forge.mu.Unlock()
		if answered > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the PR at c1 was never read")
		}
	}

	forge.mu.Lock()
	forge.prs["c1"] = &backend.PullRequest{Number: 5, State: "OPEN"}
	forge.mu.Unlock()
	send(m, api.SessionStatusEvent.Name, statusEvent{ID: "s1", State: statusBusy})
	send(m, api.SessionStatusEvent.Name, statusEvent{ID: "s1", State: statusDone})
	awaitPROnCard(t, m, 5)
}
