package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"slices"
	"sync"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

const notesRepo = "https://github.com/lichdotdev/lich-notes"

func testCatalog() backend.ExtensionCatalog {
	return backend.ExtensionCatalog{Platform: "linux_amd64", Catalog: extension.Catalog{
		Categories: []extension.CatalogCategory{
			{ID: "agents", Names: map[string]string{"en": "Agents", "pt-br": "Agentes"}},
			{ID: "git", Names: map[string]string{"en": "Git"}},
			{ID: "tools", Names: map[string]string{"en": "Tools"}},
		},
		Extensions: []extension.CatalogEntry{
			{ID: "notes", Name: "LichNotes", Description: "Markdown notes", Translations: map[string]extension.CatalogText{"pt-br": {Description: "Notas em Markdown"}},
				Publisher: "lichdotdev", Official: true, Stars: 3, Released: "2026-11-14T02:57:14Z", Categories: []string{"agents"},
				Builds: map[string]string{"linux_amd64": "a"}, Repo: notesRepo, Version: "0.1.2"},
			{ID: "board", Name: "PR Board", Description: "Every pull request on one board, grouped by review state, across your projects.",
				Publisher: "sample-dev", Stars: 200, Released: "2026-11-10T00:00:00Z", Categories: []string{"git", "agents"},
				Builds: map[string]string{"linux_amd64": "b"}, Repo: "https://github.com/sample-dev/board"},
			{ID: "menubar", Name: "Menu Bar", Description: "The sessions that wait on you, in the macOS menu bar.",
				Publisher: "someone", Stars: 12, Released: "2026-11-02T00:00:00Z", Categories: []string{"agents"},
				Builds: map[string]string{"darwin_arm64": "c"}, Repo: "https://github.com/someone/menubar"},
		},
	}}
}

func ids(entries []extension.CatalogEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

func TestCatalogCountsStartWithAllAndLeaveOutAnEmptyCategory(t *testing.T) {
	got := catalogCounts(testCatalog().Catalog)
	want := []catalogCount{{"", "All", 3}, {"agents", "Agents", 3}, {"git", "Git", 1}}
	if !slices.Equal(got, want) {
		t.Errorf("counts = %+v, want %+v", got, want)
	}
}

func TestTheFilterNarrowsByCategoryAndQueryAndOrders(t *testing.T) {
	cat := testCatalog().Catalog
	cases := []struct {
		name   string
		filter catalogFilter
		want   []string
	}{
		{"recent first by default", catalogFilter{}, []string{"notes", "board", "menubar"}},
		{"by stars", catalogFilter{sort: sortStars}, []string{"board", "menubar", "notes"}},
		{"by name", catalogFilter{sort: sortName}, []string{"notes", "menubar", "board"}},
		{"a category", catalogFilter{category: "git"}, []string{"board"}},
		{"a word of the description, any case", catalogFilter{query: " MARKDOWN "}, []string{"notes"}},
		{"the publisher", catalogFilter{query: "sample-dev"}, []string{"board"}},
		{"the id", catalogFilter{query: "menubar"}, []string{"menubar"}},
		{"nothing", catalogFilter{query: "zzz"}, []string{}},
	}
	for _, c := range cases {
		if got := ids(c.filter.entries(cat)); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCatalogTextFollowsTheInterfaceLanguage(t *testing.T) {
	cat := testCatalog().Catalog
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	if got := entryDescription(cat.Extensions[0]); got != "Notas em Markdown" {
		t.Errorf("description = %q, want the pt-br one", got)
	}
	if got := entryName(cat.Extensions[0]); got != "LichNotes" {
		t.Errorf("name = %q, want the entry's own where pt-br has none", got)
	}
	if got := categoryName(cat.Categories[0]); got != "Agentes" {
		t.Errorf("category = %q", got)
	}
	if got := categoryName(cat.Categories[1]); got != "Git" {
		t.Errorf("category without pt-br = %q, want English", got)
	}
	if !entryMatches(cat.Extensions[0], "notas") {
		t.Error("the search does not find the translated description")
	}
}

func TestPlatformAndSystemNames(t *testing.T) {
	if got := platformName("darwin_arm64"); got != "macOS Apple Silicon" {
		t.Errorf("darwin_arm64 = %q", got)
	}
	if got := platformName("plan9_386"); got != "plan9_386" {
		t.Errorf("an unknown key = %q, want itself", got)
	}
	if got := systemName("windows_arm64"); got != "Windows" {
		t.Errorf("windows_arm64's system = %q", got)
	}
}

// catalogBackend answers extensions.Catalog with answers in turn, the last
// one from then on, and each image with a PNG.
func catalogBackend(answers ...func() (any, int)) *rpcStub {
	var mu sync.Mutex
	n := 0
	return &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		switch method {
		case api.ExtensionsCatalog.Name:
			mu.Lock()
			a := answers[min(n, len(answers)-1)]
			n++
			mu.Unlock()
			return a()
		case api.ExtensionsCatalogImage.Name:
			var buf bytes.Buffer
			_ = png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 4, 4)))
			return buf.Bytes(), http.StatusOK
		}
		return api.None{}, http.StatusOK
	}}
}

