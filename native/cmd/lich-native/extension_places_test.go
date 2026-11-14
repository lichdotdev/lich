package main

import (
	"image"
	"slices"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	issueKey = extensionKey{"tracker", "issue"}
	boardKey = extensionKey{"tracker", "board"}
)

// tracker is an extension with a surface in each place.
func tracker(status string) backend.Extension {
	return backend.Extension{ID: "tracker", Name: "Tracker", Status: status, Surfaces: []backend.ExtensionSurface{
		{ID: "main", Title: "Main"},
		{ID: "issue", Title: "Issue", Icon: "ticket", Place: string(extension.PlaceDock)},
		{ID: "board", Title: "Board", Icon: "kanban", Place: string(extension.PlacePanel)},
	}}
}

func trackerStore(status string) *extensionStore {
	return &extensionStore{list: []backend.Extension{tracker(status), {ID: "notes", Status: api.ExtensionRunning, Surfaces: []backend.ExtensionSurface{{ID: "main"}}}}}
}

func TestPlacedSurfacesAreTheDockAndPanelOnesOfExtensionsThatCanRun(t *testing.T) {
	list := []backend.Extension{tracker(api.ExtensionRunning), {ID: "broken", Status: api.ExtensionInvalid, Surfaces: []backend.ExtensionSurface{{ID: "x", Place: "dock"}}}}
	got := placedSurfaces(list, nil)
	if len(got) != 2 || got[0].key != issueKey || got[1].key != boardKey {
		t.Fatalf("placed = %+v, want issue then board", got)
	}
	if got[0].label() != "Issue · Tracker" || got[0].glyph == nil {
		t.Errorf("issue reads %q with glyph %v", got[0].label(), got[0].glyph)
	}
	if dock := placedIn(got, extension.PlaceDock); len(dock) != 1 || dock[0].key != issueKey {
		t.Errorf("dock surfaces = %+v", dock)
	}
}

func TestTheTopBarOffersOnlyFullScreenSurfaces(t *testing.T) {
	var keys []extensionKey
	for _, b := range surfaceButtons([]backend.Extension{tracker(api.ExtensionRunning)}) {
		keys = append(keys, b.key)
	}
	if !slices.Equal(keys, []extensionKey{{"tracker", "main"}}) {
		t.Errorf("top bar offers %v, want only the full screen", keys)
	}
}

func TestExtensionItemIDsNameTheirSurface(t *testing.T) {
	id := extensionItemID(extensionKey{"tracker", "a/b"})
	if k, ok := extensionItemKey(id); !ok || k != (extensionKey{"tracker", "a/b"}) {
		t.Errorf("%q reads back as %v %v", id, k, ok)
	}
	for _, bad := range []string{"checkout", "ext:", "ext:tracker", "ext:/issue", "ext:tracker/"} {
		if _, ok := extensionItemKey(bad); ok {
			t.Errorf("%q read as an extension item", bad)
		}
	}
}

func TestAFooterTextHoldsForTheCheckoutsTheExtensionWasToldOf(t *testing.T) {
	s := trackerStore(api.ExtensionRunning)
	s.setFooter("tracker", extension.FooterData{Surface: "issue", Path: "/a", Text: "never told"})
	if got := s.footer(issueKey, "/a"); got != "" {
		t.Errorf("a text for an unknown checkout was kept: %q", got)
	}
	s.sawPath("/a")
	s.setFooter("tracker", extension.FooterData{Surface: "issue", Path: "/a", Text: "LICH-1"})
	s.setFooter("tracker", extension.FooterData{Surface: "issue", Text: "every"})
	if got := s.footer(issueKey, "/a"); got != "LICH-1" {
		t.Errorf("checkout /a reads %q", got)
	}
	if got := s.footer(issueKey, "/b"); got != "every" {
		t.Errorf("a checkout without its own reads %q, want the one for every checkout", got)
	}
	s.setFooter("tracker", extension.FooterData{Surface: "issue", Path: "/a"})
	if got := s.footer(issueKey, "/a"); got != "every" {
		t.Errorf("a cleared text reads %q", got)
	}
	s.setStatus(tracker(api.ExtensionFailed))
	if got := s.footer(issueKey, "/b"); got != "" {
		t.Errorf("a failed extension's text stayed: %q", got)
	}
}

