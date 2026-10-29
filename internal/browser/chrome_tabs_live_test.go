package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/api"
)

// waitTabs waits for the page to report tabs that ok accepts.
func waitTabs(t *testing.T, tabs <-chan []api.BrowserTab, ok func([]api.BrowserTab) bool) []api.BrowserTab {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case got := <-tabs:
			if ok(got) {
				return got
			}
		case <-deadline:
			t.Fatal("the page never reported the tabs expected")
		}
	}
}

func frontOf(tabs []api.BrowserTab) api.BrowserTab {
	for _, t := range tabs {
		if t.Front {
			return t
		}
	}
	return api.BrowserTab{}
}

func TestChromeTabsOpenSwitchAndCloseKeepingEachPage(t *testing.T) {
	p, srv := liveChrome(t)
	tabs := make(chan []api.BrowserTab, 32)
	p.Listen(func(string) {}, func(string, string) {}, func(string, api.BrowserDownload) {}, func(string) {},
		func(*api.BrowserDialog) {}, func(got []api.BrowserTab) { tabs <- got })
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL+"/"); err != nil {
		t.Fatal(err)
	}
	if err := p.Screencast(ctx, Viewport{W: 400, H: 300, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if err := p.run(ctx, chromedp.Evaluate(`window.kept = "first"`, nil)); err != nil {
		t.Fatal(err)
	}
	first := p.tabs.frontTab().id
	if err := p.NewTab(ctx); err != nil {
		t.Fatal(err)
	}
	two := waitTabs(t, tabs, func(got []api.BrowserTab) bool { return len(got) == 2 && frontOf(got).ID != string(first) })
	second := frontOf(two).ID
	if err := p.SelectTab(ctx, string(first)); err != nil {
		t.Fatal(err)
	}
	var kept string
	if err := p.run(ctx, chromedp.Evaluate(`window.kept`, &kept)); err != nil {
		t.Fatal(err)
	}
	if kept != "first" {
		t.Fatalf("the first tab came back as %q, its script's state gone", kept)
	}
	if err := p.CloseTab(ctx, string(first)); err != nil {
		t.Fatal(err)
	}
	one := waitTabs(t, tabs, func(got []api.BrowserTab) bool { return len(got) == 1 })
	if !one[0].Front || one[0].ID != second {
		t.Fatalf("tabs after closing the first = %+v, want the second alone in front", one)
	}
	quick, stop := context.WithTimeout(ctx, inputWait)
	defer stop()
	if err := p.Mouse(quick, api.BrowserMouseOptions{Type: "move", X: 50, Y: 50}); err != nil {
		t.Fatalf("the tab left after the first closed takes no input: %v", err)
	}
	if err := p.CloseTab(ctx, second); !errors.Is(err, errLastTab) {
		t.Fatalf("closing the last tab = %v, want it refused", err)
	}
	if err := p.SelectTab(ctx, "gone"); !errors.Is(err, errNoTab) {
		t.Fatalf("selecting an unknown tab = %v, want it refused", err)
	}
}

func TestChromeNewTabReportsThePageTitleOnceLoaded(t *testing.T) {
	p, srv := liveChrome(t)
	tabs := make(chan []api.BrowserTab, 32)
	p.Listen(func(string) {}, func(string, string) {}, func(string, api.BrowserDownload) {}, func(string) {},
		func(*api.BrowserDialog) {}, func(got []api.BrowserTab) { tabs <- got })
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.NewTab(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Navigate(ctx, srv.URL+"/"); err != nil {
		t.Fatal(err)
	}
	waitTabs(t, tabs, func(got []api.BrowserTab) bool { return frontOf(got).Title == "Probe" })
}
