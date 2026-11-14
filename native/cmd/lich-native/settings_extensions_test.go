package main

import (
	"encoding/json"
	"image"
	"net/http"
	"slices"
	"sync"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func TestTheStoreKeepsSourceAndReleaseAndDropsARemovedExtension(t *testing.T) {
	m := newTestModel(nil)
	m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, backend.Extension{ID: "a", Status: api.ExtensionRunning, Source: "https://github.com/o/a", Release: "v0.1.0"}))
	m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, backend.Extension{ID: "b", Status: api.ExtensionRunning}))
	m.extensionEvent(messageEv(t, "a", extension.Render.New(extension.RenderData{Surface: "main", Root: extension.Node{Type: "stack"}})))
	if e, _ := m.extensions.byID("a"); e.Source != "https://github.com/o/a" || e.Release != "v0.1.0" {
		t.Errorf("a = %+v, want its source and release", e)
	}
	m.extensionEvent(extensionEv(t, api.ExtensionRemovedEvent.Name, api.ExtensionIDOptions{ID: "a"}))
	if _, ok := m.extensions.byID("a"); ok {
		t.Error("a removed extension is still in the store")
	}
	if _, ok := m.extensions.render(extensionKey{"a", "main"}); ok {
		t.Error("a removed extension's render is still in the store")
	}
	if all := m.extensions.all(); len(all) != 1 || all[0].ID != "b" {
		t.Errorf("list = %+v, want only b", all)
	}
}

func TestARemovedExtensionsScreenCloses(t *testing.T) {
	var store extensionStore
	store.setStatus(backend.Extension{ID: "a", Status: api.ExtensionRunning})
	var sent []string
	s := newExtensionScreen(&store, new(ui.Root), func(id string, msg extension.Message) { sent = append(sent, id+":"+msg.Kind) })
	s.show(extensionKey{"a", "main"})
	store.remove("a")
	gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Exact(image.Pt(400, 300))}
	s.layout(gtx, testTheme(), false)
	if s.open {
		t.Fatal("the screen of a removed extension stayed open")
	}
	if len(sent) != 1 {
		t.Errorf("sent %v, want only the opening: the process is gone", sent)
	}
}

func TestExtensionSourceLineSaysWhereItCameFrom(t *testing.T) {
	cases := []struct {
		e    backend.Extension
		want string
	}{
		{backend.Extension{Source: "https://github.com/o/r", Release: "v0.1.0"}, "github.com/o/r · v0.1.0"},
		{backend.Extension{Source: "/home/me/ext", Dir: "/cfg/ext"}, "/home/me/ext"},
		{backend.Extension{Dir: "/cfg/lich/extensions/ext"}, "/cfg/lich/extensions/ext"},
	}
	for _, c := range cases {
		if got := extensionSourceLine(c.e); got != c.want {
			t.Errorf("%+v = %q, want %q", c.e, got, c.want)
		}
	}
}

// extensionBackend answers the extension calls and records what they asked.
type extensionBackend struct {
	mu    sync.Mutex
	calls []string
	args  []string
	fail  string
	// pick is the folder the directory picker answers.
	pick string
}

func (b *extensionBackend) stub() *rpcStub {
	return &rpcStub{answer: func(method string, args []json.RawMessage) (any, int) {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.calls = append(b.calls, method)
		if len(args) > 0 {
			b.args = append(b.args, string(args[0]))
		}
		if b.fail != "" {
			return failing(b.fail)
		}
		switch method {
		case api.ProjectPickDirectory.Name:
			return b.pick, http.StatusOK
		case api.ExtensionsRemove.Name, api.ExtensionsDeleteData.Name, api.ExtensionsRestart.Name, api.SystemOpenFolder.Name, api.ExtensionsDiscardInstall.Name:
			return api.None{}, http.StatusOK
		case api.ExtensionsPrepareFromGit.Name, api.ExtensionsPrepareFromDir.Name:
			return testPreview, http.StatusOK
		}
		return backend.Extension{ID: "x"}, http.StatusOK
	}}
}

func (b *extensionBackend) called() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.calls)
}

func (b *extensionBackend) last() (string, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.calls) == 0 {
		return "", ""
	}
	return b.calls[len(b.calls)-1], b.args[len(b.args)-1]
}

