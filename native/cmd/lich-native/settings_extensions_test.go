package main

import (
	"encoding/json"
	"image"
	"net/http"
	"slices"
	"sync"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"

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
		case api.ExtensionsRemove.Name, api.ExtensionsDeleteData.Name, api.ExtensionsRestart.Name, api.SystemOpenFolder.Name:
			return api.None{}, http.StatusOK
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

func TestInstallFromGitHubAsksFirstAndNamesTheURL(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{})
	p := &s.extensions
	p.askInstall(installFromGit, "  ")
	if p.snapshot().asking != "" {
		t.Fatal("an empty field asked to install")
	}
	p.with(func(t *extensionTasks) { t.err = extensionError{installFromGit, "old"} })
	p.askInstall(installFromGit, "  https://github.com/o/r  ")
	if got := p.snapshot(); got.asking != "https://github.com/o/r" || got.askingFrom != installFromGit || got.err.text != "" {
		t.Errorf("%+v, want the URL asked from GitHub and the old error gone", got)
	}
}

func TestChooseFolderGoesStraightToTheConfirmationForThatFolder(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{pick: "/home/me/ext"})
	p := &s.extensions
	p.with(func(t *extensionTasks) { t.err = extensionError{installFromDir, "old"} })
	s.pickExtensionDir()
	eventually(t, "the picker answered", func() bool { return p.snapshot().asking != "" })
	if got := p.snapshot(); got.asking != "/home/me/ext" || got.askingFrom != installFromDir || got.err.text != "" {
		t.Errorf("%+v, want the folder asked from a folder and the old error gone", got)
	}
}

func TestACancelledFolderPickAsksNothing(t *testing.T) {
	b := &extensionBackend{}
	s := extensionsScreen(t, b)
	s.pickExtensionDir()
	eventually(t, "the picker was called", func() bool { m, _ := b.last(); return m != "" })
	if got := s.extensions.snapshot().asking; got != "" {
		t.Errorf("asking %q after a cancelled pick, want nothing", got)
	}
}

func TestConfirmedInstallCallsTheRightMethodAndShowsBusyOnItsRow(t *testing.T) {
	cases := []struct {
		fromDir         bool
		src, want, from string
		wantMethod      string
	}{
		{false, "https://github.com/o/r", `{"url":"https://github.com/o/r"}`, installFromGit, api.ExtensionsInstallFromGit.Name},
		{true, "/home/me/ext", `{"dir":"/home/me/ext"}`, installFromDir, api.ExtensionsInstallFromDir.Name},
	}
	for _, c := range cases {
		b := &extensionBackend{}
		s := extensionsScreen(t, b)
		p := &s.extensions
		p.with(func(t *extensionTasks) { t.asking = c.src })
		s.installExtension(c.fromDir, c.src)
		if got := p.snapshot(); got.installing != c.from || got.asking != "" {
			t.Errorf("%s: %+v, want %s busy and the dialog shut", c.wantMethod, got, c.from)
		}
		eventually(t, "the install finished", func() bool { return p.snapshot().installing == "" })
		if method, args := b.last(); method != c.wantMethod || args != c.want {
			t.Errorf("called %s %s, want %s %s", method, args, c.wantMethod, c.want)
		}
		if got := p.snapshot().err; got.text != "" {
			t.Errorf("%s: error %+v after a success", c.wantMethod, got)
		}
	}
}

func TestAFailedInstallLeavesTheBackendsWordsUnderItsRow(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{fail: "no release for linux_amd64"})
	p := &s.extensions
	p.url.SetText("https://github.com/o/r")
	s.installExtension(false, "https://github.com/o/r")
	eventually(t, "the install finished", func() bool { return p.snapshot().installing == "" })
	t2 := p.snapshot()
	if got := t2.errorAt(installFromGit); got != "no release for linux_amd64" {
		t.Errorf("GitHub row's error = %q, want the backend's words", got)
	}
	if got := t2.errorAt(installFromDir); got != "" {
		t.Errorf("the folder row shows %q, want nothing: the error came from GitHub", got)
	}
	if p.url.Text() == "" {
		t.Error("a failed install emptied the field")
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

func TestTheListIsAHeadingOverOneRowEachOrTheEmptyState(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{})
	th := testTheme()
	empty := len(s.extensionsPaneRows(th))
	s.extensions.store.setStatus(backend.Extension{ID: "a"})
	s.extensions.store.setStatus(backend.Extension{ID: "b"})
	if got := len(s.extensionsPaneRows(th)); got != empty+1 {
		t.Errorf("rows = %d with two extensions, want %d: the empty state gives way to a row each", got, empty+1)
	}
	if empty != 5 {
		t.Errorf("rows = %d with none, want the description, two install rows, the heading and the empty state", empty)
	}
}
