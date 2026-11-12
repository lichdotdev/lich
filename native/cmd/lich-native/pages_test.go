package main

import (
	"context"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func pageScreen(t *testing.T, m *model) (shape, focus string) {
	t.Helper()
	lt, ok := m.shownLayoutTab()
	if !ok {
		return "", ""
	}
	return paneShape(lt.Root), lt.Focus
}

func TestShowPagePutsThePageBesideItsSessionAndMakesNoTab(t *testing.T) {
	m, _ := newLayoutsModel(t)
	if !m.showPage(context.Background(), "orch") {
		t.Fatal("refused a live session")
	}
	if shape, focus := pageScreen(t, m); shape != "row(orch,browser:orch)" || focus != "browser:orch" {
		t.Fatalf("on screen %s focused on %q, want the page right of its session and in focus", shape, focus)
	}
	tabs, active := m.stripTabs()
	for _, tab := range tabs {
		if tab.layout {
			t.Fatalf("the strip lists %+v: a page must not make a tab", tab)
		}
	}
	if active != "lich" {
		t.Fatalf("strip on %q, want the session's project", active)
	}
	if m.showPage(context.Background(), "gone") {
		t.Fatal("showed the page of a session no project holds")
	}
}

func TestAPageFollowsItsSessionIntoAndOutOfALayout(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.showPage(ctx, "wa")
	if !m.splitBeside(ctx, "orch", "wa", edgeRight, false) {
		t.Fatal("a session showing its page was refused a split")
	}
	if shape, _ := pageScreen(t, m); shape != "row(orch,row(wa,browser:wa))" {
		t.Fatalf("on screen %s, want the page beside its session inside the layout", shape)
	}
	m.mu.Lock()
	stored := paneShape(m.layouts.tabs[0].Root)
	m.mu.Unlock()
	if stored != "row(orch,wa)" {
		t.Fatalf("stored %s: a page is no pane of a stored layout", stored)
	}
	m.unsplit(ctx, "wa")
	m.setActive(ctx, "wa")
	if shape, _ := pageScreen(t, m); shape != "row(wa,browser:wa)" {
		t.Fatalf("on screen %s, want the session out of the layout with its page still beside it", shape)
	}
}

func TestALayoutTakesAnotherSessionBesideOneShowingItsPage(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.showPage(ctx, "orch")
	if !m.splitBeside(ctx, "orch", "notes", edgeBottom, false) {
		t.Fatal("split refused beside a session showing its page")
	}
	if shape, _ := pageScreen(t, m); shape != "column(row(orch,browser:orch),notes)" {
		t.Fatalf("on screen %s", shape)
	}
}

func TestMinimizeTakesThePageOffScreenAndLeavesTheBrowserRunning(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: true})
	m.showPage(ctx, "orch")
	if !m.minimizePage("orch") || m.minimizePage("orch") {
		t.Fatal("minimize must report the one page it took")
	}
	if _, ok := m.shownLayoutTab(); ok {
		t.Fatal("the page stayed on screen")
	}
	if st, ok := m.browserState("orch"); !ok || !st.Open {
		t.Fatal("minimizing closed the browser")
	}
	m.showPage(ctx, "orch")
	if shape, _ := pageScreen(t, m); shape != "row(orch,browser:orch)" {
		t.Fatalf("on screen %q, want the page back", shape)
	}
}

func TestUnsplitOfASessionAloneMinimizesItsPage(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.showPage(ctx, "orch")
	m.unsplit(ctx, "orch")
	if m.pageShown("orch") {
		t.Fatal("the page stayed beside a session taken out of its only split")
	}
}

func TestClosedBrowserTakesItsPageOffScreen(t *testing.T) {
	m, _ := newLayoutsModel(t)
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: true})
	m.showPage(context.Background(), "orch")
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: false})
	if m.pageShown("orch") {
		t.Fatal("kept the page of a closed browser")
	}
	if _, ok := m.browserState("orch"); ok {
		t.Fatal("kept a closed browser")
	}
	if _, project, active := layoutScreen(t, m); project != "lich" || active != "orch" {
		t.Fatalf("project %q active %q, want the session alone where it was", project, active)
	}
}