func extensionsScreen(t *testing.T, b *extensionBackend) *settingsScreen {
	t.Helper()
	s := newSettingsScreen(t.Context(), b.stub().client(t), new(ui.Root), func() {})
	s.extensions.store = new(extensionStore)
	return s
}

var testPreview = backend.ExtensionInstallPreview{Token: "tok", ID: "x", Name: "X", Version: "1.0.0", Permissions: []string{"sessions:write"}}

// The contract changed: Install and Choose folder… stage the extension first,
// and the confirmation opens on what staging returned.
func TestInstallPreparesFirstThenAsksOnThePreview(t *testing.T) {
	cases := []struct {
		from, src, method, args string
	}{
		{installFromGit, "  https://github.com/o/r  ", api.ExtensionsPrepareFromGit.Name, `{"url":"https://github.com/o/r"}`},
		{installFromDir, "/home/me/ext", api.ExtensionsPrepareFromDir.Name, `{"dir":"/home/me/ext"}`},
	}
	for _, c := range cases {
		b := &extensionBackend{}
		s := extensionsScreen(t, b)
		p := &s.extensions
		p.with(func(t *extensionTasks) { t.err = extensionError{c.from, "old"} })
		s.prepareInstall(c.from, c.src)
		if got := p.snapshot(); got.installing != c.from || got.asking || got.err.text != "" {
			t.Errorf("%s: %+v, want the row busy, nothing asked yet and the old error gone", c.from, got)
		}
		eventually(t, "the prepare answered", func() bool { return p.snapshot().asking })
		if method, args := b.last(); method != c.method || args != c.args {
			t.Errorf("called %s %s, want %s %s", method, args, c.method, c.args)
		}
		if got := p.snapshot(); got.installing != "" || got.askingFrom != c.from || got.preview.Token != "tok" {
			t.Errorf("%s: %+v, want the preview asked from its row", c.from, got)
		}
	}
}

func TestAnEmptyFieldPreparesNothing(t *testing.T) {
	b := &extensionBackend{}
	s := extensionsScreen(t, b)
	s.prepareInstall(installFromGit, "  ")
	if got := s.extensions.snapshot(); got.installing != "" || got.asking || len(b.called()) != 0 {
		t.Errorf("%+v and calls %v for an empty field, want nothing", got, b.called())
	}
}

func TestChooseFolderStagesThatFolder(t *testing.T) {
	b := &extensionBackend{pick: "/home/me/ext"}
	s := extensionsScreen(t, b)
	s.pickExtensionDir()
	eventually(t, "the folder was staged", func() bool { return s.extensions.snapshot().asking })
	if method, args := b.last(); method != api.ExtensionsPrepareFromDir.Name || args != `{"dir":"/home/me/ext"}` {
		t.Errorf("called %s %s, want the picked folder prepared", method, args)
	}
}

func TestACancelledFolderPickAsksNothing(t *testing.T) {
	b := &extensionBackend{}
	s := extensionsScreen(t, b)
	s.pickExtensionDir()
	eventually(t, "the picker was called", func() bool { m, _ := b.last(); return m != "" })
	if got := s.extensions.snapshot(); got.asking || got.installing != "" {
		t.Errorf("%+v after a cancelled pick, want nothing", got)
	}
}

func asked(s *settingsScreen, from string) {
	s.extensions.with(func(t *extensionTasks) { t.asking, t.preview, t.askingFrom = true, testPreview, from })
}

func TestInstallInTheConfirmationCommitsTheToken(t *testing.T) {
	b := &extensionBackend{}
	s := extensionsScreen(t, b)
	p := &s.extensions
	p.url.SetText("https://github.com/o/r")
	asked(s, installFromGit)
	s.commitInstall()
	if got := p.snapshot(); got.installing != installFromGit || got.asking {
		t.Errorf("%+v, want the GitHub row busy and the dialog shut", got)
	}
	eventually(t, "the commit finished", func() bool { return p.snapshot().installing == "" })
	if method, args := b.last(); method != api.ExtensionsCommitInstall.Name || args != `{"token":"tok"}` {
		t.Errorf("called %s %s, want the token committed", method, args)
	}
	if got := p.snapshot().err; got.text != "" || p.url.Text() != "" {
		t.Errorf("error %+v and field %q after a success, want neither", got, p.url.Text())
	}
}

