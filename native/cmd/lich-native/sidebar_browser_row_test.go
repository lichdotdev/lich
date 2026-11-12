package main

import (
	"slices"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

func browsing(sv sessionView) sessionView {
	sv.browser = true
	return sv
}

func TestABrowserHangsFirstUnderItsSession(t *testing.T) {
	sessions := []sessionView{
		browsing(treeSession("mother", "", "/repo")),
		treeSession("child", "mother", "/repo"),
		browsing(treeSession("solo", "", "/repo")),
	}
	rows := sessionRows(sessions, "", func(string) bool { return false })
	want := []string{"0:mother", "1:__browser__:mother", "1:child.", "0:solo.", "1:__browser__:solo."}
	if got := rowShape(rows); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if rows[1].kind != rowBrowser || rows[1].sv.ID != "mother" {
		t.Fatalf("row %+v, want the mother's browser", rows[1])
	}
	if !rows[3].stub || rows[3].kids != 0 {
		t.Errorf("solo stub %v kids %d: its line runs down to its browser, which is no child", rows[3].stub, rows[3].kids)
	}
}

func TestAFoldedMotherHidesHerBrowserAndAStaleFoldDoesNot(t *testing.T) {
	sessions := []sessionView{
		browsing(treeSession("mother", "", "/repo")),
		treeSession("child", "mother", "/repo"),
		browsing(treeSession("solo", "", "/repo")),
	}
	rows := sessionRows(sessions, "", func(string) bool { return true })
	want := []string{"0:mother", "0:solo.", "1:__browser__:solo."}
	if got := rowShape(rows); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v: a fold with nothing under it hides no browser", got, want)
	}
}

func TestACarriedCardLeavesItsBrowserOutOfTheList(t *testing.T) {
	b := blockView{sessions: []sessionView{browsing(treeSession("a", "", "/repo")), browsing(treeSession("b", "", "/repo"))}}
	rows := blockRows(carriedFold(b, "a"), b.sessions)
	want := []string{"0:a", "0:b.", "1:__browser__:b."}
	if got := rowShape(rows); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if rows[0].stub {
		t.Fatal("the carried card keeps a line down to a browser the list no longer holds")
	}
}

func TestTheRailLeavesBrowsersOut(t *testing.T) {
	b := blockView{sessions: []sessionView{browsing(treeSession("a", "", "/repo"))}}
	if items := railRows(b); len(items) != 1 || items[0].kind != railCard {
		t.Fatalf("rail items %+v, want the session alone", items)
	}
}

func TestBrowserRowNamesThePageAndWhereItIs(t *testing.T) {
	cases := []struct {
		web          webView
		label, state string
	}{
		{webView{url: "http://localhost:5173/login", shown: true}, "localhost:5173", ""},
		{webView{url: "about:blank"}, "Browser", "Minimized"},
		{webView{url: "https://example.com/", headed: true}, "example.com", "In its own window"},
	}
	for _, c := range cases {
		if got := c.web.label(); got != c.label {
			t.Errorf("label of %+v = %q, want %q", c.web, got, c.label)
		}
		if got := c.web.state(); got != c.state {
			t.Errorf("state of %+v = %q, want %q", c.web, got, c.state)
		}
	}
}

func TestBrowserMenuOffersWhatTheBrowsersPlaceAllows(t *testing.T) {
	labels := func(w webView) []string {
		var out []string
		for _, e := range browserMenu("s", w, new(sidebarAction)) {
			if e.Kind != 0 {
				continue
			}
			out = append(out, e.Label)
		}
		return out
	}
	if got, want := labels(webView{shown: true}), []string{"Minimize", "Open in a window", "Close the browser"}; !slices.Equal(got, want) {
		t.Errorf("on screen: %v, want %v", got, want)
	}
	if got, want := labels(webView{}), []string{"Show", "Open in a window", "Close the browser"}; !slices.Equal(got, want) {
		t.Errorf("minimized: %v, want %v", got, want)
	}
	if got, want := labels(webView{headed: true}), []string{"Show", "Close the browser"}; !slices.Equal(got, want) {
		t.Errorf("in a window: %v, want %v", got, want)
	}
	var act sidebarAction
	menu := browserMenu("s", webView{shown: true}, &act)
	menu[0].OnClick()
	menu[len(menu)-1].OnClick()
	if act.browserMinimize != "s" || act.browserClose != "s" {
		t.Fatalf("act %+v, want the session minimized and closed", act)
	}
}

func TestASessionsViewSaysWhereItsBrowserIs(t *testing.T) {
	m, _ := newLayoutsModel(t)
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: true, URL: "http://localhost:3000/", Title: "App"})
	m.showPage(t.Context(), "orch")
	m.mu.Lock()
	sv := m.viewOf(m.projects[0], m.projects[0].Sessions[0], time.Now())
	m.mu.Unlock()
	if want := (webView{url: "http://localhost:3000/", title: "App", shown: true, focused: true}); !sv.browser || sv.web != want {
		t.Fatalf("view %+v browser %v, want %+v", sv.web, sv.browser, want)
	}
}
