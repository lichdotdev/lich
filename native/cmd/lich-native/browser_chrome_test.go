package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/ui"
)

func TestShortURLIsHostAndPath(t *testing.T) {
	cases := map[string]string{
		"https://example.com/":                   "example.com",
		"http://127.0.0.1:5191/":                 "127.0.0.1:5191",
		"https://example.com/docs/intro?q=1#top": "example.com/docs/intro",
		"https://example.com/docs/":              "example.com/docs",
		"file:///tmp/page.html":                  "file:///tmp/page.html",
		"about:blank":                            "",
		"":                                       "",
	}
	for raw, want := range cases {
		if got := shortURL(raw); got != want {
			t.Errorf("shortURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

// chromeRig draws a page pane's header and toolbar for a browser on st, in a
// window that sends nothing: a click shows as a call waiting in the pane's
// queue.
func chromeRig(t *testing.T, st api.BrowserState) (*window, *browserPaneUI, *frameRig) {
	t.Helper()
	u := &window{browserZoom: defaultZoom}
	p := &browserPaneUI{queue: newBrowserQueue(), copied: make(chan string, 1), notes: newBrowserNotes()}
	u.browserPanes = map[string]*browserPaneUI{"s": p}
	head, bar := 32, 36
	rig := newFrameRig(image.Pt(700, head+bar), func(gtx C, th *ui.Theme) {
		u.theme = th
		header := gtx
		header.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, head))
		u.browserHeader(header, p, "s", st, true)
		toolbar := gtx
		toolbar.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, bar))
		defer op.Offset(image.Pt(0, head)).Push(gtx.Ops).Pop()
		u.browserToolbar(toolbar, p, "s", st)
	})
	rig.settle(t)
	return u, p, rig
}

func TestTabsAreAlwaysOnTheHeader(t *testing.T) {
	_, _, rig := chromeRig(t, api.BrowserState{Open: true, Tabs: []api.BrowserTab{{ID: "a", Title: "Alpha", Front: true}}})
	if !rig.shows("Alpha") {
		t.Fatalf("a browser with one tab shows no tab:\n%s", rig.drawn())
	}
	if rig.shows("Browser") {
		t.Fatalf("the header still names the pane:\n%s", rig.drawn())
	}
}

func TestClickingATabSwitchesAndPlusOpensOne(t *testing.T) {
	_, p, rig := chromeRig(t, api.BrowserState{Open: true, Tabs: []api.BrowserTab{
		{ID: "a", Title: "Alpha", Front: true}, {ID: "b", Title: "Beta"},
	}})
	rig.click(t, rig.at(t, "Alpha"))
	if n := len(p.queue.calls); n != 0 {
		t.Fatalf("the tab in front, clicked, sent %d calls, want none", n)
	}
	rig.click(t, rig.at(t, "Beta"))
	if n := len(p.queue.calls); n != 1 {
		t.Fatalf("a tab behind, clicked, sent %d calls, want one select", n)
	}
	plus := p.tabs.newAt.Add(image.Pt(tabsOrigin(layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}), 0))
	rig.click(t, centre(plus))
	if n := len(p.queue.calls); n != 2 {
		t.Fatalf("plus sent %d calls in all, want the select and a new tab", n)
	}
	if !rig.router.Source().Focused(&p.url) {
		t.Fatal("a new tab left the address field without the focus")
	}
}

func TestAddressShowsHostAndPathAndWholeURLWhileEditing(t *testing.T) {
	const page = "https://example.com/docs?q=1"
	_, p, rig := chromeRig(t, api.BrowserState{Open: true, URL: page, Tabs: []api.BrowserTab{{ID: "a", Front: true}}})
	if got := p.url.Text(); got != "example.com/docs" {
		t.Fatalf("the address at rest reads %q, want host and path", got)
	}
	rig.click(t, f32.Pt(360, 50))
	rig.frames(2)
	if got := p.url.Text(); got != page {
		t.Fatalf("the address being edited reads %q, want the whole URL", got)
	}
}

func TestBlankPageShowsTheAddressPlaceholder(t *testing.T) {
	_, _, rig := chromeRig(t, api.BrowserState{Open: true, URL: "about:blank", Tabs: []api.BrowserTab{{ID: "a", Front: true}}})
	if !rig.shows("Search or enter address") {
		t.Fatalf("a blank page's address shows no placeholder:\n%s", rig.drawn())
	}
}

// A loading tab turns a spinner where its mark sits, and its title stays put.
func TestALoadingTabSpinsInItsMarksPlace(t *testing.T) {
	tab := api.BrowserTab{ID: "a", Title: "Alpha", Front: true}
	_, _, loaded := chromeRig(t, api.BrowserState{Open: true, Tabs: []api.BrowserTab{tab}})
	tab.Loading = true
	_, _, loading := chromeRig(t, api.BrowserState{Open: true, Tabs: []api.BrowserTab{tab}})
	still, _ := loaded.frame()
	spun, _ := loading.frame()
	if paintOf(still) == paintOf(spun) {
		t.Fatal("a loading tab draws as a loaded one")
	}
	if a, b := titleBounds(t, loaded), titleBounds(t, loading); a != b {
		t.Fatalf("the title moved while the tab loads: %v, loaded %v", b, a)
	}
	loaded.now, loading.now = loaded.now.Add(250*time.Millisecond), loading.now.Add(250*time.Millisecond)
	if again, _ := loaded.frame(); paintOf(again) != paintOf(still) {
		t.Fatal("a loaded tab's mark moves")
	}
	if again, _ := loading.frame(); paintOf(again) == paintOf(spun) {
		t.Fatal("a loading tab's spinner does not turn")
	}
}

func paintOf(drawn string) string { return strings.SplitN(drawn, "\n", 2)[0] }

func titleBounds(t *testing.T, rig *frameRig) image.Rectangle {
	t.Helper()
	for _, n := range rig.nodes() {
		if n.Desc.Label == "Alpha" {
			return n.Desc.Bounds
		}
	}
	t.Fatalf("no tab titled Alpha:\n%s", rig.drawn())
	return image.Rectangle{}
}