func TestClosingTheConfirmationDiscardsTheToken(t *testing.T) {
	b := &extensionBackend{}
	s := extensionsScreen(t, b)
	asked(s, installFromDir)
	s.discardInstall()
	if s.extensions.snapshot().asking {
		t.Error("the dialog stayed open")
	}
	eventually(t, "the discard was sent", func() bool { m, _ := b.last(); return m != "" })
	if method, args := b.last(); method != api.ExtensionsDiscardInstall.Name || args != `{"token":"tok"}` {
		t.Errorf("called %s %s, want the token discarded", method, args)
	}
}

func TestAFailedPrepareLeavesTheBackendsWordsUnderItsRowAndAsksNothing(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{fail: "no release for linux_amd64"})
	p := &s.extensions
	s.prepareInstall(installFromGit, "https://github.com/o/r")
	eventually(t, "the prepare finished", func() bool { return p.snapshot().installing == "" })
	got := p.snapshot()
	if got.errorAt(installFromGit) != "no release for linux_amd64" || got.errorAt(installFromDir) != "" || got.asking {
		t.Errorf("%+v, want the backend's words under the GitHub row and no dialog", got)
	}
}

func TestAFailedCommitLeavesTheBackendsWordsAndTheField(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{fail: "already installed"})
	p := &s.extensions
	p.url.SetText("https://github.com/o/r")
	asked(s, installFromGit)
	s.commitInstall()
	eventually(t, "the commit finished", func() bool { return p.snapshot().installing == "" })
	if got := p.snapshot().errorAt(installFromGit); got != "already installed" {
		t.Errorf("GitHub row's error = %q, want the backend's words", got)
	}
	if p.url.Text() == "" {
		t.Error("a failed install emptied the field")
	}
}

func TestEachPermissionHasLichsOwnLine(t *testing.T) {
	want := map[extension.Permission]string{
		extension.PermissionProjectsRead:  "See your open projects and their worktrees",
		extension.PermissionSessionsRead:  "See your sessions and what each one is doing",
		extension.PermissionSessionsWrite: "Open sessions and type prompts into them",
		extension.PermissionSessionEvents: "Follow each session's status as it changes",
		extension.PermissionSessionsAsk:   "Ask your Claude Code sessions side questions and read their answers",
		extension.PermissionServicesRead:  "See your projects' services and what they print",
		extension.PermissionServicesWrite: "Start, stop and restart your projects' services",
		extension.PermissionGitRead:       "See each checkout's branch, changes and pull request",
		extension.PermissionNotify:        "Show desktop notifications",
		extension.PermissionWindow:        "Show files in the dock and open pages in a session's browser",
	}
	for _, p := range extension.Permissions {
		if got := permissionText(string(p)); got != want[p] {
			t.Errorf("%s = %q, want %q", p, got, want[p])
		}
	}
	if got := permissionText("files:write"); got != "files:write" {
		t.Errorf("an unknown permission reads %q, want its name", got)
	}
}

func TestOnlyThePermissionsThatActAreInTheWaitTone(t *testing.T) {
	th := testTheme()
	acting := []extension.Permission{
		extension.PermissionSessionsWrite, extension.PermissionSessionsAsk, extension.PermissionServicesWrite, extension.PermissionWindow,
	}
	for _, p := range extension.Permissions {
		want := th.Foreground
		if slices.Contains(acting, p) {
			want = th.ToneWait
		}
		if got := permissionTone(th, string(p)); got != want {
			t.Errorf("%s tone = %v, want %v", p, got, want)
		}
	}
}

func TestTheConfirmationTitleAndSourceLine(t *testing.T) {
	git := backend.ExtensionInstallPreview{Name: "Notes", Version: "1.2.0", Source: "https://github.com/o/notes", Release: "v1.2.0"}
	if got := installTitle(git); got != "Install Notes 1.2.0?" {
		t.Errorf("title = %q", got)
	}
	if got := installTitle(backend.ExtensionInstallPreview{Name: "Notes"}); got != "Install Notes?" {
		t.Errorf("title without a version = %q", got)
	}
	if got := previewSourceLine(git); got != "github.com/o/notes · v1.2.0" {
		t.Errorf("source line = %q", got)
	}
}