func TestTheCatalogStoreReadsOnceAndAgainOnlyOnTryAgain(t *testing.T) {
	stub := catalogBackend(func() (any, int) { return failing("dial tcp: no such host") }, func() (any, int) { return testCatalog(), http.StatusOK })
	store := newCatalogStore(stub.client(t), func() {})
	store.read()
	eventually(t, "the failed read", func() bool { _, err := store.read(); return err != "" })
	if _, err := store.read(); err != "dial tcp: no such host" || stub.calls(api.ExtensionsCatalog.Name) != 1 {
		t.Fatalf("err %q after %d reads, want the backend's words after one", err, stub.calls(api.ExtensionsCatalog.Name))
	}
	store.retry()
	eventually(t, "the second read", func() bool { cat, _ := store.read(); return cat != nil })
	if cat, err := store.read(); err != "" || len(cat.Catalog.Extensions) != 3 {
		t.Errorf("catalog %+v, %q after Try again", cat, err)
	}
}

func TestACopyTheEventBroughtWinsOverAnAnswerStillOnItsWay(t *testing.T) {
	release := make(chan struct{})
	stale := testCatalog()
	stale.Catalog.Extensions = stale.Catalog.Extensions[:1]
	stub := catalogBackend(func() (any, int) { <-release; return stale, http.StatusOK })
	store := newCatalogStore(stub.client(t), func() {})
	store.read()
	eventually(t, "the read under way", func() bool { return stub.calls(api.ExtensionsCatalog.Name) == 1 })
	store.set(testCatalog())
	close(release)
	eventually(t, "the read done", func() bool { store.mu.Lock(); defer store.mu.Unlock(); return !store.reading })
	if cat, _ := store.read(); len(cat.Catalog.Extensions) != 3 {
		t.Errorf("catalog has %d extensions, want the event's three", len(cat.Catalog.Extensions))
	}
}

func TestEachCatalogImageIsReadOnce(t *testing.T) {
	stub := catalogBackend(func() (any, int) { return testCatalog(), http.StatusOK })
	store := newCatalogStore(stub.client(t), func() {})
	if store.image("") != nil {
		t.Fatal("no URL drew an image")
	}
	store.image("https://x/cover.png")
	eventually(t, "the image", func() bool { return store.image("https://x/cover.png") != nil })
	store.image("https://x/cover.png")
	if n := stub.calls(api.ExtensionsCatalogImage.Name); n != 1 {
		t.Errorf("read %d times, want once", n)
	}
}

// catalogScreen is the Extensions pane with cat held and nothing installed.
func catalogScreen(t *testing.T, b *extensionBackend, cat backend.ExtensionCatalog) *settingsScreen {
	t.Helper()
	s := extensionsScreen(t, b)
	s.extensions.catalog = newCatalogStore(nil, func() {})
	s.extensions.catalog.set(cat)
	return s
}

