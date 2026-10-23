package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/backend"

	"github.com/lichdotdev/lich/api"
)

// layoutsBackend answers every call with null and keeps the last value
// stored under layoutsKey.
type layoutsBackend struct {
	mu     sync.Mutex
	stored string
	saves  int
}

func (b *layoutsBackend) last(t *testing.T, saves int) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		n, s := b.saves, b.stored
		b.mu.Unlock()
		if n >= saves {
			return s
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("layouts saved %d times, want %d", b.saves, saves)
	return ""
}

func newLayoutsModel(t *testing.T) (*model, *layoutsBackend) {
	t.Helper()
	b := new(layoutsBackend)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var args []api.SetSettingOptions
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &args)
		if r.URL.Path == "/rpc/store.SetSetting" && len(args) == 1 && args[0].Key == layoutsKey {
			b.mu.Lock()
			b.stored, b.saves = args[0].Value, b.saves+1
			b.mu.Unlock()
		}
		_ = json.NewEncoder(w).Encode(nil)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	m := newModel(backend.New(backend.Runtime{Port: port, Token: "tok"}), func() {})
	m.projects = []backend.Project{
		{ID: "lich", Name: "lich", Path: "/dev/lich", ActiveSessionID: "orch", Sessions: []backend.Session{{ID: "orch", Label: "orchestrator"}, {ID: "notes", Label: "notes"}}},
		{ID: "plugin", Name: "lich-plugin", Path: "/dev/plugin", ActiveSessionID: "wa", Sessions: []backend.Session{{ID: "wa", Label: "worker-a"}, {ID: "wb", Label: "worker-b"}}},
	}
	m.activeProject, m.active = "lich", "orch"
	return m, b
}

func layoutScreen(t *testing.T, m *model) (layout, project, active string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	lt, _ := m.shownLayoutLocked()
	return lt.ID, m.activeProject, m.active
}

func TestSplitBesideOpensALayoutAcrossProjects(t *testing.T) {
	m, b := newLayoutsModel(t)
	ctx := context.Background()
	if !m.splitBeside(ctx, "orch", "wa", edgeRight, false) {
		t.Fatal("split refused")
	}
	id, project, active := layoutScreen(t, m)
	if id == "" || project != "plugin" || active != "wa" {
		t.Fatalf("on screen: layout %q project %q active %q, want the new pane in focus", id, project, active)
	}
	tabs, shown := m.stripTabs()
	last := tabs[len(tabs)-1]
	if !last.layout || last.Name != "orchestrator" || last.Path != "orchestrator, worker-a" || shown != id {
		t.Fatalf("strip %+v, shown %q", tabs, shown)
	}
	saved, err := parseLayouts(b.last(t, 1))
	if err != nil || len(saved.tabs) != 1 || paneShape(saved.tabs[0].Root) != "row(orch,wa)" {
		t.Fatalf("stored %+v, %v", saved.tabs, err)
	}
}

func TestASessionInALayoutIsAlwaysShownThere(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	layoutID, _, _ := layoutScreen(t, m)

	m.setActive(ctx, "notes")
	if id, project, active := layoutScreen(t, m); id != "" || project != "lich" || active != "notes" {
		t.Fatalf("a session outside layouts: layout %q project %q active %q, want it alone", id, project, active)
	}
	m.setActive(ctx, "orch")
	if id, project, active := layoutScreen(t, m); id != layoutID || project != "lich" || active != "orch" {
		t.Fatalf("back to a pane: layout %q project %q active %q", id, project, active)
	}
	if !m.setActiveProject("plugin") {
		t.Fatal("project not found")
	}
	if id, _, active := layoutScreen(t, m); id != "" || active != "wa" {
		t.Fatalf("a project tab: layout %q active %q, want the project's own session", id, active)
	}
	if !m.showLayout(ctx, layoutID) {
		t.Fatal("showLayout refused")
	}
	if id, _, active := layoutScreen(t, m); id != layoutID || active != "orch" {
		t.Fatalf("the layout tab: layout %q active %q, want the pane it last focused", id, active)
	}
}