func TestAFooterMessageLandsInTheStore(t *testing.T) {
	m := newTestModel(nil)
	m.extensions = *trackerStore(api.ExtensionRunning)
	m.extensions.sawPath("/a")
	var asked []string
	m.extensionMessage = func(id string, msg extension.Message) { asked = append(asked, msg.Kind) }
	m.extensionEvent(messageEv(t, "tracker", extension.Footer.New(extension.FooterData{Surface: "issue", Path: "/a", Text: "LICH-1"})))
	if got := m.extensions.footer(issueKey, "/a"); got != "LICH-1" || len(asked) != 0 {
		t.Errorf("store reads %q and the window was asked %v", got, asked)
	}
}

func TestTheContextReachesRunningExtensionsWithAPlacedSurfaceOncePerChange(t *testing.T) {
	var log sentLog
	store := trackerStore(api.ExtensionRunning)
	store.list = append(store.list, backend.Extension{ID: "idle", Status: api.ExtensionFailed, Surfaces: []backend.ExtensionSurface{{ID: "d", Place: "dock"}}})
	s := newExtensionScreen(store, nil, log.send)
	s.follow(extension.ContextData{})
	s.follow(extension.ContextData{Session: "s1", Path: "/a"})
	s.follow(extension.ContextData{Session: "s1", Path: "/a"})
	s.follow(extension.ContextData{Session: "s2", Path: "/b"})
	want := []string{"tracker context /a", "tracker context /b"}
	if !slices.Equal(log.sent, want) {
		t.Errorf("sent %v, want %v", log.sent, want)
	}
	if !store.paths["/a"] || !store.paths["/b"] || s.activePath() != "/b" {
		t.Errorf("paths %v, active %q", store.paths, s.activePath())
	}
}

func TestAnExtensionThatStartsHearsTheContextThenItsPlacedSurfaces(t *testing.T) {
	var log sentLog
	s := newExtensionScreen(trackerStore(api.ExtensionRunning), nil, log.send)
	s.follow(extension.ContextData{Path: "/a"})
	s.drawn[placeDock] = issueKey
	s.settle()
	log.sent = nil
	s.started("tracker")
	s.started("notes")
	want := []string{"tracker context /a", "tracker surface.opened issue"}
	if !slices.Equal(log.sent, want) {
		t.Errorf("sent %v, want %v", log.sent, want)
	}
}

func TestAPlaceTellsTheSurfaceItTookAndTheOneItLeft(t *testing.T) {
	var log sentLog
	s := newExtensionScreen(trackerStore(api.ExtensionRunning), nil, log.send)
	other := extensionKey{"tracker", "other"}
	for _, frame := range []extensionKey{issueKey, issueKey, other, {}} {
		s.drawn[placeDock] = frame
		s.settle()
	}
	want := []string{"tracker surface.opened issue", "tracker surface.closed issue", "tracker surface.opened other", "tracker surface.closed other"}
	if !slices.Equal(log.sent, want) {
		t.Errorf("sent %v, want %v", log.sent, want)
	}
}

// placedFrame lays surface k out in the dock once, at 320 by 240.
func placedFrame(s *extensionScreen, k extensionKey) {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(320, 240))}
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	s.placed(gtx, th, placeDock, k)
}

