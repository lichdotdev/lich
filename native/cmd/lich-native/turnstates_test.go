package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/backend"

	"github.com/lichdotdev/lich/api"
)

func turnState(id, state string) backend.TurnState {
	return backend.TurnState{ID: id, State: state, Since: t0.UnixMilli()}
}

func TestHydrationSeedsWhatTheWindowMissed(t *testing.T) {
	s := newStatusStore()
	busy := turnState("s1", "busy")
	busy.Tool, busy.Detail = "Bash", "go test ./..."
	waiting := turnState("s2", "waiting")
	waiting.Reason = "permission"

	s.hydrate([]backend.TurnState{busy, waiting, turnState("s3", "idle"), turnState("s4", "interrupted")})

	if got := s.status("s1"); got != statusBusy {
		t.Errorf("s1 = %q, want busy", got)
	}
	if tool, ok := s.tool("s1"); !ok || tool != (sessionTool{Name: "Bash", Detail: "go test ./..."}) {
		t.Errorf("s1 tool = %+v, %v", tool, ok)
	}
	if got, want := s.status("s2"), statusWaiting; got != want || s.reason("s2") != "permission" {
		t.Errorf("s2 = %q / %q, want waiting on permission", got, s.reason("s2"))
	}
	for _, id := range []string{"s3", "s4"} {
		if got := s.status(id); got != statusNone {
			t.Errorf("%s = %q, want no status", id, got)
		}
		if !s.reported(id) {
			t.Errorf("%s: a report that maps to no status still proves the provider reports", id)
		}
	}
	if age, ok := s.age("s1", t0.Add(90*time.Second)); !ok || age != 90*time.Second {
		t.Errorf("age = %v, %v, want 90s since the transition the backend stamped", age, ok)
	}
}

// A finished turn's unread mark is the database's (restoreUnread): the snapshot
// holds the last state of a session, not whether anyone read it.
func TestHydrationLeavesFinishedTurnsToTheDatabase(t *testing.T) {
	s := newStatusStore()
	s.restoreUnread([]string{"unread"})

	s.hydrate([]backend.TurnState{turnState("unread", "done"), turnState("read", "done")})

	if !s.unread("unread") {
		t.Error("the unread turn lost its mark")
	}
	if s.status("read") != statusNone || s.unread("read") {
		t.Errorf("a turn the database holds as read came back as %q", s.status("read"))
	}
}

// What the snapshot knows is newer than the unread mark from the database: a
// session that started another turn while the window was away is busy.
func TestHydrationOutranksTheRestoredUnreadMark(t *testing.T) {
	s := newStatusStore()
	s.restoreUnread([]string{"s1"})

	s.hydrate([]backend.TurnState{turnState("s1", "busy")})

	if got := s.status("s1"); got != statusBusy {
		t.Errorf("status = %q, want busy", got)
	}
}

// The snapshot is taken at some point after /events attached and may be applied
// after an event the stream delivered meanwhile; the event is the newer.
func TestHydrationNeverOverwritesAnEventTheStreamDelivered(t *testing.T) {
	s := newStatusStore()
	replay(t, s, []step{{0, statusEvent{ID: "s1", State: "busy", Tool: "Edit"}}, {1, statusEvent{ID: "s1", State: "done"}}})

	s.hydrate([]backend.TurnState{{ID: "s1", State: "busy", Tool: "Bash", Since: t0.UnixMilli()}})

	if got := s.status("s1"); got != statusDone {
		t.Errorf("status = %q, want done: the snapshot was older than the event", got)
	}
	if _, ok := s.tool("s1"); ok {
		t.Error("the snapshot's tool came back over the event that ended the turn")
	}
}

func TestHydrationRunsOnceBeforeTheEventsItIsOlderThan(t *testing.T) {
	s := newStatusStore()
	s.hydrate([]backend.TurnState{turnState("s1", "busy")})

	replay(t, s, []step{{5, statusEvent{ID: "s1", State: "done"}}})

	if got := s.status("s1"); got != statusDone {
		t.Errorf("status = %q, want done: an event after the snapshot wins", got)
	}
}