func shapingTheme() *ui.Theme {
	return &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))}
}

func TestNoPermissionsDrawsNoList(t *testing.T) {
	th := shapingTheme()
	height := func(perms []string) int {
		gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Constraints{Max: image.Pt(600, 2000)}}
		return installWarning(gtx, th, perms, nil).Size.Y
	}
	none, one, two := height(nil), height([]string{"projects:read"}), height([]string{"projects:read", "sessions:write"})
	if !(none < one && one < two) {
		t.Errorf("heights none %d, one %d, two %d: want the list only when there are permissions, a line each", none, one, two)
	}
}

func TestTheStatusTooltipListsThePermissions(t *testing.T) {
	th := shapingTheme()
	height := func(e backend.Extension) int {
		gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Constraints{Max: image.Pt(600, 2000)}}
		return statusTip(gtx, th, e).Size.Y
	}
	errOnly := height(backend.Extension{Error: "exit 1"})
	both := height(backend.Extension{Error: "exit 1", Permissions: []string{"sessions:read"}})
	permsOnly := height(backend.Extension{Permissions: []string{"sessions:read"}})
	if !(errOnly < both && permsOnly < both && permsOnly > 0) {
		t.Errorf("heights error %d, permissions %d, both %d: want the list under the error", errOnly, permsOnly, both)
	}
}

func TestEditingTheURLClearsOnlyTheGitHubRowsError(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{})
	p := &s.extensions
	gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Exact(image.Pt(400, 300))}
	p.with(func(t *extensionTasks) { t.err = extensionError{installFromDir, "bad folder"} })
	p.url.SetText("https://github.com/o/r")
	p.urlEvents(gtx)
	if got := p.snapshot().errorAt(installFromDir); got != "bad folder" {
		t.Errorf("the folder row's error = %q after a URL edit, want it kept", got)
	}
	p.with(func(t *extensionTasks) { t.err = extensionError{installFromGit, "no release"} })
	p.url.SetText("https://github.com/o/r2")
	p.urlEvents(gtx)
	if got := p.snapshot().err; got.text != "" {
		t.Errorf("error %+v after editing the URL, want it gone", got)
	}
}

func TestTheRemoveConfirmationDeletesTheDataOnlyWhenTicked(t *testing.T) {
	cases := []struct {
		ticked bool
		want   []string
	}{
		{false, []string{api.ExtensionsRemove.Name}},
		{true, []string{api.ExtensionsRemove.Name, api.ExtensionsDeleteData.Name}},
	}
	for _, c := range cases {
		b := &extensionBackend{}
		s := extensionsScreen(t, b)
		p := &s.extensions
		e := backend.Extension{ID: "notes", Name: "Notes"}
		p.deleteData.Value = true
		p.askRemove(e)
		if p.deleteData.Value {
			t.Fatal("the remove confirmation opened with its box ticked")
		}
		p.deleteData.Value = c.ticked
		s.confirmRemoval(e)
		eventually(t, "the calls were made", func() bool { return len(b.called()) == len(c.want) })
		if got := b.called(); !slices.Equal(got, c.want) {
			t.Errorf("ticked %v called %v, want %v", c.ticked, got, c.want)
		}
		if _, args := b.last(); args != `{"id":"notes"}` {
			t.Errorf("ticked %v last call had %s, want notes", c.ticked, args)
		}
	}
}

func TestDeleteDataIsNotCalledWhenRemoveFails(t *testing.T) {
	b := &extensionBackend{fail: "busy"}
	s := extensionsScreen(t, b)
	e := backend.Extension{ID: "notes", Name: "Notes"}
	s.extensions.askRemove(e)
	s.extensions.deleteData.Value = true
	s.confirmRemoval(e)
	eventually(t, "the error landed on the row", func() bool { return s.extensions.snapshot().errorAt(errAtRow("notes")) != "" })
	if got := b.called(); !slices.Equal(got, []string{api.ExtensionsRemove.Name}) {
		t.Errorf("called %v after a failed Remove, want Remove alone", got)
	}
}

