package browser

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestAgentTabActionsTellTheWindowWhichTab(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	for _, opts := range []api.BrowserTabOptions{
		{ID: "s", Action: api.TabNew},
		{ID: "s", Action: api.TabSelect, Tab: "t1"},
		{ID: "s", Action: api.TabClose, Tab: "t2"},
	} {
		if err := s.TabAct(opts); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := launch.pages[0].tabActions, []string{"new ", "select t1", "close t2"}; !slices.Equal(got, want) {
		t.Fatalf("page tab actions %q, want %q", got, want)
	}
	var got []string
	for _, d := range out.named(api.BrowserAgentEvent.Name) {
		a := d.(api.BrowserAgentAct)
		got = append(got, a.Action+" "+a.Tab)
	}
	want := []string{"tab-new ", "tab-select t1", "tab-close t2"}
	if !slices.Equal(got, want) {
		t.Fatalf("agent actions %q, want %q", got, want)
	}
}

func TestAgentNewTabWithAURLLoadsItThere(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	if err := s.TabAct(api.BrowserTabOptions{ID: "s", Action: api.TabNew, URL: "https://example.com/b"}); err != nil {
		t.Fatal(err)
	}
	if got := launch.pages[0].tabActions; !slices.Equal(got, []string{"new "}) {
		t.Fatalf("page tab actions %q, want one new tab", got)
	}
	acts := out.named(api.BrowserAgentEvent.Name)
	last := acts[len(acts)-1].(api.BrowserAgentAct)
	if last.Action != "navigate" || last.URL != "https://example.com/b" {
		t.Fatalf("last agent action %+v, want the new tab's URL typed in", last)
	}
}

func TestWindowTabActionsPlayNoAgentCursor(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	before := len(out.named(api.BrowserAgentEvent.Name))
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: api.TabSelect, Tab: "t1"}); err != nil {
		t.Fatal(err)
	}
	if got := launch.pages[0].tabActions; !slices.Equal(got, []string{"select t1"}) {
		t.Fatalf("page tab actions %q, want the selection", got)
	}
	if after := len(out.named(api.BrowserAgentEvent.Name)); after != before {
		t.Fatalf("the window's own tab action told the window the agent acted (%d events, want %d)", after, before)
	}
}

func TestTabActionRefusesAnUnknownAction(t *testing.T) {
	s, _, _ := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: "pin"}); err == nil {
		t.Fatal("the window's tab action accepted an unknown action")
	}
	if err := s.TabAct(api.BrowserTabOptions{ID: "s", Action: "pin"}); err == nil {
		t.Fatal("the agent's tab action accepted an unknown action")
	}
}

func TestAgentClosingTheLastTabSaysWhy(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].tabErr = errLastTab
	_, err := s.ActOnTab(context.Background(), api.BrowserTabOptions{ID: "s", Action: api.TabClose, Tab: "t0"})
	if !errors.Is(err, errLastTab) {
		t.Fatalf("err = %v, want the last tab refused", err)
	}
}

func TestPageTabsReachTheWindowAndTheAgent(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	tabs := []api.BrowserTab{{ID: "t0", URL: "https://example.com/a"}, {ID: "t1", URL: "https://example.com/b", Front: true}}
	launch.pages[0].onTabs(tabs)
	states := out.states()
	if got := states[len(states)-1].Tabs; !slices.Equal(got, tabs) {
		t.Fatalf("window's tabs = %+v, want %+v", got, tabs)
	}
	got, err := s.ListTabs(context.Background(), api.SessionIDOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, tabs) {
		t.Fatalf("agent's tabs = %+v, want %+v", got, tabs)
	}
}
