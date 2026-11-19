package browser

import (
	"context"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// frontLoading is whether the window's last browser-state shows the front tab
// loading.
func frontLoading(t *testing.T, out *emitted) bool {
	t.Helper()
	states := out.states()
	for _, tab := range states[len(states)-1].Tabs {
		if tab.Front {
			return tab.Loading
		}
	}
	t.Fatal("the window's last state has no front tab")
	return false
}

func TestWindowsOwnPageLoadingShowsOnTheFrontTab(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].onTabs([]api.BrowserTab{{ID: "t0", URL: "https://example.com/a", Front: true}, {ID: "t1", URL: "https://example.com/b"}})
	ctx := context.Background()
	if _, err := s.NativeLoad(ctx, api.BrowserNativeLoadOptions{ID: "s", Loading: true}); err != nil {
		t.Fatal(err)
	}
	if !frontLoading(t, out) {
		t.Fatal("a load the window's page started does not show on the front tab")
	}
	states := out.states()
	if back := states[len(states)-1].Tabs[1]; back.Loading {
		t.Fatalf("a tab behind shows the front page's load: %+v", back)
	}
	if _, err := s.NativeLoad(ctx, api.BrowserNativeLoadOptions{ID: "s", Loading: false}); err != nil {
		t.Fatal(err)
	}
	if frontLoading(t, out) {
		t.Fatal("a load the window's page finished still shows on the front tab")
	}
}

func TestALinkFollowedInTheWindowsOwnPageLoadsInTheAgentsPage(t *testing.T) {
	s, launch, out := viewService(t)
	if _, err := s.Open("s", "https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	agentActs := len(out.named(api.BrowserAgentEvent.Name))
	if _, err := s.NativeLoad(context.Background(), api.BrowserNativeLoadOptions{ID: "s", Loading: true, URL: "https://example.com/b"}); err != nil {
		t.Fatal(err)
	}
	if got := launch.pages[0].navigations; !slices.Contains(got, "https://example.com/b") {
		t.Fatalf("agent's page navigations %q, want the link the person followed", got)
	}
	if got := len(out.named(api.BrowserAgentEvent.Name)); got != agentActs {
		t.Fatalf("the person's link played the agent's cursor: %d agent events, want %d", got, agentActs)
	}
}

func TestTheWindowsOwnPageLandingWhereTheAgentsIsLoadsNothing(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	before := len(launch.pages[0].navigations)
	if _, err := s.NativeLoad(context.Background(), api.BrowserNativeLoadOptions{ID: "s", Loading: true, URL: "https://example.com/a"}); err != nil {
		t.Fatal(err)
	}
	if got := launch.pages[0].navigations; len(got) != before {
		t.Fatalf("agent's page navigations %q, want none past the %d it had", got, before)
	}
}