func TestUsageHydrationSeedsAndYieldsToTheStream(t *testing.T) {
	seeded := json.RawMessage(`{"id":"s1","percent":10,"tokens":1,"window":200000,"model":"m","effort":""}`)
	f := newFooterUsage()
	if err := f.seed(seeded); err != nil {
		t.Fatal(err)
	}
	if u, ok := f.get("s1"); !ok || u.Percent != 10 {
		t.Fatalf("seeded = %+v, %v", u, ok)
	}

	f = newFooterUsage()
	_, _ = f.apply(footerEvent("session-usage", opusReport))
	if err := f.seed(seeded); err != nil {
		t.Fatal(err)
	}
	if u, _ := f.get("s1"); u.Percent != 42 {
		t.Errorf("percent = %d, want the streamed 42", u.Percent)
	}

	f = newFooterUsage()
	_, _ = f.apply(footerEvent("session-usage", opusReport))
	_, _ = f.apply(footerEvent("session-status", `{"id":"s1","state":"idle"}`))
	if err := f.seed(seeded); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.get("s1"); ok {
		t.Error("a snapshot revived the report of a provider that left")
	}
}

func fakeTurnStates(t *testing.T, states []backend.TurnState) *backend.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc/terminal.TurnStates" {
			t.Errorf("unexpected call %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(states)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return backend.New(backend.Runtime{Port: port, Token: "tok"})
}

func TestModelHydratesStatusAndUsageThenFollowsNewerEvents(t *testing.T) {
	var usage api.SessionUsage
	if err := json.Unmarshal([]byte(opusReport), &usage); err != nil {
		t.Fatal(err)
	}
	busy := backend.TurnState{ID: "s1", State: "busy", Tool: "Bash", Since: time.Now().UnixMilli(),
		Usage: &usage, Cwd: "/elsewhere"}
	m := newTestModel(fakeTurnStates(t, []backend.TurnState{busy}))
	toasts := 0
	m.notify = func(statusReport) { toasts++ }

	if err := m.hydrateTurnStates(context.Background()); err != nil {
		t.Fatal(err)
	}

	m.mu.Lock()
	status, cwd := m.status.status("s1"), m.cwd["s1"]
	m.mu.Unlock()
	if cwd.Cwd != "/elsewhere" {
		t.Errorf("cwd = %+v, want the /elsewhere the session last moved to", cwd)
	}
	if status != statusBusy {
		t.Errorf("status = %q, want busy", status)
	}
	if u, ok := m.usage.get("s1"); !ok || u.Percent != 42 {
		t.Errorf("usage = %+v, %v", u, ok)
	}
	if toasts != 0 {
		t.Errorf("hydration notified %d times: nothing happened, the window only learned of it", toasts)
	}

	send(m, statusEventName, statusEvent{ID: "s1", State: "done"})
	m.mu.Lock()
	status = m.status.status("s1")
	m.mu.Unlock()
	if status != statusDone {
		t.Errorf("status after a newer event = %q, want done", status)
	}
}

func statusOf(m *model, id string) (status string, hasTool bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, hasTool = m.status.tool(id)
	return m.status.status(id), hasTool
}

// A process that dies mid-turn never reports the turn's end, and the backend
// publishes only the exit.
func TestExitEndsTheTurnTheProcessDiedIn(t *testing.T) {
	m := newTestModel(nil)
	send(m, statusEventName, statusEvent{ID: "s1", State: "busy", Tool: "Bash"})
	if status, _ := statusOf(m, "s1"); status != statusBusy {
		t.Fatalf("status = %q, want busy before the exit", status)
	}

	send(m, "terminal:exit:s1", map[string]any{"code": 1})

	status, hasTool := statusOf(m, "s1")
	if status != statusNone || hasTool {
		t.Errorf("after exit: status %q, tool %v; want no status and no tool", status, hasTool)
	}
}

func TestExitKeepsAFinishedTurnUnreadAndOtherSessionsBusy(t *testing.T) {
	m := newTestModel(nil)
	m.projects[0].Sessions = append(m.projects[0].Sessions, backend.Session{ID: "s2"})
	send(m, statusEventName, statusEvent{ID: "s1", State: "done"})
	send(m, statusEventName, statusEvent{ID: "s2", State: "busy"})

	send(m, "terminal:exit:s1", map[string]any{})

	if status, _ := statusOf(m, "s1"); status != statusDone {
		t.Errorf("s1 = %q, want its finished turn kept", status)
	}
	if status, _ := statusOf(m, "s2"); status != statusBusy {
		t.Errorf("s2 = %q, want busy: another session's exit is no news", status)
	}
}
