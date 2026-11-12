package main

import "testing"

func TestPaneSessionIsThePagesOwner(t *testing.T) {
	if paneSession("browser:s") != "s" || paneSession("s") != "s" {
		t.Fatal("paneSession")
	}
}

// The contract changed: a page is no pane of a stored layout any more, so
// what an older build stored as a browser tab is dropped on read.
func TestParseLayoutsDropsStoredPagePanes(t *testing.T) {
	raw := `[{"id":"layout-1","root":{"axis":"row","first":{"session":"s"},"second":{"session":"browser:s"}},"focus":"browser:s"},
		{"id":"layout-2","root":{"axis":"row","first":{"session":"a"},"second":{"axis":"row","first":{"session":"b"},"second":{"session":"browser:b"}}},"focus":"browser:b"}]`
	ls, err := parseLayouts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls.tabs) != 1 || paneShape(ls.tabs[0].Root) != "row(a,b)" || ls.tabs[0].Focus != "a" {
		t.Fatalf("tabs = %+v, want the browser tab gone and the other layout without its page", ls.tabs)
	}
}
