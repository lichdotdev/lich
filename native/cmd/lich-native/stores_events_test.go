package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// rpcStub answers /rpc calls with what answer says and counts them by method.
type rpcStub struct {
	mu     sync.Mutex
	counts map[string]int
	answer func(method string, args []json.RawMessage) (any, int)
}

func (s *rpcStub) calls(method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts[method]
}

func (s *rpcStub) client(t *testing.T) *lichclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var args []json.RawMessage
		_ = json.Unmarshal(body, &args)
		method := strings.TrimPrefix(r.URL.Path, "/rpc/")
		s.mu.Lock()
		if s.counts == nil {
			s.counts = map[string]int{}
		}
		s.counts[method]++
		s.mu.Unlock()
		out, status := s.answer(method, args)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
}

func failing(message string) (any, int) {
	return map[string]string{"error": message}, http.StatusInternalServerError
}

func TestSessionTouchedReadsGitNowAndNudgesTheDiff(t *testing.T) {
	var mu sync.Mutex
	head := "h1"
	stub := &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		mu.Lock()
		defer mu.Unlock()
		switch method {
		case "project.Diff":
			return lichclient.DiffStats{Branch: "main", Head: head}, http.StatusOK
		case "project.BaseStatus", "project.PullRequest":
			return nil, http.StatusOK
		}
		return failing("unexpected " + method)
	}}
	m := newTestModel(stub.client(t))
	var nudged []string
	m.diffTouched = func(checkout string) { nudged = append(nudged, checkout) }
	m.refreshGit(context.Background())
	if got := cardOf(t, m).git.Head; got != "h1" {
		t.Fatalf("head %q before the touch, want h1", got)
	}

	mu.Lock()
	head = "h2"
	mu.Unlock()
	send(m, "session-touched", map[string]string{"id": "s1"})

	eventually(t, "the card reads the new head without a tick", func() bool {
		return cardOf(t, m).git.Head == "h2"
	})
	if len(nudged) != 1 || nudged[0] != checkout {
		t.Errorf("diff nudged for %q, want the session's checkout", nudged)
	}
}

func TestSessionTouchedOfAnotherProjectWaitsForItsScreen(t *testing.T) {
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) { return failing("unexpected") }}
	m := newModel(stub.client(t), func() {})
	m.projects = twoProjects()
	m.activeProject, m.active = "a", "a2"
	nudged := false
	m.diffTouched = func(string) { nudged = true }

	send(m, "session-touched", map[string]string{"id": "b1"})
	send(m, "session-touched", map[string]string{"id": "nobody"})

	time.Sleep(50 * time.Millisecond)
	if nudged || stub.calls("project.Diff") != 0 {
		t.Errorf("touched sessions of a hidden project read git (nudged %v, %d reads)", nudged, stub.calls("project.Diff"))
	}
}

func TestSessionFocusBringsItsCardUp(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = twoProjects()
	m.activeProject, m.active = "a", "a2"
	type focus struct{ project, id string }
	var got []focus
	m.focusSession = func(projectID, id string) { got = append(got, focus{projectID, id}) }

	send(m, "session-focus", map[string]string{"id": "b1"})
	send(m, "session-focus", map[string]string{"id": "gone"})

	if len(got) != 1 || got[0] != (focus{"b", "b1"}) {
		t.Errorf("focused %+v, want b1 of project b only", got)
	}
}

func TestReloadAdoptsTheBackendsActiveCardWhenTheOnScreenOneWasClosed(t *testing.T) {
	ps := twoProjects()
	m := newModel(fakeState(t, &ps, "/projects/a"), func() {})
	if err := m.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, s := onScreen(m); s != "a2" {
		t.Fatalf("on %q, want a2", s)
	}

	// `lich close` on the card on screen: the backend picked a1 meanwhile.
	ps[0].Sessions, ps[0].ActiveSessionID = ps[0].Sessions[:1], "a1"
	if err := m.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, s := onScreen(m); s != "a1" {
		t.Errorf("on %q after its card was closed, want the backend's a1", s)
	}
}

