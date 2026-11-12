package main

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

// twoTabs lists tabs a and b, front in front.
func twoTabs(front string) api.BrowserState {
	return api.BrowserState{Owner: "orch", Open: true, Tabs: []api.BrowserTab{
		{ID: "a", Front: front == "a"}, {ID: "b", Front: front == "b"},
	}}
}

func frameWidth(m *model) int {
	_, f, _ := m.browserView("orch")
	return f.size.X
}

func TestSwitchingTabsShowsThatTabsOwnLastFrame(t *testing.T) {
	m, _ := newLayoutsModel(t)
	name := api.BrowserFrameEvents.Of("orch").Name
	m.applyBrowserState(twoTabs("a"))
	send(m, name, tabPayload(t, "a", 8))
	m.applyBrowserState(twoTabs("b"))
	if w := frameWidth(m); w != 0 {
		t.Fatalf("tab b, never in front, shows a %d wide frame, want the empty state", w)
	}
	send(m, name, tabPayload(t, "b", 9))
	m.applyBrowserState(twoTabs("a"))
	if w := frameWidth(m); w != 8 {
		t.Fatalf("back on tab a it shows a %d wide frame, want a's own last one (8)", w)
	}
}

func TestOldTabsFrameDecodedAfterTheSwitchIsNotTheNewTabs(t *testing.T) {
	m, _ := newLayoutsModel(t)
	name := api.BrowserFrameEvents.Of("orch").Name
	m.applyBrowserState(twoTabs("b"))
	if err := m.applyBrowserFrame("orch", tabFrame(t, name, "a", 8), 1); err != nil {
		t.Fatal(err)
	}
	if w := frameWidth(m); w != 0 {
		t.Fatalf("a's late frame shows under tab b as a %d wide frame", w)
	}
	m.applyBrowserState(twoTabs("a"))
	if w := frameWidth(m); w != 8 {
		t.Fatalf("a's late frame is kept for a, shows %d wide, want 8", w)
	}
}

func TestClosedTabsFrameIsFreed(t *testing.T) {
	m, _ := newLayoutsModel(t)
	name := api.BrowserFrameEvents.Of("orch").Name
	m.applyBrowserState(twoTabs("a"))
	send(m, name, tabPayload(t, "a", 8))
	send(m, name, tabPayload(t, "b", 9))
	st := twoTabs("a")
	st.Tabs = st.Tabs[:1]
	m.applyBrowserState(st)
	if got := len(m.browsers.frames["orch"]); got != 1 {
		t.Fatalf("%d tabs keep a frame after b closed, want 1", got)
	}
	m.applyBrowserState(api.BrowserState{Owner: "orch"})
	if _, kept := m.browsers.frames["orch"]; kept {
		t.Fatal("a closed browser keeps its frames")
	}
}

func TestOnlyTheLatestFrontedTabsKeepAFrame(t *testing.T) {
	m, _ := newLayoutsModel(t)
	name := api.BrowserFrameEvents.Of("orch").Name
	for i := range tabFramesKept + 2 {
		send(m, name, tabPayload(t, string(rune('a'+i)), 8))
	}
	frames := m.browsers.frames["orch"]
	if len(frames) != tabFramesKept {
		t.Fatalf("%d frames kept, want %d", len(frames), tabFramesKept)
	}
	if _, oldest := frames["a"]; oldest {
		t.Fatal("the tab fronted longest ago kept its frame")
	}
}

func TestBrowserWithoutTabListShowsItsNewestFrame(t *testing.T) {
	m, _ := newLayoutsModel(t)
	name := api.BrowserFrameEvents.Of("orch").Name
	m.applyBrowserState(api.BrowserState{Owner: "orch", Open: true})
	send(m, name, tabPayload(t, "a", 8))
	if got := frameWidth(m); got != 8 {
		t.Fatalf("frame width %d, want the one tab's frame", got)
	}
}