func TestTheCatalogGroupIsTheHeadingTheStripAndTheGrid(t *testing.T) {
	th := testTheme()
	s := catalogScreen(t, &extensionBackend{}, testCatalog())
	if got := len(s.catalogRows(th, nil, true)); got != 3 {
		t.Errorf("rows = %d, want the heading, the strip and the grid", got)
	}
	s.extensions.grid.query.SetText("zzz")
	if got := len(s.catalogRows(th, nil, true)); got != 3 {
		t.Errorf("rows = %d for no match, want the heading, the strip and the sentence", got)
	}
	s.extensions.catalog = &catalogStore{reading: true}
	if got := len(s.catalogRows(th, nil, true)); got != 2 {
		t.Errorf("rows = %d while reading, want the heading and the skeleton", got)
	}
	s.extensions.catalog = &catalogStore{err: "no such host"}
	if got := len(s.catalogRows(th, nil, true)); got != 2 {
		t.Errorf("rows = %d after a failed read, want the heading and why", got)
	}
}

func TestTheGridTakesAsManyColumnsAsTheWidthHolds(t *testing.T) {
	gtx := layout.Context{Ops: new(op.Ops)}
	for _, c := range []struct{ width, want int }{{200, 1}, {400, 1}, {600, 2}, {700, 2}, {1000, 3}, {1330, 4}} {
		if got := catalogColumns(gtx, c.width); got != c.want {
			t.Errorf("%dpx holds %d columns, want %d", c.width, got, c.want)
		}
	}
}

func TestEveryCardAndTheSkeletonStandAsTall(t *testing.T) {
	th := shapingTheme()
	s := catalogScreen(t, &extensionBackend{}, testCatalog())
	cat, _ := s.extensions.catalog.read()
	height := func(w func(gtx C) D) int {
		gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Constraints{Max: image.Pt(340, 2000)}}
		return w(gtx).Size.Y
	}
	short := height(func(gtx C) D { return s.catalogCard(gtx, th, cat, cat.Catalog.Extensions[0], true) })
	long := height(func(gtx C) D { return s.catalogCard(gtx, th, cat, cat.Catalog.Extensions[1], false) })
	skeleton := height(func(gtx C) D { return catalogSkeletonCard(gtx, th) })
	if short != long || skeleton != long {
		t.Errorf("heights: one line of description %d, two %d, skeleton %d; want one height", short, long, skeleton)
	}
}

func TestInstallInTheDetailStagesTheListedRepositoryAndLeadsToTheConfirmation(t *testing.T) {
	b := &extensionBackend{}
	s := catalogScreen(t, b, testCatalog())
	p := &s.extensions
	p.grid.detail.show("notes")
	s.prepareInstall(installFromCatalog, notesRepo)
	eventually(t, "the confirmation", func() bool { return p.snapshot().asking })
	if method, args := b.last(); method != api.ExtensionsPrepareFromGit.Name || args != `{"url":"`+notesRepo+`"}` {
		t.Errorf("called %s %s, want the listed repository staged", method, args)
	}
	gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Exact(image.Pt(1280, 800))}
	s.layoutExtensionDialogs(gtx, shapingTheme())
	if p.grid.detail.id != "" {
		t.Error("the detail stayed open under the confirmation")
	}
}

func TestTheDetailWaitsForTheCatalogInsteadOfClosing(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{})
	s.extensions.catalog = &catalogStore{reading: true}
	s.extensions.grid.detail.show("notes")
	gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Exact(image.Pt(1280, 800))}
	s.layoutExtensionDialogs(gtx, shapingTheme())
	if s.extensions.grid.detail.id != "notes" {
		t.Error("a detail opened before the catalog arrived closed itself")
	}
}

func TestAFailedInstallFromTheDetailSaysWhyInIt(t *testing.T) {
	s := catalogScreen(t, &extensionBackend{fail: "no build for linux_amd64"}, testCatalog())
	p := &s.extensions
	p.grid.detail.show("notes")
	s.prepareInstall(installFromCatalog, notesRepo)
	eventually(t, "the prepare done", func() bool { return p.snapshot().installing == "" })
	if got := p.snapshot().errorAt(installFromCatalog); got != "no build for linux_amd64" || p.grid.detail.id == "" {
		t.Errorf("error %q, detail %q; want the backend's words with the detail open", got, p.grid.detail.id)
	}
}

