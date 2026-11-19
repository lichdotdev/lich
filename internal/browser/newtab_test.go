package browser

import (
	"context"
	"errors"
	"testing"

	"github.com/lichdotdev/lich/api"
)

const testNewTabURL = "http://127.0.0.1:27000/new-tab/s"

type heard struct{ owner, url, title string }

// newTabService is a service whose new-tab page lives at testNewTabURL.
func newTabService(t *testing.T) (*Service, *fakeLauncher, *emitted, *[]heard) {
	t.Helper()
	s, launch, out := viewService(t)
	var visits []heard
	s.SetNewTab(NewTab{
		URL:   func(owner string) (string, error) { return testNewTabURL + owner[1:], nil },
		Is:    func(raw string) bool { return raw == testNewTabURL },
		Visit: func(owner, url, title string) { visits = append(visits, heard{owner, url, title}) },
		Look:  func(api.BrowserNewTabPageOptions) error { return nil },
	})
	return s, launch, out, &visits
}

func TestAttachWithoutABrowserOpensTheNewTabPage(t *testing.T) {
	s, launch, out, _ := newTabService(t)
	h, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	if h.URL != testNewTabURL || launch.pages[0].navigations[0] != testNewTabURL {
		t.Fatalf("opened at %q, navigations %v", h.URL, launch.pages[0].navigations)
	}
	if st := out.states(); len(st) == 0 || !st[len(st)-1].NewTab {
		t.Fatalf("states = %+v, want the new-tab flag", st)
	}
}

func TestOpenVisibleWithoutABrowserOpensTheNewTabPage(t *testing.T) {
	s, _, _, _ := newTabService(t)
	h, err := s.OpenVisible("s")
	if err != nil {
		t.Fatal(err)
	}
	if h.URL != testNewTabURL {
		t.Fatalf("opened at %q", h.URL)
	}
}

func TestTheAgentsOwnOpenKeepsAboutBlank(t *testing.T) {
	s, launch, _, _ := newTabService(t)
	if _, err := s.Open("s", ""); err != nil {
		t.Fatal(err)
	}
	if got := launch.pages[0].navigations[0]; got != "about:blank" {
		t.Fatalf("the agent's open went to %q", got)
	}
}

func TestANewTabOpensOnTheNewTabPage(t *testing.T) {
	s, launch, _, _ := newTabService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: api.TabNew}); err != nil {
		t.Fatal(err)
	}
	page := launch.pages[0]
	if got := page.tabActions; len(got) != 1 || got[0] != "new " {
		t.Fatalf("tab actions = %v", got)
	}
	if n := len(page.navigations); n != 2 || page.navigations[1] != testNewTabURL {
		t.Fatalf("navigations = %v, want the new tab to load the page", page.navigations)
	}
}

func TestSelectingATabLoadsNothing(t *testing.T) {
	s, launch, _, _ := newTabService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: api.TabSelect, Tab: "t0"}); err != nil {
		t.Fatal(err)
	}
	if n := len(launch.pages[0].navigations); n != 1 {
		t.Fatalf("navigations = %v", launch.pages[0].navigations)
	}
}

func TestANewTabWithoutAPageStaysBlank(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.TabNav(api.BrowserTabOptions{ID: "s", Action: api.TabNew}); err != nil {
		t.Fatal(err)
	}
	if got := launch.pages[0].navigations; len(got) != 1 || got[0] != "about:blank" {
		t.Fatalf("navigations = %v", got)
	}
}

func TestAFailingNewTabPageFallsBackToAboutBlank(t *testing.T) {
	s, launch, _ := viewService(t)
	s.SetNewTab(NewTab{URL: func(string) (string, error) { return "", errors.New("no listener") }})
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if got := launch.pages[0].navigations[0]; got != "about:blank" {
		t.Fatalf("opened at %q", got)
	}
}

func TestTheStateFlagsTheNewTabPageAndItsTab(t *testing.T) {
	s, launch, out, _ := newTabService(t)
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].onTabs([]api.BrowserTab{{ID: "a", URL: testNewTabURL, Front: true}, {ID: "b", URL: "https://example.com"}})
	st := out.states()
	last := st[len(st)-1]
	if !last.NewTab || !last.Tabs[0].NewTab || last.Tabs[1].NewTab {
		t.Fatalf("state = %+v", last)
	}
}

func TestEveryPageTheBrowserComesToIsHeard(t *testing.T) {
	s, launch, _, visits := newTabService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].onInfo("https://example.com/docs", "Docs")
	if len(*visits) != 1 || (*visits)[0] != (heard{"s", "https://example.com/docs", "Docs"}) {
		t.Fatalf("visits = %v", *visits)
	}
}

func TestResolveURLRefusesWhatNavigateRefuses(t *testing.T) {
	s, _, _ := viewService(t)
	if got, err := s.ResolveURL("s", "https://example.com/"); err != nil || got != "https://example.com/" {
		t.Fatalf("ResolveURL = %q, %v", got, err)
	}
	for _, raw := range []string{"javascript:alert(1)", "file:///etc/passwd", "data:text/html,x"} {
		if _, err := s.ResolveURL("s", raw); err == nil {
			t.Errorf("%q was resolved", raw)
		}
	}
}

func TestNewTabPageNeedsAHandler(t *testing.T) {
	s, _, _ := viewService(t)
	if _, err := s.NewTabPage(context.Background(), api.BrowserNewTabPageOptions{}); err == nil {
		t.Fatal("no new-tab page, no error")
	}
	s, _, _, _ = newTabService(t)
	if _, err := s.NewTabPage(context.Background(), api.BrowserNewTabPageOptions{}); err != nil {
		t.Fatal(err)
	}
}