func TestRemoveAsksThenCallsRemoveWithTheID(t *testing.T) {
	b := &extensionBackend{}
	s := extensionsScreen(t, b)
	p := &s.extensions
	e := backend.Extension{ID: "notes", Name: "Notes"}
	p.askRemove(e)
	if got := p.snapshot().removal; got == nil || got.ID != "notes" {
		t.Fatalf("removal = %+v, want notes asked", got)
	}
	s.removeExtension(e)
	if p.snapshot().removal != nil {
		t.Error("the remove dialog stayed open")
	}
	eventually(t, "Remove was called", func() bool { m, _ := b.last(); return m != "" })
	if method, args := b.last(); method != api.ExtensionsRemove.Name || args != `{"id":"notes"}` {
		t.Errorf("called %s %s, want %s for notes", method, args, api.ExtensionsRemove.Name)
	}
}

func TestTheRowMenuOffersRestartOnlyWhenNotRunning(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{})
	cases := map[string][]string{
		api.ExtensionRunning:    {"Open folder", "---", "Remove…"},
		api.ExtensionRestarting: {"Restart", "Open folder", "---", "Remove…"},
		api.ExtensionFailed:     {"Restart", "Open folder", "---", "Remove…"},
		api.ExtensionInvalid:    {"Restart", "Open folder", "---", "Remove…"},
		api.ExtensionStopped:    {"Restart", "Open folder", "---", "Remove…"},
	}
	for status, want := range cases {
		got := menuLabels(s.extensionMenu(backend.Extension{ID: "a", Status: status}))
		if !slices.Equal(got, want) {
			t.Errorf("%s: menu %q, want %q", status, got, want)
		}
	}
}

func TestEachRowMenuEntryCallsItsMethod(t *testing.T) {
	b := &extensionBackend{}
	s := extensionsScreen(t, b)
	e := backend.Extension{ID: "notes", Name: "Notes", Status: api.ExtensionFailed, Dir: "/cfg/ext/notes"}
	entries := s.extensionMenu(e)
	want := []struct{ method, args string }{
		{api.ExtensionsRestart.Name, `{"id":"notes"}`},
		{api.SystemOpenFolder.Name, `{"dir":"/cfg/ext/notes"}`},
	}
	for i, w := range want {
		entries[i].OnClick()
		eventually(t, w.method+" was called", func() bool { m, _ := b.last(); return m == w.method })
		if _, args := b.last(); args != w.args {
			t.Errorf("%s with %s, want %s", w.method, args, w.args)
		}
	}
	entries[len(entries)-1].OnClick()
	if got := s.extensions.snapshot().removal; got == nil || got.ID != "notes" {
		t.Errorf("Remove… left removal %+v, want the confirmation for notes", got)
	}
}

func TestARowActionsErrorLandsUnderThatRow(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{fail: "not failed"})
	s.restartExtension("notes")
	eventually(t, "the restart failed", func() bool { return s.extensions.snapshot().err.text != "" })
	if got := s.extensions.snapshot().errorAt(errAtRow("notes")); got != "not failed" {
		t.Errorf("notes' row shows %q, want the backend's words", got)
	}
}

// The contract changed: with nothing installed the Installed group gives way
// to the catalog under it, and the install rows moved under Other sources.
func TestTheListIsAHeadingOverOneRowEachOrNothing(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{})
	th := testTheme()
	empty := len(s.extensionsPaneRows(th))
	s.extensions.store.setStatus(backend.Extension{ID: "a"})
	s.extensions.store.setStatus(backend.Extension{ID: "b"})
	if got := len(s.extensionsPaneRows(th)); got != empty+3 {
		t.Errorf("rows = %d with two extensions, want %d: the heading and a row each", got, empty+3)
	}
	if empty != 4 {
		t.Errorf("rows = %d with none, want the description, the Other sources heading and two install rows", empty)
	}
}

// updateBackend answers an update's calls: PrepareUpdate with preview, the
// commit with nothing worth reading.
func updateBackend(preview backend.ExtensionInstallPreview, found []backend.Extension) (*extensionBackend, *rpcStub) {
	b := &extensionBackend{}
	return b, &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.calls = append(b.calls, method)
		switch method {
		case api.ExtensionsPrepareUpdate.Name:
			return preview, http.StatusOK
		case api.ExtensionsCheckUpdates.Name:
			return found, http.StatusOK
		}
		return backend.Extension{ID: "x"}, http.StatusOK
	}}
}

