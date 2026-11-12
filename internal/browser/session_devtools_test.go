package browser

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// inspecting is a browser whose page t0 has its DevTools shown in the pane,
// beside a second tab t1.
func inspecting(t *testing.T) (*Service, *fakeLauncher, *emitted) {
	t.Helper()
	s, launch, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].onTabs([]api.BrowserTab{
		{ID: "t0", URL: "https://example.com/a"},
		{ID: "dt0", DevTools: true, Front: true},
		{ID: "t1", URL: "https://example.com/b"},
	})
	return s, launch, out
}

func TestTheAgentNeverListsADevToolsTab(t *testing.T) {
	s, _, out := inspecting(t)
	got, err := s.ListTabs(context.Background(), api.SessionIDOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	want := []api.BrowserTab{{ID: "t0", URL: "https://example.com/a", Front: true}, {ID: "t1", URL: "https://example.com/b"}}
	if !slices.Equal(got, want) {
		t.Fatalf("agent's tabs = %+v, want %+v", got, want)
	}
	states := out.states()
	if pane := states[len(states)-1].Tabs; len(pane) != 3 || !pane[1].DevTools || !pane[1].Front {
		t.Fatalf("window's tabs = %+v, want the DevTools tab in front", pane)
	}
}

func TestTheAgentCannotSelectOrCloseADevToolsTab(t *testing.T) {
	s, launch, _ := inspecting(t)
	for _, action := range []string{api.TabSelect, api.TabClose} {
		_, err := s.ActOnTab(context.Background(), api.BrowserTabOptions{ID: "s", Action: action, Tab: "dt0"})
		if !errors.Is(err, errNoTab) {
			t.Fatalf("%s on a DevTools tab = %v, want no such tab", action, err)
		}
	}
	if got := launch.pages[0].tabActions; len(got) != 0 {
		t.Fatalf("page tab actions %q, want none", got)
	}
}

func TestTheAgentCannotOpenDevTools(t *testing.T) {
	s, launch, _ := inspecting(t)
	if err := s.TabAct(api.BrowserTabOptions{ID: "s", Action: "devtools"}); err == nil {
		t.Fatal("the agent's tab action opened DevTools")
	}
	if got := launch.pages[0].tabActions; len(got) != 0 {
		t.Fatalf("page tab actions %q, want none", got)
	}
}

func TestTheAgentSelectingItsOwnTabLeavesThePaneOnDevTools(t *testing.T) {
	s, launch, out := inspecting(t)
	if err := s.TabAct(api.BrowserTabOptions{ID: "s", Action: api.TabSelect, Tab: "t0"}); err != nil {
		t.Fatal(err)
	}
	if got := launch.pages[0].tabActions; len(got) != 0 {
		t.Fatalf("page tab actions %q, want the page left as it is", got)
	}
	acts := out.named(api.BrowserAgentEvent.Name)
	if last := acts[len(acts)-1].(api.BrowserAgentAct); last.Action != "tab-select" || last.Tab != "t0" {
		t.Fatalf("last agent action %+v, want the selection played", last)
	}
}

func TestTheAgentSelectingAnotherTabStillSelectsIt(t *testing.T) {
	s, launch, _ := inspecting(t)
	if err := s.TabAct(api.BrowserTabOptions{ID: "s", Action: api.TabSelect, Tab: "t1"}); err != nil {
		t.Fatal(err)
	}
	if got := launch.pages[0].tabActions; !slices.Equal(got, []string{"select t1"}) {
		t.Fatalf("page tab actions %q, want t1 selected", got)
	}
}

func TestThePaneOpensDevToolsAndActsOnItsTab(t *testing.T) {
	s, launch, out := inspecting(t)
	before := len(out.named(api.BrowserAgentEvent.Name))
	if _, err := s.DevToolsView(context.Background(), api.SessionIDOptions{ID: "s"}); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{api.TabSelect, api.TabClose} {
		if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: action, Tab: "dt0"}); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := launch.pages[0].tabActions, []string{"devtools ", "select dt0", "close dt0"}; !slices.Equal(got, want) {
		t.Fatalf("page tab actions %q, want %q", got, want)
	}
	if after := len(out.named(api.BrowserAgentEvent.Name)); after != before {
		t.Fatal("the person's DevTools played the agent's cursor")
	}
}

func TestDevToolsIsRefusedForABrowserInItsOwnWindow(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.OpenVisible("s"); err != nil {
		t.Fatal(err)
	}
	if err := s.DevTools("s"); !errors.Is(err, errInWindow) {
		t.Fatalf("DevTools = %v, want the browser in its own window", err)
	}
	if got := launch.pages[0].tabActions; len(got) != 0 {
		t.Fatalf("page tab actions %q, want none", got)
	}
}

func TestAPausedPageFailsTheAgentsReadNamingThePause(t *testing.T) {
	s, launch, _ := inspecting(t)
	launch.pages[0].infoErr = errPaused
	if _, err := s.Read(context.Background(), api.BrowserInfoOptions{ID: "s"}); !errors.Is(err, errPaused) {
		t.Fatalf("Read = %v, want the pause named", err)
	}
	if !launch.pages[0].Alive() {
		t.Fatal("a paused page closed the browser")
	}
}

func TestAPausedPageTakesThePanesInputWithoutAnError(t *testing.T) {
	s, launch, _ := inspecting(t)
	launch.pages[0].mouseErr = errPaused
	if _, err := s.MouseInput(context.Background(), api.BrowserMouseOptions{ID: "s", Type: "move"}); err != nil {
		t.Fatalf("pane input on a paused page = %v, want none", err)
	}
}
