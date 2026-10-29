package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
)

func TestTabLabelFallsBackFromTitleToAddressToBlank(t *testing.T) {
	cases := []struct {
		tab  api.BrowserTab
		want string
	}{
		{api.BrowserTab{Title: "Lotes", URL: "https://erp.example/lotes"}, "Lotes"},
		{api.BrowserTab{URL: "https://erp.example/lotes"}, displayURL("https://erp.example/lotes")},
		{api.BrowserTab{URL: "about:blank"}, "New tab"},
	}
	for _, c := range cases {
		if got := tabLabel(c.tab); got != c.want {
			t.Errorf("tabLabel(%+v) = %q, want %q", c.tab, got, c.want)
		}
	}
}

func TestAgentCursorQueuesTabActions(t *testing.T) {
	var c agentCursor
	markSeq++
	c.take([]browserMark{{seq: markSeq, at: time.Now(), action: "tab-select", tab: "T1"}})
	if len(c.pending) != 1 {
		t.Fatalf("pending = %+v, want the tab action queued", c.pending)
	}
}

func TestTabPointLandsOnWhatTheTabActionPressed(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	p := &browserPaneUI{}
	p.tabs.widgets("T1")
	p.tabs.at["T1"] = image.Rect(0, 4, 100, 28)
	p.tabs.closeAt["T1"] = image.Rect(80, 8, 96, 24)
	p.tabs.newAt = image.Rect(104, 4, 128, 28)
	origin := float32(tabsOrigin(gtx))
	cases := map[string]f32.Point{
		"tab-select": f32.Pt(origin+50, 16),
		"tab-close":  f32.Pt(origin+88, 16),
		"tab-new":    f32.Pt(origin+116, 16),
	}
	for action, want := range cases {
		if got := tabPoint(gtx, p, browserMark{action: action, tab: "T1"}, 32); got != want {
			t.Errorf("%s lands at %v, want %v", action, got, want)
		}
	}
	if got := tabPoint(gtx, p, browserMark{action: "tab-select", tab: "gone"}, 32); got != f32.Pt(origin, 16) {
		t.Errorf("a tab never drawn lands at %v, want the start of the tabs", got)
	}
}