func TestATreeForAnotherCheckoutWaitsForTheActiveOnesOwn(t *testing.T) {
	store := trackerStore(api.ExtensionRunning)
	s := newExtensionScreen(store, nil, func(string, extension.Message) {})
	s.follow(extension.ContextData{Path: "/b"})
	store.setRender("tracker", extension.RenderData{Surface: "issue", Path: "/a", Root: extension.Box.New(extension.BoxProps{})})
	placedFrame(s, issueKey)
	if _, drawn := s.trees[issueKey]; drawn {
		t.Fatal("the tree drawn for /a was drawn while /b is active")
	}
	store.setRender("tracker", extension.RenderData{Surface: "issue", Path: "/b", Root: extension.Box.New(extension.BoxProps{})})
	placedFrame(s, issueKey)
	if _, drawn := s.trees[issueKey]; !drawn {
		t.Fatal("the tree drawn for the active checkout was not drawn")
	}
}

func TestTheFooterAdoptsAnExtensionItemOnceAtTheEndOfTheLeft(t *testing.T) {
	issue, board := extensionItemID(issueKey), extensionItemID(boardKey)
	l := footerLayout{Left: []string{"attach"}, Right: []string{board}}
	next, fresh := l.adopt([]string{issue, board})
	if !fresh || !slices.Equal(next.Left, []string{"attach", issue}) || !slices.Equal(next.Right, []string{board}) {
		t.Fatalf("adopted into %+v", next)
	}
	hidden, _ := moveFooterItem(next, issue, footerAvailable)
	if again, fresh := hidden.adopt([]string{issue, board}); fresh || again.has(issue) {
		t.Errorf("an item moved out came back: %+v", again)
	}
}

func TestASavedLayoutKeepsExtensionItemsAndWhatItSaw(t *testing.T) {
	issue := extensionItemID(issueKey)
	l, ok := parseFooterLayout(`{"left":["attach","` + issue + `"],"right":[],"seen":["` + issue + `","checkout","ext:x"]}`)
	if !ok || !slices.Equal(l.Left, []string{"attach", issue}) || !slices.Equal(l.Seen, []string{issue}) {
		t.Errorf("parsed %+v", l)
	}
}

func TestTheEditorListsOnlyTheExtensionItemsDrawnNow(t *testing.T) {
	in := footerInput{extItems: []footerExtItem{{placedSurface: placedSurface{key: issueKey, title: "Issue", name: "Tracker"}, id: extensionItemID(issueKey)}}}
	gone := extensionItemID(extensionKey{"gone", "x"})
	if got := drawnItems([]string{"attach", gone, extensionItemID(issueKey)}, in); !slices.Equal(got, []string{"attach", extensionItemID(issueKey)}) {
		t.Errorf("drawn items %v", got)
	}
	avail := availableItems(footerLayout{}, in)
	if avail[len(avail)-1] != extensionItemID(issueKey) {
		t.Errorf("available %v, want the extension item last", avail)
	}
	if s, ok := footerSlotOf(gone, in); ok {
		t.Errorf("an item no extension draws got a slot: %+v", s)
	}
}

func TestTheDockTabsCountLichsThenTheExtensions(t *testing.T) {
	placed := []placedSurface{{key: issueKey}, {key: extensionKey{"notes", "d"}}}
	d := &dock{}
	d.pickTab(dockBuiltinTabs+1, placed)
	if d.tab != dockExtension || d.ext != placed[1].key || d.selectedTab(placed) != dockBuiltinTabs+1 {
		t.Errorf("picked %v %v, selected %d", d.tab, d.ext, d.selectedTab(placed))
	}
	d.pickTab(1, placed)
	if d.tab != dockReview || d.selectedTab(placed) != 1 {
		t.Errorf("picked %v", d.tab)
	}
	d.toggleExtension(issueKey)
	if !d.showsExtension(issueKey) {
		t.Error("the item did not open its tab")
	}
	d.toggleExtension(issueKey)
	if d.tab != dockClosed {
		t.Error("the item did not close its own tab")
	}
}

func TestTheDockIsNeverNarrowerThanItsTabs(t *testing.T) {
	d := &dock{widthRem: dockMinRem, minRem: dockMinRem}
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 2, PxPerSp: 2}}
	d.fitTabs(gtx, 2*remDp*30)
	if d.minRem != 30 || d.width(gtx) != gtx.Dp(30*remDp) {
		t.Errorf("min %v, width %d", d.minRem, d.width(gtx))
	}
	d.fitTabs(gtx, 10)
	if d.minRem != dockMinRem {
		t.Errorf("tabs that fit left the minimum at %v", d.minRem)
	}
}