func TestTheDetailOffersWhatTheListingAllows(t *testing.T) {
	th := testTheme()
	s := catalogScreen(t, &extensionBackend{}, testCatalog())
	cat, _ := s.extensions.catalog.read()
	notes, menubar := cat.Catalog.Extensions[0], cat.Catalog.Extensions[2]
	cases := []struct {
		name      string
		e         extension.CatalogEntry
		installed *backend.Extension
		action    bool
	}{
		{"built for this system", notes, nil, true},
		{"no build for it", menubar, nil, false},
		{"installed with a newer release", notes, &backend.Extension{ID: "notes", Version: "0.1.1", Latest: "v0.1.2"}, true},
		{"installed and current", notes, &backend.Extension{ID: "notes", Version: "0.1.2"}, false},
	}
	for _, c := range cases {
		if _, action := s.detailAction(th, cat, c.e, c.installed, extensionTasks{}); (action != nil) != c.action {
			t.Errorf("%s: action %v, want %v", c.name, action != nil, c.action)
		}
	}
}

func TestTheConfirmationNamesThePublisherAndTheCatalogsWord(t *testing.T) {
	git := backend.ExtensionInstallPreview{Source: "https://github.com/o/notes", Release: "v1.2.0", Publisher: "o"}
	if got := previewSource(git); got != "From github.com/o/notes · v1.2.0, published by o" {
		t.Errorf("source = %q", got)
	}
	if got := previewSource(backend.ExtensionInstallPreview{Source: "/home/me/ext"}); got != "From /home/me/ext" {
		t.Errorf("a folder's source = %q", got)
	}
	th := shapingTheme()
	height := func(p backend.ExtensionInstallPreview) int {
		gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Constraints{Max: image.Pt(500, 2000)}}
		return previewWarning(gtx, th, p, nil).Size.Y
	}
	listed := git
	listed.Listed = true
	if height(listed) <= height(git) {
		t.Error("a listed release's confirmation does not say what the catalog checks")
	}
}

func TestARevokedRowSaysWhyIsStoppedAndOffersNoRestart(t *testing.T) {
	th := testTheme()
	e := backend.Extension{ID: "clip", Status: api.ExtensionInvalid, Revoked: "it typed prompts", Description: "Clips", Source: "https://github.com/o/clip", Release: "v0.9.0"}
	if got := extensionSecondLine(th, e).Text; got != "Removed from the lich catalog: it typed prompts" {
		t.Errorf("second line = %q", got)
	}
	if got := rowStatus(e); got != api.ExtensionStopped {
		t.Errorf("status = %q, want stopped", got)
	}
	s := extensionsScreen(t, &extensionBackend{})
	for _, entry := range s.extensionMenu(e) {
		if entry.Label == "Restart" {
			t.Error("a revoked extension's menu offers Restart")
		}
	}
	e.Revoked = ""
	if got := extensionSecondLine(th, e).Text; got != "Clips" {
		t.Errorf("second line = %q, want the description", got)
	}
	e.Description = ""
	if got := extensionSecondLine(th, e).Text; got != "github.com/o/clip · v0.9.0" {
		t.Errorf("second line = %q, want the source without a description", got)
	}
}

func TestTheStatusTooltipStartsWithWhereItCameFrom(t *testing.T) {
	th := shapingTheme()
	height := func(e backend.Extension) int {
		gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Constraints{Max: image.Pt(600, 2000)}}
		return statusTip(gtx, th, e).Size.Y
	}
	perms := []string{"sessions:read"}
	if height(backend.Extension{Source: "https://github.com/o/r", Permissions: perms}) <= height(backend.Extension{Permissions: perms}) {
		t.Error("the tooltip leaves the source out")
	}
	revoked := backend.Extension{Revoked: "x", Error: "removed from the lich catalog: x", Permissions: perms}
	if height(revoked) != height(backend.Extension{Permissions: perms}) {
		t.Error("a revoked extension's tooltip repeats the reason its row shows")
	}
}