func TestSplitBesideAsksBeforeTakingAPaneFromAnotherLayout(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	m.splitBeside(ctx, "notes", "wb", edgeRight, false)
	if m.splitBeside(ctx, "notes", "wa", edgeBottom, false) {
		t.Fatal("moved a pane out of its layout without move")
	}
	if !m.splitBeside(ctx, "notes", "wa", edgeBottom, true) {
		t.Fatal("refused the move it was told to make")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// The move left orch's layout with orch alone, which is no layout.
	if len(m.layouts.tabs) != 1 || paneShape(m.layouts.tabs[0].Root) != "row(column(notes,wa),wb)" {
		t.Fatalf("got %+v", m.layouts.tabs)
	}
}

func TestClosingTheFocusedPaneFocusesWhatIsLeft(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	m.splitBeside(ctx, "wa", "wb", edgeBottom, false)
	layoutID, _, _ := layoutScreen(t, m)

	if _, active, ok := m.closeSession("wb"); !ok || active != "orch" {
		t.Fatalf("active %q after closing the focused pane, want the first pane left", active)
	}
	if id, project, _ := layoutScreen(t, m); id != layoutID || project != "lich" {
		t.Fatalf("layout %q project %q", id, project)
	}
	m.closeSession("orch")
	m.closeSession("wa")
	if id, _, _ := layoutScreen(t, m); id != "" {
		t.Fatalf("an emptied layout stayed on screen: %q", id)
	}
	if tabs, _ := m.stripTabs(); len(tabs) != 2 {
		t.Fatalf("strip kept a layout with no pane: %+v", tabs)
	}
}

func TestUnsplitAndCloseLayoutLeaveTheSessionsRunning(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	m.splitBeside(ctx, "wa", "wb", edgeBottom, false)
	layoutID, _, _ := layoutScreen(t, m)
	m.unsplit(ctx, "wb")
	if id, _, active := layoutScreen(t, m); id != layoutID || active != "orch" {
		t.Fatalf("unsplit: layout %q active %q, want the pane left in focus", id, active)
	}
	m.closeLayout(ctx, layoutID)
	if id, project, active := layoutScreen(t, m); id != "" || project != "lich" || active != "orch" {
		t.Fatalf("closed layout: %q %q %q, want its focus alone", id, project, active)
	}
	if !m.liveLocked("wa") || !m.liveLocked("orch") || !m.liveLocked("wb") {
		t.Fatal("a session closed with its layout")
	}
}

func TestTheLastPaneLeftClosesTheLayoutAndShowsItsSessionAlone(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	m.unsplit(ctx, "orch")
	if id, project, active := layoutScreen(t, m); id != "" || project != "plugin" || active != "wa" {
		t.Fatalf("unsplit: layout %q project %q active %q, want wa alone", id, project, active)
	}
	if tabs, _ := m.stripTabs(); len(tabs) != 2 {
		t.Fatalf("the layout tab stayed: %+v", tabs)
	}
	m.splitBeside(ctx, "orch", "wb", edgeRight, false)
	if _, active, ok := m.closeSession("wb"); !ok || active != "orch" {
		t.Fatalf("closing the focused pane left %q on screen, want orch", active)
	}
	if id, project, _ := layoutScreen(t, m); id != "" || project != "lich" {
		t.Fatalf("closed session: layout %q project %q, want orch alone in lich", id, project)
	}
}

func TestReloadDropsPanesOfSessionsGone(t *testing.T) {
	m, b := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	b.last(t, 1)
	m.mu.Lock()
	m.projects[1].Sessions = m.projects[1].Sessions[1:]
	pruned := m.layouts.prune(m.liveLocked)
	m.mu.Unlock()
	if !pruned {
		t.Fatal("prune kept a session no project holds")
	}
	if lt, ok := m.shownLayoutTab(); ok {
		t.Fatalf("a layout left with one pane stayed: %+v", lt)
	}
}

func TestCyclePaneAndZoomFollowTheLayoutOnScreen(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	m.splitBeside(ctx, "wa", "wb", edgeBottom, false)
	m.cyclePane(ctx, 1)
	if _, _, active := layoutScreen(t, m); active != "orch" {
		t.Fatalf("cycle from the last pane landed on %q, want the first", active)
	}
	m.toggleZoom()
	if lt, _ := m.shownLayoutTab(); !lt.Zoomed {
		t.Fatal("zoom did not take")
	}
}

func TestMoveLayoutReordersOnlyAmongLayouts(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	m.splitBeside(ctx, "notes", "wb", edgeRight, false)
	m.mu.Lock()
	first, second := m.layouts.tabs[0].ID, m.layouts.tabs[1].ID
	m.mu.Unlock()
	if m.moveLayout(first, "lich") {
		t.Fatal("a layout moved among the projects")
	}
	if !m.moveLayout(first, second) {
		t.Fatal("layouts did not reorder")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.layouts.tabs[0].ID != second {
		t.Fatalf("order %s, %s", m.layouts.tabs[0].ID, m.layouts.tabs[1].ID)
	}
}

func TestRenameLayoutNamesItsTabAndIsStored(t *testing.T) {
	m, b := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	b.last(t, 1)
	id, _, _ := layoutScreen(t, m)
	if !m.renameLayout(id, "review") || m.renameLayout("missing", "x") {
		t.Fatal("rename reported the wrong layouts")
	}
	m.saveLayouts()
	saved, err := parseLayouts(b.last(t, 2))
	if err != nil || saved.tabs[0].Name != "review" {
		t.Fatalf("stored %+v, %v", saved.tabs, err)
	}
	if tabs, _ := m.stripTabs(); tabs[len(tabs)-1].Name != "review" {
		t.Fatalf("strip %+v", tabs)
	}
}

func TestMovePaneRearrangesOneLayoutOnly(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	m.splitBeside(ctx, "wa", "wb", edgeRight, false)
	if !m.movePane(ctx, "wb", "orch", edgeBottom) {
		t.Fatal("move refused")
	}
	lt, _ := m.shownLayoutTab()
	if got, want := paneShape(lt.Root), "row(column(orch,wb),wa)"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if _, _, active := layoutScreen(t, m); active != "wb" {
		t.Fatalf("focus %q, want the pane moved", active)
	}
	if m.movePane(ctx, "notes", "orch", edgeLeft) {
		t.Fatal("moved a session that is in no layout")
	}
	if !m.rotateFocused() {
		t.Fatal("rotate refused")
	}
	lt, _ = m.shownLayoutTab()
	if got, want := paneShape(lt.Root), "row(row(orch,wb),wa)"; got != want {
		t.Fatalf("after rotate got %s, want %s", got, want)
	}
}

func TestShowOtherThanNeverLeavesTheDraggedSessionAloneOnScreen(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "notes", "wa", edgeRight, false)
	m.setActiveProject("lich")
	m.mu.Lock()
	m.active, m.shownLayout = "orch", ""
	m.mu.Unlock()
	m.showOtherThan(ctx, "notes")
	if _, _, active := layoutScreen(t, m); active != "orch" {
		t.Fatalf("active %q, want orch left alone: it is not the dragged session", active)
	}
	m.mu.Lock()
	m.active = "notes"
	m.mu.Unlock()
	m.showOtherThan(ctx, "notes")
	if id, _, active := layoutScreen(t, m); id != "" || active != "orch" {
		t.Fatalf("layout %q active %q, want orch, the session in no layout", id, active)
	}
}

func TestALayoutTabSaysAPaneWaitsWhileAnotherTabIsUp(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	report(t, m, "wa", statusWaiting)
	layoutTab := func() projectTab {
		tabs, _ := m.stripTabs()
		return tabs[len(tabs)-1]
	}
	if got := layoutTab().status; got != statusNone {
		t.Fatalf("the layout on screen badged %q, want none: its panes say it", got)
	}
	m.setActive(ctx, "notes")
	if got := layoutTab().status; got != statusWaiting {
		t.Fatalf("the layout off screen badged %q, want waiting", got)
	}
}
