package main

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestDevToolsOfTheFrontTab(t *testing.T) {
	for _, c := range []struct {
		name        string
		tabs        []api.BrowserTab
		open, shown bool
	}{
		{"one page", []api.BrowserTab{{ID: "a", Front: true}}, false, false},
		{"page in front with its DevTools", []api.BrowserTab{{ID: "a", Front: true}, {ID: "d", DevTools: true}}, true, false},
		{"DevTools in front", []api.BrowserTab{{ID: "a"}, {ID: "d", DevTools: true, Front: true}}, true, true},
		{"DevTools of another tab", []api.BrowserTab{{ID: "a"}, {ID: "d", DevTools: true}, {ID: "b", Front: true}}, false, false},
		{"no tabs listed yet", nil, false, false},
	} {
		if open, shown := devtoolsOf(c.tabs); open != c.open || shown != c.shown {
			t.Errorf("%s: devtoolsOf = %v, %v; want %v, %v", c.name, open, shown, c.open, c.shown)
		}
	}
}
