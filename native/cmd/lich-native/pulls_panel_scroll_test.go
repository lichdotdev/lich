package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestPanelOpensAtTheTopOfANewTabOrPullRequest(t *testing.T) {
	var v pullView
	v.init()
	d := backend.PRDetail{Number: 7, URL: "https://github.com/o/r/pull/7"}
	scrolled := func() {
		v.panel.Position.First, v.panel.Position.Offset = 12, 40
	}
	v.panelScroll(d)
	scrolled()
	v.panelScroll(d)
	if v.panel.Position.First != 12 {
		t.Fatalf("the same tab lost its scroll: %+v", v.panel.Position)
	}
	v.showTab(d, tabConversation)
	v.panelScroll(d)
	if v.panel.Position.First != 0 || v.panel.Position.Offset != 0 {
		t.Fatalf("Conversation opened scrolled to the last tab's place: %+v", v.panel.Position)
	}
	scrolled()
	other := backend.PRDetail{Number: 8, URL: "https://github.com/o/r/pull/8"}
	v.panelScroll(other)
	if v.panel.Position.First != 0 {
		t.Fatalf("another pull request opened at the last one's scroll: %+v", v.panel.Position)
	}
}