func updatingScreen(t *testing.T, stub *rpcStub, installed backend.Extension) *settingsScreen {
	t.Helper()
	s := newSettingsScreen(t.Context(), stub.client(t), new(ui.Root), func() {})
	s.theme = &themePicker{toaster: new(ui.Toaster)}
	s.extensions.store = &extensionStore{list: []backend.Extension{installed}}
	return s
}

func TestUpdateInstallsAtOnceWhenItAsksForNothingNew(t *testing.T) {
	installed := backend.Extension{ID: "x", Name: "X", Release: "v1.0.0", Latest: "v1.1.0", Permissions: []string{"sessions:read", "notify"}}
	preview := backend.ExtensionInstallPreview{Token: "tok", ID: "x", Release: "v1.1.0", Permissions: []string{"notify"}}
	b, stub := updateBackend(preview, nil)
	s := updatingScreen(t, stub, installed)

	s.updateExtension(installed)
	waitFor(t, "the commit", func() bool { return slices.Contains(b.called(), api.ExtensionsCommitInstall.Name) })
	waitFor(t, "the update done", func() bool { return s.extensions.snapshot().installing == "" })
	if s.extensions.snapshot().asking {
		t.Error("an update that asks for nothing new opened the confirmation")
	}
	if e, _ := s.extensions.store.byID("x"); e.Latest != "" {
		t.Errorf("the row still offers %q after the update", e.Latest)
	}
}

func TestUpdateAsksFirstWhenItWantsANewPermission(t *testing.T) {
	installed := backend.Extension{ID: "x", Name: "X", Release: "v1.0.0", Latest: "v1.1.0", Permissions: []string{"notify"}}
	preview := backend.ExtensionInstallPreview{Token: "tok", ID: "x", Release: "v1.1.0", Permissions: []string{"notify", "sessions:write"}}
	b, stub := updateBackend(preview, nil)
	s := updatingScreen(t, stub, installed)

	s.updateExtension(installed)
	waitFor(t, "the confirmation", func() bool { return s.extensions.snapshot().asking })
	tasks := s.extensions.snapshot()
	if tasks.replacing == nil || tasks.replacing.ID != "x" || tasks.installing != "" {
		t.Fatalf("tasks = %+v, want the confirmation open on the update of x", tasks)
	}
	if slices.Contains(b.called(), api.ExtensionsCommitInstall.Name) {
		t.Fatal("the update was installed before its new permission was confirmed")
	}
	if got := newPermissions(installed.Permissions, preview.Permissions); !slices.Equal(got, []string{"sessions:write"}) {
		t.Errorf("newPermissions() = %v, want only sessions:write", got)
	}
	s.commitUpdate(installed, preview)
	if !slices.Contains(b.called(), api.ExtensionsCommitInstall.Name) || s.extensions.snapshot().asking {
		t.Errorf("after the confirmation: calls %v, asking %v", b.called(), s.extensions.snapshot().asking)
	}
}

func TestCheckForUpdatesFillsTheRows(t *testing.T) {
	installed := backend.Extension{ID: "x", Release: "v1.0.0"}
	found := installed
	found.Latest = "v1.1.0"
	_, stub := updateBackend(backend.ExtensionInstallPreview{}, []backend.Extension{found})
	s := updatingScreen(t, stub, installed)

	s.checkExtensionUpdates()
	waitFor(t, "the newer release on the row", func() bool { e, _ := s.extensions.store.byID("x"); return e.Latest == "v1.1.0" })
	waitFor(t, "the check done", func() bool { return !s.extensions.snapshot().checking })
}

func TestAnUpdateEventReachesTheStoreAndTheWindow(t *testing.T) {
	var m model
	var heard []string
	m.extensionUpdate = func(id string) { heard = append(heard, id) }
	m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, backend.Extension{ID: "a", Release: "v1.0.0"}))
	m.extensionEvent(extensionEv(t, api.ExtensionUpdateEvent.Name, api.ExtensionUpdate{ID: "a", Latest: "v1.1.0"}))
	if e, _ := m.extensions.byID("a"); e.Latest != "v1.1.0" || !slices.Equal(heard, []string{"a"}) {
		t.Fatalf("store has %q, window heard %v; want v1.1.0 and a", e.Latest, heard)
	}
}