func TestShownPathsHoldsAnEmptyProjectsOwn(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = []lichclient.Project{{ID: "p", Path: checkout}}
	m.activeProject = "p"
	if paths := m.shownPaths(); len(paths) != 1 || paths[0] != checkout {
		t.Errorf("polls %q, want the project's own path for the footer", paths)
	}
}

func TestGitStoreDropsWhatNoOpenProjectCanShow(t *testing.T) {
	m := newTestModel(fakeDiff(t, ""))
	m.projects[0].Sessions[0].Path = "/wt/feat"
	m.refreshGit(context.Background())
	if _, ok := m.gitStatus("/wt/feat"); !ok {
		t.Fatal("no reading of the worktree card")
	}

	send(m, "session-cwd", sessionCwd{ID: "s1", Cwd: elsewhere})
	m.refreshGit(context.Background())
	if _, ok := m.gitStatus("/wt/feat"); ok {
		t.Error("the worktree the shell left is still held")
	}
	if _, ok := m.gitStatus(elsewhere); !ok {
		t.Error("the directory the shell is in has no reading")
	}
	m.mu.Lock()
	held := len(m.git) + len(m.base) + len(m.prs)
	m.mu.Unlock()
	if held != 6 {
		t.Errorf("%d entries held across git, base and PR stores, want the shell's directory and the project's, three each", held)
	}
}

func TestGitStoreKeepsTheLastReadingThroughATransientFailure(t *testing.T) {
	var mu sync.Mutex
	broken, exists := false, true
	stub := &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		mu.Lock()
		defer mu.Unlock()
		switch method {
		case "project.Diff":
			if broken {
				return failing("index.lock exists")
			}
			return lichclient.DiffStats{Branch: "main", Head: "h"}, http.StatusOK
		case "project.Exists":
			return exists, http.StatusOK
		case "project.BaseStatus", "project.PullRequest":
			return nil, http.StatusOK
		}
		return failing("unexpected " + method)
	}}
	m := newTestModel(stub.client(t))
	m.refreshGit(context.Background())

	mu.Lock()
	broken = true
	mu.Unlock()
	m.refreshGit(context.Background())
	if g, ok := m.gitStatus(checkout); !ok || g.Branch != "main" {
		t.Fatalf("after a transient failure: %+v %v, want the last reading kept", g, ok)
	}

	mu.Lock()
	exists = false
	mu.Unlock()
	m.refreshGit(context.Background())
	if _, ok := m.gitStatus(checkout); ok {
		t.Error("a path that is gone kept its reading")
	}
}

func TestAnOpenPullRequestIsAskedAgainOnASlowCadence(t *testing.T) {
	forge := &fakeForge{prs: map[string]*lichclient.PullRequest{"c1": {Number: 7}}}
	forge.set("feat", "c1")
	m := newTestModel(forge.client(t))
	prOnCard(t, m, 7)

	forge.mu.Lock()
	forge.prs["c1"] = nil // merged on GitHub: branch and head did not move
	forge.mu.Unlock()
	m.refreshGit(context.Background())
	time.Sleep(50 * time.Millisecond)
	if pr := cardOf(t, m).pr; pr == nil || pr.Number != 7 {
		t.Fatalf("PR %+v before the cadence elapsed, want #7 kept", pr)
	}

	m.mu.Lock()
	l := m.prs[checkout]
	l.asked = l.asked.Add(-2 * prRecheck)
	m.prs[checkout] = l
	m.mu.Unlock()
	prOnCard(t, m, 0)
}

