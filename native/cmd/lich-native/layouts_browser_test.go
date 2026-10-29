package main

import (
	"reflect"
	"testing"
)

func TestOpenBrowserPutsThePageRightOfItsSession(t *testing.T) {
	var ls layoutTabs
	tab := ls.byID(ls.openBrowser("checkout", "s"))
	if got, want := paneShape(tab.Root), "row(s,browser:s)"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if tab.Focus != "browser:s" {
		t.Fatalf("focus %q, want the page", tab.Focus)
	}
	if got := tab.sessions(); !reflect.DeepEqual(got, []string{"s"}) {
		t.Fatalf("sessions %v, want the session alone", got)
	}
	if owner, ok := tab.browserOf(); !ok || owner != "s" {
		t.Fatalf("browserOf = %q, %v", owner, ok)
	}
	if ls.of("s") != 0 || !ls.locked(0) {
		t.Fatal("the session's tab is not found or not locked")
	}
}

func TestBrowserTabRotatesLikeAnyLayout(t *testing.T) {
	var ls layoutTabs
	tab := ls.byID(ls.openBrowser("checkout", "s"))
	if !tab.rotate("s") {
		t.Fatal("rotate refused the browser tab")
	}
	if got, want := paneShape(tab.Root), "column(s,browser:s)"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestTakingTheSessionDissolvesItsBrowserTabLeavingNoPage(t *testing.T) {
	var ls layoutTabs
	ls.openBrowser("checkout", "s")
	left, ok := ls.take("s")
	if !ok || left != "" || len(ls.tabs) != 0 {
		t.Fatalf("take = %q, %v with %d tabs; want the tab gone and no page left on screen", left, ok, len(ls.tabs))
	}
}

func TestTakingAFocusedSessionBesideItsPageDissolvesTheTab(t *testing.T) {
	var ls layoutTabs
	ls.byID(ls.openBrowser("checkout", "s")).Focus = "s"
	left, ok := ls.take("s")
	if !ok || left != "" || len(ls.tabs) != 0 {
		t.Fatalf("take = %q, %v with %d tabs; want the tab gone and no page left on screen", left, ok, len(ls.tabs))
	}
}

func TestDropBrowserDissolvesOnlyThatTab(t *testing.T) {
	var ls layoutTabs
	ls.open("pair", "a", "b", edgeRight)
	ls.openBrowser("checkout", "s")
	if ls.dropBrowser("a") {
		t.Fatal("dropped a browser a session without one")
	}
	if !ls.dropBrowser("s") || len(ls.tabs) != 1 || ls.locked(0) {
		t.Fatalf("tabs after drop: %+v", ls.tabs)
	}
}

func TestPruneLeavesAPageWhoseSessionLives(t *testing.T) {
	var ls layoutTabs
	ls.openBrowser("checkout", "s")
	if ls.prune(func(id string) bool { return id == "s" }) {
		t.Fatal("prune took a page whose session is live")
	}
	if !ls.prune(func(string) bool { return false }) || len(ls.tabs) != 0 {
		t.Fatal("prune kept the browser tab of a closed session")
	}
}

func TestBrowserTabRoundTrips(t *testing.T) {
	var ls layoutTabs
	ls.openBrowser("checkout", "s")
	raw, err := ls.marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := parseLayouts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.tabs) != 1 || paneShape(back.tabs[0].Root) != "row(s,browser:s)" || back.tabs[0].Focus != "browser:s" {
		t.Fatalf("round trip: %+v", back.tabs)
	}
}

func TestPaneSessionIsThePagesOwner(t *testing.T) {
	if paneSession("browser:s") != "s" || paneSession("s") != "s" {
		t.Fatal("paneSession")
	}
}

func TestParseLayoutsFocusesAPageWhenOnlyPagesAreLeft(t *testing.T) {
	raw := `[{"id":"layout-1","root":{"axis":"row","first":{"session":"browser:a"},"second":{"session":"browser:b"}},"focus":"gone"}]`
	ls, err := parseLayouts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls.tabs) != 1 || ls.tabs[0].Focus != "browser:a" {
		t.Fatalf("tabs = %+v, want the layout focused on its first page", ls.tabs)
	}
}
