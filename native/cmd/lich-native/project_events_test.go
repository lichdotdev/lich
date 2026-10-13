package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

// rpcLog serves every call with answer (nil for none), failing the ones whose
// path is in refuse, and records what was asked.
type rpcLog struct {
	mu     sync.Mutex
	calls  []string
	refuse map[string]bool
	answer map[string]any
}

func (l *rpcLog) client(t *testing.T) *lichclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		l.mu.Lock()
		l.calls = append(l.calls, r.URL.Path+" "+string(body))
		refused := l.refuse[r.URL.Path]
		answer := l.answer[r.URL.Path]
		l.mu.Unlock()
		if refused {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "refused"})
			return
		}
		_ = json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(srv.Close)
	return clientOf(srv)
}

func (l *rpcLog) seen() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.calls)
}

func tabIDs(m *model) []string {
	tabs, _ := m.projectTabs()
	ids := make([]string, len(tabs))
	for i, tab := range tabs {
		ids[i] = tab.ID
	}
	return ids
}

func TestRefusedCloseLeavesTheTabWhereItWas(t *testing.T) {
	rpc := &rpcLog{refuse: map[string]bool{"/rpc/store.CloseProject": true}}
	m := modelOn("b")
	m.client = rpc.client(t)

	sessions, err := m.askClose(context.Background(), "b")
	if err == nil {
		t.Fatal("a refused close came back as a success")
	}
	if sessions != nil {
		t.Errorf("sessions %v after a refusal; their terminals must stay up", sessions)
	}
	if got := tabIDs(m); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("tabs %v after a refused close, want a b c", got)
	}
}

func TestAskCloseTakesNoTabItself(t *testing.T) {
	rpc := &rpcLog{}
	m := modelOn("b")
	m.client = rpc.client(t)

	sessions, err := m.askClose(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sessions, []string{"a1", "a2"}) {
		t.Errorf("sessions %v, want the project's two", sessions)
	}
	if got := rpc.seen(); !slices.Equal(got, []string{`/rpc/store.CloseProject ["a"]`}) {
		t.Errorf("calls %q, want one CloseProject", got)
	}
	if got := tabIDs(m); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("tabs %v before the event, want a b c: the strip answers to project-closed", got)
	}
}

func TestProjectClosedEventTakesTheTabAndTellsTheWindow(t *testing.T) {
	m := modelOn("b")
	var told []closedProject
	m.projectClosed = func(c closedProject) { told = append(told, c) }

	send(m, "project-closed", map[string]string{"id": "b"})

	if got := tabIDs(m); !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("tabs %v, want a c", got)
	}
	want := []closedProject{{sessions: []string{"b1"}, moved: true, next: "a"}}
	if len(told) != 1 || !slices.Equal(told[0].sessions, want[0].sessions) || told[0].moved != true || told[0].next != "a" {
		t.Errorf("told %+v, want %+v: the screen moves to the neighbour", told, want)
	}
}

func TestProjectClosedEventForAProjectNotOpenSaysNothing(t *testing.T) {
	m := modelOn("b")
	m.projectClosed = func(closedProject) { t.Error("the window was told of a project it did not hold") }

	send(m, "project-closed", map[string]string{"id": "gone"})

	if got := tabIDs(m); len(got) != 3 {
		t.Errorf("tabs %v, want all three", got)
	}
}

func TestDragShowsAtOnceAndTheEventSettlesIt(t *testing.T) {
	m := modelOn("a")
	ids := m.moveProject("a", "c")
	if !slices.Equal(ids, []string{"b", "c", "a"}) {
		t.Fatalf("drag stores %v, want b c a", ids)
	}
	if got := tabIDs(m); !slices.Equal(got, []string{"b", "c", "a"}) {
		t.Errorf("strip %v during the drag, want b c a", got)
	}
	if m.projects[0].ID != "a" {
		t.Error("the drag wrote the store's order before the backend confirmed it")
	}

	send(m, "projects-reordered", map[string]any{"ids": ids})

	if m.projects[0].ID != "b" || m.projects[2].ID != "a" {
		t.Errorf("store order %v after the event, want b c a", tabIDs(m))
	}
	if m.pendingOrder != nil {
		t.Errorf("drag state %v outlived its confirmation", m.pendingOrder)
	}
}

func TestRefusedReorderRevertsTheDrag(t *testing.T) {
	rpc := &rpcLog{refuse: map[string]bool{"/rpc/store.ReorderProjects": true}}
	m := modelOn("a")
	m.client = rpc.client(t)
	ids := m.moveProject("a", "c")

	if err := m.storeOrder(context.Background(), ids); err == nil {
		t.Fatal("a refused reorder came back as a success")
	}
	if got := tabIDs(m); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("strip %v after a refusal, want a b c", got)
	}
}