func TestAPathWithNoPullRequestIsNotAskedAgainByTheCadence(t *testing.T) {
	stub := &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		switch method {
		case "project.Diff":
			return lichclient.DiffStats{Branch: "main", Head: "h"}, http.StatusOK
		case "project.BaseStatus", "project.PullRequest":
			return nil, http.StatusOK
		}
		return failing("unexpected " + method)
	}}
	m := newTestModel(stub.client(t))
	m.refreshGit(context.Background())
	eventually(t, "the first lookup", func() bool { return stub.calls("project.PullRequest") == 1 })

	m.mu.Lock()
	l := m.prs[checkout]
	l.asked = l.asked.Add(-2 * prRecheck)
	m.prs[checkout] = l
	m.mu.Unlock()
	m.refreshGit(context.Background())
	time.Sleep(50 * time.Millisecond)
	if n := stub.calls("project.PullRequest"); n != 1 {
		t.Errorf("gh asked %d times for a branch with no PR, want 1", n)
	}
}

func TestALoginEndingReloadsTheProfilesBeforeTheScreenHearsIt(t *testing.T) {
	var mu sync.Mutex
	loggedIn := false
	stub := &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		mu.Lock()
		defer mu.Unlock()
		if method != "claudeprofile.Profiles" {
			return failing("unexpected " + method)
		}
		return []lichclient.ClaudeProfile{{Slug: personalProfile, Name: "Personal", LoggedIn: true},
			{Slug: "work", Name: "Work", LoggedIn: loggedIn}}, http.StatusOK
	}}
	m := newModel(stub.client(t), func() {})
	if err := m.profiles.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.profiles.plans["work"] = profileReading{at: time.Now()}

	mu.Lock()
	loggedIn = true
	mu.Unlock()
	heard := make(chan []string, 1)
	m.loginProgress = func(lichclient.ClaudeProfileLogin) {
		var slugs []string
		for _, p := range m.profiles.loggedIn() {
			slugs = append(slugs, p.Slug)
		}
		heard <- slugs
	}
	send(m, lichclient.ClaudeProfileLoginEvent, lichclient.ClaudeProfileLogin{Profile: "work", Done: true, LoggedIn: true})

	select {
	case slugs := <-heard:
		if len(slugs) != 2 {
			t.Errorf("screen heard the login end with %v logged in, want the new login already read", slugs)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the screen never heard the login end")
	}
	if _, ok := m.profiles.cachedPlan("work"); ok {
		t.Error("a plan reading survived a login that changed whose plan it is")
	}
}

func TestAWindowFocusReadsTheProfilesAgainOnlyOnceTheyAreOld(t *testing.T) {
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) {
		return []lichclient.ClaudeProfile{{Slug: personalProfile, Name: "Personal", LoggedIn: true}}, http.StatusOK
	}}
	m := newModel(stub.client(t), func() {})
	ctx := context.Background()
	m.setFocused(ctx, true)
	time.Sleep(30 * time.Millisecond)
	if n := stub.calls("claudeprofile.Profiles"); n != 0 {
		t.Fatalf("%d reads before the first one, want focus to leave that to the start", n)
	}
	if err := m.profiles.reload(ctx); err != nil {
		t.Fatal(err)
	}
	m.setFocused(ctx, true)
	time.Sleep(30 * time.Millisecond)
	if n := stub.calls("claudeprofile.Profiles"); n != 1 {
		t.Fatalf("%d reads right after one, want no new one", n)
	}

	m.profiles.mu.Lock()
	m.profiles.loadedAt = time.Now().Add(-2 * profileListMaxAge)
	m.profiles.mu.Unlock()
	m.setFocused(ctx, true)
	eventually(t, "the stale list is read again", func() bool { return stub.calls("claudeprofile.Profiles") == 2 })
}

func TestRenameFailureIsShownAndNothingElseChanges(t *testing.T) {
	stub := &rpcStub{answer: func(method string, args []json.RawMessage) (any, int) {
		if method != "store.RenameSession" {
			return failing("unexpected " + method)
		}
		return failing("database is locked")
	}}
	var shown []ui.Toast
	renameSession(context.Background(), stub.client(t), func(t ui.Toast) string {
		shown = append(shown, t)
		return ""
	}, "s1", "build")

	if len(shown) != 1 || shown[0].Variant != ui.ToastError || !strings.Contains(shown[0].Title, "database is locked") {
		t.Errorf("toasts %+v, want the backend's sentence as an error", shown)
	}
}