func TestBrowserMovedToAWindowTakesItsPageOffScreen(t *testing.T) {
	m, _ := newLayoutsModel(t)
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: true})
	m.showPage(context.Background(), "orch")
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: true, Headed: true})
	if m.pageShown("orch") {
		t.Fatal("kept the page of a browser in its own window")
	}
}

func TestThePagesDividerAndRotationAreItsOwn(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.showPage(ctx, "orch")
	m.setRatio(soloLayoutPrefix+"orch", nil, 0.3)
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	m.showPage(ctx, "wa")
	layoutID, _, _ := layoutScreen(t, m)
	m.setRatio(layoutID, []bool{true}, 0.7)
	if !m.rotateFocused() {
		t.Fatal("rotate refused a session showing its page")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if got := m.pages["orch"].Ratio; got != 0.3 {
		t.Fatalf("orch's page at %v, want the ratio dragged while alone", got)
	}
	if page := m.pages["wa"]; page.Ratio != 0.7 || page.Axis != splitColumn {
		t.Fatalf("wa's page %+v, want its divider at 0.7 and the page under its terminal", page)
	}
	if root := m.layouts.tabs[0].Root; root.Ratio != 0.5 || root.Axis != splitRow {
		t.Fatalf("the layout's own split moved: %+v", root)
	}
}

func TestPickingTheSessionAgainGivesTheKeysBackToItsTerminal(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.showPage(ctx, "orch")
	m.setActive(ctx, "notes")
	m.setActive(ctx, "orch")
	if _, focus := pageScreen(t, m); focus != "orch" {
		t.Fatalf("focus on %q, want the terminal", focus)
	}
	m.focusPane(ctx, "browser:orch")
	if _, focus := pageScreen(t, m); focus != "browser:orch" {
		t.Fatalf("focus on %q, want the page", focus)
	}
}

func TestZoomOfASessionAloneCoversItsPageOrItsTerminal(t *testing.T) {
	m, _ := newLayoutsModel(t)
	m.showPage(context.Background(), "orch")
	m.toggleZoom()
	if lt, _ := m.shownLayoutTab(); !lt.Zoomed || lt.Focus != "browser:orch" {
		t.Fatalf("shown %+v, want the page zoomed over its terminal", lt)
	}
	m.toggleZoom()
	if lt, _ := m.shownLayoutTab(); lt.Zoomed {
		t.Fatal("the zoom did not come back")
	}
}

func TestPagesRoundTripAndRefuseABrokenAxis(t *testing.T) {
	back, err := parsePages(`{"s":{"axis":"column","ratio":0.99,"zoomed":true}}`)
	if err != nil {
		t.Fatal(err)
	}
	if p := back["s"]; p == nil || p.Axis != splitColumn || p.Ratio != ratioMax || !p.Zoomed {
		t.Fatalf("read %+v, want the page with its ratio clamped", back["s"])
	}
	if empty, err := parsePages(""); err != nil || empty == nil {
		t.Fatalf("no stored value: %v, %v", empty, err)
	}
	for _, raw := range []string{`{"s":{"axis":"diagonal","ratio":0.5}}`, `{"s":null}`, `[`} {
		if _, err := parsePages(raw); err == nil {
			t.Fatalf("read %s without an error", raw)
		}
	}
}

func TestPrunePagesDropsOnlyWhatIsRefused(t *testing.T) {
	m, _ := newLayoutsModel(t)
	ctx := context.Background()
	m.showPage(ctx, "orch")
	m.showPage(ctx, "wa")
	m.mu.Lock()
	m.prunePagesLocked(func(session string) bool { return session == "wa" })
	m.mu.Unlock()
	if m.pageShown("orch") || !m.pageShown("wa") {
		t.Fatal("prune must drop orch's page and keep wa's")
	}
}
