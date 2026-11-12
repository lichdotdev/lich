package browser

import (
	"slices"
	"testing"

	"github.com/chromedp/cdproto/target"
	"github.com/lichdotdev/lich/api"
)

// stackOf starts a stack on a tab "root" and adds tabs named by id, each
// opened by the tab named in openers (absent for none).
func stackOf(ids []target.ID, openers map[target.ID]target.ID) *tabStack {
	s := &tabStack{}
	s.start(&tab{id: "root"})
	for _, id := range ids {
		s.add(&tab{id: id, opener: s.byID(openers[id])})
	}
	return s
}

func order(s *tabStack) []target.ID {
	var out []target.ID
	for _, t := range s.list() {
		out = append(out, target.ID(t.ID))
	}
	return out
}

func TestTabStackPutsANewTabAfterTheOnesItsOpenerOpened(t *testing.T) {
	s := stackOf([]target.ID{"blank", "a", "b"}, map[target.ID]target.ID{"a": "root", "b": "root"})
	if got, want := order(s), []target.ID{"root", "a", "b", "blank"}; !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestTabStackClosingTheFrontTabBringsItsOpenerBack(t *testing.T) {
	s := stackOf([]target.ID{"a", "b"}, map[target.ID]target.ID{"a": "root"})
	s.setFront(s.byID("a"))
	if got, front := s.remove("a"); got == nil || !front {
		t.Fatalf("remove(a) = %v, %v; want the tab, in front", got, front)
	}
	if s.frontTab().id != "root" {
		t.Fatalf("front = %s, want the tab that opened the closed one", s.frontTab().id)
	}
}

func TestTabStackClosingAFrontTabNobodyOpenedBringsItsNeighbour(t *testing.T) {
	s := stackOf([]target.ID{"a", "b"}, nil)
	s.setFront(s.byID("a"))
	s.remove("a")
	if s.frontTab().id != "b" {
		t.Fatalf("front = %s, want the tab that took its place", s.frontTab().id)
	}
	s.remove("b")
	if s.frontTab().id != "root" {
		t.Fatalf("front = %s, want the tab before the last one", s.frontTab().id)
	}
}

func TestTabStackClosingATabBehindKeepsTheFront(t *testing.T) {
	s := stackOf([]target.ID{"a", "b"}, map[target.ID]target.ID{"a": "root", "b": "a"})
	s.setFront(s.byID("b"))
	if _, front := s.remove("a"); front {
		t.Fatal("removing a tab behind said it was in front")
	}
	if s.frontTab().id != "b" {
		t.Fatalf("front = %s, want it unchanged", s.frontTab().id)
	}
	if s.byID("b").opener == nil || s.byID("b").opener.id != "root" {
		t.Fatal("a tab whose opener closed does not fall back to its opener's opener")
	}
}

func TestTabStackNeverRemovesTheLastTab(t *testing.T) {
	s := stackOf(nil, nil)
	if got, _ := s.remove("root"); got != nil {
		t.Fatalf("remove(root) = %v, want nothing", got)
	}
	if got, _ := stackOf([]target.ID{"a"}, nil).remove("unknown"); got != nil {
		t.Fatalf("remove(unknown) = %v, want nothing", got)
	}
}

func TestTabStackListsInfoAndMarksTheFront(t *testing.T) {
	s := stackOf([]target.ID{"a"}, nil)
	a := s.byID("a")
	s.setFront(a)
	if !s.setInfo(a, "https://example.com/", "Example") {
		t.Fatal("setInfo with a new URL said nothing changed")
	}
	if s.setInfo(a, "https://example.com/", "Example") {
		t.Fatal("setInfo with the same URL and title said it changed")
	}
	got := s.list()[1]
	if got.URL != "https://example.com/" || got.Title != "Example" || !got.Front || s.list()[0].Front {
		t.Fatalf("list = %+v, want a's info with a alone in front", s.list())
	}
}

func TestTabStackAnswersTheDialogWhereItWasOpened(t *testing.T) {
	s := stackOf([]target.ID{"a"}, nil)
	root, a := s.byID("root"), s.byID("a")
	s.setFront(a)
	if s.dialogHolder() != a {
		t.Fatal("with no dialog open, the holder is not the front tab")
	}
	s.setDialog(root)
	if s.dialogHolder() != root {
		t.Fatal("the dialog's tab behind the front one is not its holder")
	}
	if s.dialogClosed(a) {
		t.Fatal("a tab holding no dialog closed it")
	}
	if !s.dialogClosed(root) || s.dialogHolder() != a {
		t.Fatal("once the dialog closed, the holder is not the front tab")
	}
}

// withDevTools is s with tab id inspected by a DevTools target named after it.
func withDevTools(t *testing.T, s *tabStack, id target.ID) *tab {
	t.Helper()
	tb := s.byID(id)
	if !s.setDevTools(tb, &devtools{id: "dt-" + id, cancel: func() {}}) {
		t.Fatalf("setDevTools(%s) refused", id)
	}
	return tb
}

func TestTabStackListsADevToolsTabRightAfterTheTabItInspects(t *testing.T) {
	s := stackOf([]target.ID{"a"}, nil)
	withDevTools(t, s, "root")
	if got, want := order(s), []target.ID{"root", "dt-root", "a"}; !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if dt := s.list()[1]; !dt.DevTools || dt.URL != "" || dt.Front {
		t.Fatalf("DevTools tab = %+v, want a DevTools tab with no URL, behind the page", dt)
	}
}

func TestTabStackInspectingPutsTheDevToolsTabInFrontInThePaneOnly(t *testing.T) {
	s := stackOf(nil, nil)
	root := withDevTools(t, s, "root")
	s.inspect(root)
	pane := s.list()
	if pane[0].Front || !pane[1].Front {
		t.Fatalf("pane tabs = %+v, want the DevTools tab in front", pane)
	}
	agent := agentTabs(pane)
	if len(agent) != 1 || agent[0].ID != "root" || !agent[0].Front {
		t.Fatalf("agent tabs = %+v, want only the page, in front", agent)
	}
	if s.shown() != root.devtools || s.frontTab() != root {
		t.Fatal("the pane does not show the DevTools of the tab the agent drives")
	}
}

func TestTabStackSelectingATabLeavesDevTools(t *testing.T) {
	s := stackOf([]target.ID{"a"}, nil)
	s.inspect(withDevTools(t, s, "root"))
	s.setFront(s.byID("root"))
	if s.shown() != nil {
		t.Fatal("selecting the page left its DevTools on the pane")
	}
}

func TestTabStackInspectsNoTabWithoutDevTools(t *testing.T) {
	s := stackOf([]target.ID{"a"}, nil)
	s.inspect(s.byID("a"))
	if s.shown() != nil || s.frontTab().id != "root" {
		t.Fatal("inspect moved to a tab with no DevTools")
	}
}

func TestTabStackClosingTheInspectedTabTakesItsDevToolsAndShowsThePageBack(t *testing.T) {
	s := stackOf([]target.ID{"a"}, nil)
	a := withDevTools(t, s, "a")
	s.inspect(a)
	got, front := s.remove("a")
	if got == nil || !front || got.devtools == nil {
		t.Fatalf("remove(a) = %v, %v; want the front tab with its DevTools to close", got, front)
	}
	if s.shown() != nil || s.inspected("dt-a") != nil {
		t.Fatal("the closed tab's DevTools stayed on the pane")
	}
	if got, want := order(s), []target.ID{"root"}; !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestTabStackDroppingDevToolsSaysWhetherThePaneShowedIt(t *testing.T) {
	s := stackOf(nil, nil)
	root := withDevTools(t, s, "root")
	if _, shown := s.dropDevTools(root); shown {
		t.Fatal("DevTools behind its page reported as shown")
	}
	withDevTools(t, s, "root")
	s.inspect(root)
	if d, shown := s.dropDevTools(root); d == nil || !shown || s.shown() != nil {
		t.Fatalf("dropDevTools = %v, %v; want the shown DevTools, and the page back", d, shown)
	}
	if d, _ := s.dropDevTools(root); d != nil {
		t.Fatal("a tab dropped its DevTools twice")
	}
}

func TestTabStackGivesATabOneDevTools(t *testing.T) {
	s := stackOf(nil, nil)
	root := withDevTools(t, s, "root")
	if s.setDevTools(root, &devtools{id: "second"}) {
		t.Fatal("a tab took a second DevTools")
	}
}

func TestTabStackPausedMarksTheDevToolsTab(t *testing.T) {
	s := stackOf([]target.ID{"a"}, nil)
	root := withDevTools(t, s, "root")
	if s.setPaused(s.byID("a"), true) {
		t.Fatal("a tab with no DevTools took a pause")
	}
	if !s.setPaused(root, true) || s.setPaused(root, true) {
		t.Fatal("setPaused did not report the change once")
	}
	if !s.anyPaused() || !s.list()[1].Paused {
		t.Fatalf("tabs = %+v, want the DevTools tab paused", s.list())
	}
	s.setPaused(root, false)
	if s.anyPaused() {
		t.Fatal("still paused after the page went on")
	}
}

func TestAgentTabsLeaveDevToolsOutAndKeepTheirOrder(t *testing.T) {
	pane := []api.BrowserTab{{ID: "a"}, {ID: "dt-a", DevTools: true}, {ID: "b", Front: true}, {ID: "dt-b", DevTools: true, Paused: true}}
	want := []api.BrowserTab{{ID: "a"}, {ID: "b", Front: true}}
	if got := agentTabs(pane); !slices.Equal(got, want) {
		t.Fatalf("agentTabs = %+v, want %+v", got, want)
	}
}