func TestAReorderEventFromElsewhereMovesTheStrip(t *testing.T) {
	m := modelOn("a")

	send(m, "projects-reordered", map[string]any{"ids": []string{"c", "a", "b"}})

	if got := tabIDs(m); !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Errorf("strip %v, want c a b", got)
	}
}

func TestADragOverAnOlderOneKeepsItsOwnPendingOrder(t *testing.T) {
	m := modelOn("a")
	first := m.moveProject("a", "c")
	second := m.moveProject("b", "a")

	send(m, "projects-reordered", map[string]any{"ids": first})

	if m.pendingOrder == nil || !slices.Equal(m.pendingOrder, second) {
		t.Errorf("pending %v, want the second drag's %v: the first's event is not its confirmation", m.pendingOrder, second)
	}
}

func closedRows() []lichclient.RecentProject {
	return []lichclient.RecentProject{{ID: "x", Name: "ex", Path: "/p/ex"}, {ID: "y", Name: "why", Path: "/p/why"}}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("never: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func countOf(calls []string, path string) int {
	n := 0
	for _, c := range calls {
		if len(c) >= len(path) && c[:len(path)] == path {
			n++
		}
	}
	return n
}

func TestClosedProjectsAreFetchedOnceForEveryReader(t *testing.T) {
	rpc := &rpcLog{answer: map[string]any{
		"/rpc/store.RecentProjects":     closedRows(),
		"/rpc/store.ClosedProjectCount": 2,
		"/rpc/project.Missing":          []string{"/p/why"},
	}}
	redraws := make(chan struct{}, 16)
	book := newClosedProjects(rpc.client(t), func() { redraws <- struct{}{} })

	book.read("")
	waitFor(t, "the answer", func() bool { return len(book.read("").rows) == 2 })
	menu, palette := book.read(""), book.read("")

	if len(menu.rows) != 2 || len(palette.rows) != 2 || !palette.missing["/p/why"] {
		t.Errorf("menu %+v, palette %+v; want both to read the two rows with why missing", menu, palette)
	}
	if n := countOf(rpc.seen(), "/rpc/store.RecentProjects"); n != 1 {
		t.Errorf("RecentProjects asked %d times, want 1", n)
	}
}

func TestAnEventMakesTheNextReadFetchAgain(t *testing.T) {
	rpc := &rpcLog{answer: map[string]any{
		"/rpc/store.RecentProjects":     closedRows(),
		"/rpc/store.ClosedProjectCount": 2,
		"/rpc/project.Missing":          []string{},
	}}
	book := newClosedProjects(rpc.client(t), func() {})
	book.read("")
	waitFor(t, "first answer", func() bool { return len(book.read("").rows) == 2 })

	send(modelWithBook(book), "project-closed", map[string]string{"id": "z"})

	stale := book.read("")
	if len(stale.rows) != 2 {
		t.Errorf("a stale read answered %d rows, want the held two: a list never flashes empty", len(stale.rows))
	}
	waitFor(t, "the refetch", func() bool { return countOf(rpc.seen(), "/rpc/store.RecentProjects") == 2 })
}

func TestAFailedFetchWaitsForTheNextEventInsteadOfRetryingEveryFrame(t *testing.T) {
	rpc := &rpcLog{refuse: map[string]bool{"/rpc/store.RecentProjects": true}}
	book := newClosedProjects(rpc.client(t), func() {})

	book.read("")
	waitFor(t, "the failed fetch", func() bool { return countOf(rpc.seen(), "/rpc/store.RecentProjects") == 1 })
	for range 5 {
		book.read("")
	}
	time.Sleep(50 * time.Millisecond)
	if n := countOf(rpc.seen(), "/rpc/store.RecentProjects"); n != 1 {
		t.Errorf("asked %d times across five frames, want 1", n)
	}

	book.stale()
	book.read("")
	waitFor(t, "the ask after the event", func() bool { return countOf(rpc.seen(), "/rpc/store.RecentProjects") == 2 })
}

func TestOnlyThePlainListAndTheCurrentTermAreHeld(t *testing.T) {
	book := newClosedProjects((&rpcLog{}).client(t), func() {})
	book.entries[""] = &closedEntry{tried: true}
	book.entries["old"] = &closedEntry{tried: true}

	book.read("new")

	if _, ok := book.entries["old"]; ok {
		t.Error("a term the palette moved on from is still held")
	}
	if _, ok := book.entries[""]; !ok {
		t.Error("the plain list was dropped for a term")
	}
}

func modelWithBook(book *closedProjects) *model {
	m := newModel(nil, func() {})
	m.closed = book
	return m
}

// closedHolding is a store that already holds the plain list's answer.
func closedHolding(a closedAnswer) *closedProjects {
	book := newClosedProjects(nil, func() {})
	book.entries[""] = &closedEntry{tried: true, answer: a}
	return book
}