func TestOnePanelLeavesTheOneJustOpened(t *testing.T) {
	g, p := &gitPanel{open: true}, &extensionPanel{open: true, key: boardKey}
	onePanel(g, p, true)
	if g.open || !p.open {
		t.Errorf("opening the extension panel over Git left git %v, panel %v", g.open, p.open)
	}
	g.open = true
	onePanel(g, p, false)
	if !g.open || p.open {
		t.Errorf("opening Git over the extension panel left git %v, panel %v", g.open, p.open)
	}
}

func TestStoredPanelHeightsStayWithinTheirBounds(t *testing.T) {
	got := parsePanelHeights(`{"a/b": 999, "c/d": 15, "e/f": -1}`)
	if got["a/b"] != gitMaxRem || got["c/d"] != 15 || len(got) != 2 {
		t.Errorf("heights %v", got)
	}
	if len(parsePanelHeights("garbage")) != 0 {
		t.Error("garbage parsed as heights")
	}
}

// panelFrames is the stage column's bottom rows with an extension's panel
// item in the footer, as body lays them out.
func panelFrames(t *testing.T) (*window, *frameRig) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	_, client := newSidebarBackend(t)
	u := &window{ctx: t.Context(), client: client, model: newModel(nil, r.invalidate), settings: new(settingsScreen)}
	u.model.extensions = *trackerStore(api.ExtensionRunning)
	u.model.extensions.sawPath("/home/me/repo")
	u.model.extensions.setFooter("tracker", extension.FooterData{Surface: "board", Text: "Sprint 4"})
	u.extension = newExtensionScreen(&u.model.extensions, &u.root, func(string, extension.Message) {})
	u.extPanel = newExtensionPanel(&u.root, u.extension)
	u.git = newGitPanel(u.ctx, client, &u.root, r.invalidate, func(ui.Toast) {}, "monospace", 12)
	u.footer = newFooter(u.ctx, client, &u.root, newFooterUsage(), newPlanQuota(quotaReader(client), r.invalidate), r.invalidate)
	u.footer.git, u.footer.dock, u.footer.panel, u.footer.extensions = u.git, new(dock), u.extPanel, &u.model.extensions
	u.footer.layout.Left = append(u.footer.layout.Left, extensionItemID(boardKey))
	v := stageView{in: busySession()}
	u.footer.readExtensions(&v.in)
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			u.actGit(gtx)
			onePanel(u.git, u.extPanel, u.gitWasOpen)
			u.gitWasOpen = u.git.open
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Max} }),
				layout.Rigid(func(gtx C) D { return u.gitPanelRow(gtx, v) }),
				layout.Rigid(func(gtx C) D { return u.extPanelRow(gtx, v) }),
				layout.Rigid(func(gtx C) D { return u.footer.layoutBar(gtx, th, v.in) }),
			)
		})
	}
	return u, r
}

func TestAPanelItemOpensItsPanelInTheFrameThatTookTheClick(t *testing.T) {
	u, r := panelFrames(t)
	r.clickLabel(t, "Sprint 4")
	if !u.extPanel.shows(boardKey) {
		t.Fatal("the click missed the panel item")
	}
}

func TestAPanelItemTakesTheGitPanelsPlace(t *testing.T) {
	u, r := panelFrames(t)
	r.clickLabel(t, "main")
	if !u.git.open {
		t.Fatal("the click missed the branch")
	}
	// The Git panel reads its log off the frame and never goes still here.
	r.click(t, r.at(t, "Sprint 4"))
	if u.git.open || !u.extPanel.shows(boardKey) {
		t.Fatalf("git open %v, panel on board %v", u.git.open, u.extPanel.shows(boardKey))
	}
}
