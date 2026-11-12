package main

import (
	"image"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func TestBrowserGuideWaitsOnTheActiveSessionsBrowser(t *testing.T) {
	u := &window{model: newModel(nil, func() {}), browserPanes: map[string]*browserPaneUI{}}
	u.model.projects = []backend.Project{{ID: "p", Sessions: []backend.Session{{ID: "a"}, {ID: "b"}}}}
	u.model.activeProject, u.model.active = "p", "a"

	if u.tourBrowserOpen() || u.tourBrowserPicking() || u.tourBrowserNoted() {
		t.Fatal("a session with no browser and no pane counts as done")
	}
	u.model.browsers.states["b"] = api.BrowserState{Open: true}
	u.browserPanes["b"] = &browserPaneUI{notes: browserNotes{picking: true, list: []*browserNote{{}}}}
	if u.tourBrowserOpen() || u.tourBrowserPicking() || u.tourBrowserNoted() {
		t.Fatal("another session's browser moved the guide on")
	}
	u.model.browsers.states["a"] = api.BrowserState{Open: true}
	if !u.tourBrowserOpen() {
		t.Fatal("a browser opened in its own window does not count")
	}
	u.browserPanes["a"] = &browserPaneUI{notes: browserNotes{picking: true}}
	if !u.tourBrowserPicking() || u.tourBrowserNoted() {
		t.Fatal("picking with no note yet: want the pick step done, the note step waiting")
	}
	u.browserPanes["a"].notes = browserNotes{list: []*browserNote{{}}}
	if !u.tourBrowserPicking() || !u.tourBrowserNoted() {
		t.Fatal("a note left, picking over: want both steps done")
	}
}

func TestBrowserGuideFollowsTheShownCardsMenu(t *testing.T) {
	u := &window{model: newModel(nil, func() {}), browserPanes: map[string]*browserPaneUI{}, sidebar: newSidebar(nil)}
	u.model.projects = []backend.Project{{ID: "p", Sessions: []backend.Session{{ID: "a"}, {ID: "b"}}}}
	u.model.activeProject, u.model.active = "p", "a"

	other := new(ui.MenuState)
	other.OpenAt(image.Point{})
	u.sidebar.menus["b"] = other
	if u.tourCardMenuOpen() {
		t.Fatal("another card's menu opened the shown session's")
	}
	more := new(ui.MenuState)
	more.OpenAt(image.Point{})
	u.sidebar.moreMenus["a"] = more
	if !u.tourCardMenuOpen() {
		t.Fatal("the corner menu of the shown card does not count")
	}

}

func TestTourBrowserSpotMarksOnlyTheBrowserEntry(t *testing.T) {
	var act sidebarAction
	entries := cardMenu(&sessionView{Session: backend.Session{ID: "a"}}, nil, &act, func() {}, func() {})
	tourBrowserSpot(entries, nil)
	spots := 0
	for _, e := range entries {
		if e.Spot != nil {
			spots++
			if e.Label != "Browser" {
				t.Fatalf("spot on %q", e.Label)
			}
		}
	}
	if spots != 1 {
		t.Fatalf("spots = %d, want 1", spots)
	}
}
