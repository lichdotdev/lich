package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

func threeProjects() []backend.Project {
	return []backend.Project{
		{ID: "a", Name: "alpha", ActiveSessionID: "a1", Sessions: []backend.Session{{ID: "a1", Label: "A one"}, {ID: "a2", Label: "A two"}}},
		{ID: "b", Name: "beta", ActiveSessionID: "b1", Sessions: []backend.Session{{ID: "b1", Label: "B one"}}},
		{ID: "c", Name: "gamma", ActiveSessionID: "c1", Sessions: []backend.Session{{ID: "c1", Label: "C one"}}},
	}
}

func modelOn(active string) *model {
	m := newModel(nil, func() {})
	m.projects = threeProjects()
	m.setActiveProject(active)
	return m
}

func report(t *testing.T, m *model, id, state string) {
	t.Helper()
	data, _ := json.Marshal(statusEvent{ID: id, State: state})
	if _, err := m.status.apply(backend.Event{Name: api.SessionStatusEvent.Name, Data: data}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestProjectTabsBadgeEveryTabButTheActive(t *testing.T) {
	m := modelOn("a")
	report(t, m, "a1", statusWaiting)
	report(t, m, "b1", statusBusy)
	report(t, m, "c1", statusDone)
	tabs, active := m.projectTabs()
	got := map[string]string{}
	for _, tab := range tabs {
		got[tab.ID] = tab.status
	}
	want := map[string]string{"a": statusNone, "b": statusBusy, "c": statusDone}
	if active != "a" || len(tabs) != 3 || got["a"] != want["a"] || got["b"] != want["b"] || got["c"] != want["c"] {
		t.Errorf("active %q, badges %v; want a and %v", active, got, want)
	}
	m.showHome()
	if tabs, active := m.projectTabs(); active != "" || tabs[0].status != statusWaiting {
		t.Errorf("on Home: active %q, a badges %q; want no active tab and a waiting", active, tabs[0].status)
	}
}

func TestCloseProjectMovesToItsNeighbour(t *testing.T) {
	cases := []struct {
		name, active, close string
		moved               bool
		next                string
	}{
		{"the middle, active", "b", "b", true, "a"},
		{"the first, active", "a", "a", true, "b"},
		{"a background one", "a", "c", false, ""},
	}
	for _, c := range cases {
		m := modelOn(c.active)
		got, ok := m.closeProject(c.close)
		if !ok || got.moved != c.moved || got.next != c.next {
			t.Errorf("%s: got %+v %v, want moved %v next %q", c.name, got, ok, c.moved, c.next)
		}
		if tabs, _ := m.projectTabs(); len(tabs) != 2 {
			t.Errorf("%s: %d tabs left, want 2", c.name, len(tabs))
		}
	}
	m := modelOn("a")
	m.projects = m.projects[:1]
	if got, _ := m.closeProject("a"); !got.moved || got.next != "" || !slices.Equal(got.sessions, []string{"a1", "a2"}) {
		t.Errorf("closing the last tab: %+v, want Home and its sessions", got)
	}
	if !m.onHome() {
		t.Error("no project left, yet Home is not on screen")
	}
	if _, ok := m.closeProject("gone"); ok {
		t.Error("closed a project that is not open")
	}
}

func TestMoveProjectReordersEveryID(t *testing.T) {
	m := modelOn("a")
	if got := m.moveProject("a", "c"); !slices.Equal(got, []string{"b", "c", "a"}) {
		t.Errorf("a over c stores %v, want b c a", got)
	}
	if tabs, _ := m.projectTabs(); tabs[0].ID != "b" || tabs[2].ID != "a" {
		t.Errorf("strip is %+v after the move", tabs)
	}
	if got := m.moveProject("b", "b"); got != nil {
		t.Errorf("a drop on itself stores %v, want nothing", got)
	}
}

func TestNotificationsJoinProjectsAndSkipTheFocusedSession(t *testing.T) {
	m := modelOn("a")
	report(t, m, "a1", statusWaiting) // on screen: dropped
	report(t, m, "a2", statusDone)
	report(t, m, "b1", statusWaiting)
	report(t, m, "gone", statusWaiting) // no session: dropped
	report(t, m, "c1", statusBusy)      // not pending
	got := m.notifications()
	want := []notification{
		{pendingStatus{"a2", statusDone}, "a", "alpha", "A two"},
		{pendingStatus{"b1", statusWaiting}, "b", "beta", "B one"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("queue %+v, want %+v", got, want)
	}
	m.showHome()
	if n := len(m.notifications()); n != 3 {
		t.Errorf("on Home the queue has %d rows, want a1 back: nothing is focused", n)
	}
}

func TestDismissReadsAFinishedTurn(t *testing.T) {
	reads := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reads <- r.URL.Path + " " + string(body)
		_ = json.NewEncoder(w).Encode(nil)
	}))
	defer srv.Close()
	m := modelOn("a")
	m.client = clientOf(srv)
	report(t, m, "b1", statusDone)
	m.dismiss(context.Background(), "b1")
	if len(m.notifications()) != 0 {
		t.Error("a dismissed row is still queued")
	}
	select {
	case got := <-reads:
		if got != `/rpc/store.SetSessionUnread [{"sessionId":"b1","unread":false}]` {
			t.Errorf("called %q, want b1 marked read", got)
		}
	case <-time.After(5 * time.Second):
		t.Error("b1 was never marked read")
	}
}

func TestSaveLastScreenWritesTheWebsRoute(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, r.URL.Path+" "+string(body))
		_ = json.NewEncoder(w).Encode(nil)
	}))
	defer srv.Close()
	for _, id := range []string{"b", ""} {
		if err := saveLastScreen(context.Background(), clientOf(srv), id); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		`/rpc/store.SetSetting [{"key":"window.lastScreen","projectId":"","value":"/projects/b"}]`,
		`/rpc/store.SetSetting [{"key":"window.lastScreen","projectId":"","value":"/"}]`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("wrote %q, want %q", got, want)
	}
}

func clientOf(srv *httptest.Server) *backend.Client {
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return backend.New(backend.Runtime{Port: port, Token: "tok"})
}

func TestOpenHomeBringsTheHomeProjectBack(t *testing.T) {
	home := backend.Project{ID: "home", Name: "~", Path: "/home/me"}
	state := threeProjects()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		switch r.URL.Path {
		case "/rpc/store.LoadState":
			result = state
		case "/rpc/project.Home":
			result = home
		case "/rpc/store.AddProject":
			state = append(state, home)
		}
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()
	m := modelOn("a")
	m.launched, m.projects = true, nil
	m.client = clientOf(srv)
	if err := m.openHome(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p, ok := m.project(); !ok || p.ID != home.ID || !m.onHome() {
		t.Errorf("project = %+v, %v, on Home %v; want Home in view", p, ok, m.onHome())
	}
}

func TestOpenHomeSeedsNoSession(t *testing.T) {
	home := backend.Project{ID: "home", Name: "~", Path: "/home/me"}
	state := threeProjects()
	seeded := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		switch r.URL.Path {
		case "/rpc/store.LoadState":
			result = state
		case "/rpc/project.Home":
			result = home
		case "/rpc/store.AddProject":
			state = append(state, home)
		case "/rpc/store.AddSession":
			seeded = true
		}
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()
	m := modelOn("a")
	m.launched, m.projects = true, nil
	m.client = clientOf(srv)
	if err := m.openHome(context.Background()); err != nil {
		t.Fatal(err)
	}
	if seeded {
		t.Error("openHome seeded a session; the caller opens the one it needs")
	}
}
