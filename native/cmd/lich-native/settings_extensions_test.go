package main

import (
	"encoding/json"
	"image"
	"net/http"
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
		if method == api.ExtensionsRemove.Name {
			return api.None{}, http.StatusOK
		}
		return backend.Extension{ID: "x"}, http.StatusOK
	}}
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
	s.extensions.from.Value = installFromGit
	return s
}

func TestInstallAsksFirstAndNamesTheSourceInView(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{})
	p := &s.extensions
	p.askInstall()
	if p.snapshot().asking != "" {
		t.Fatal("an empty field asked to install")
	}
	p.url.SetText("  https://github.com/o/r  ")
	p.dir.SetText("/home/me/ext")
	p.with(func(t *extensionTasks) { t.err = "old" })
	p.askInstall()
	if got := p.snapshot(); got.asking != "https://github.com/o/r" || got.err != "" {
		t.Errorf("from GitHub: %+v, want the URL asked and the old error gone", got)
	}
	p.from.Value = installFromDir
	p.askInstall()
	if got := p.snapshot().asking; got != "/home/me/ext" {
		t.Errorf("from a folder: asking %q, want the folder", got)
	}
}

func TestConfirmedInstallCallsTheRightMethodAndShowsBusy(t *testing.T) {
	cases := []struct {
		fromDir    bool
		src, want  string
		wantMethod string
	}{
		{false, "https://github.com/o/r", `{"url":"https://github.com/o/r"}`, api.ExtensionsInstallFromGit.Name},
		{true, "/home/me/ext", `{"dir":"/home/me/ext"}`, api.ExtensionsInstallFromDir.Name},
	}
	for _, c := range cases {
		b := &extensionBackend{}
		s := extensionsScreen(t, b)
		p := &s.extensions
		p.with(func(t *extensionTasks) { t.asking = c.src })
		s.installExtension(c.fromDir, c.src)
		if got := p.snapshot(); !got.installing || got.asking != "" {
			t.Errorf("%s: %+v, want busy and the dialog shut", c.wantMethod, got)
		}
		eventually(t, "the install finished", func() bool { return !p.snapshot().installing })
		if method, args := b.last(); method != c.wantMethod || args != c.want {
			t.Errorf("called %s %s, want %s %s", method, args, c.wantMethod, c.want)
		}
		if p.snapshot().err != "" {
			t.Errorf("%s: error %q after a success", c.wantMethod, p.snapshot().err)
		}
	}
}

func TestAFailedInstallLeavesTheBackendsWordsUnderTheBox(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{fail: "no release for linux_amd64"})
	p := &s.extensions
	p.url.SetText("https://github.com/o/r")
	s.installExtension(false, "https://github.com/o/r")
	eventually(t, "the install finished", func() bool { return !p.snapshot().installing })
	if got := p.snapshot().err; got != "no release for linux_amd64" {
		t.Errorf("error = %q, want the backend's words", got)
	}
	if p.url.Text() == "" {
		t.Error("a failed install emptied the field")
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

func TestTheEmptyListIsOneEmptyStateAndAListIsOneRowEach(t *testing.T) {
	s := extensionsScreen(t, &extensionBackend{})
	th := testTheme()
	empty := len(s.extensionsPaneRows(th))
	s.extensions.store.setStatus(backend.Extension{ID: "a"})
	s.extensions.store.setStatus(backend.Extension{ID: "b"})
	if got := len(s.extensionsPaneRows(th)); got != empty+2 {
		t.Errorf("rows = %d with two extensions, want %d: the empty state gives way to a spacer and a row each", got, empty+2)
	}
	if empty != 3 {
		t.Errorf("rows = %d with none, want the header, the box and the empty state", empty)
	}
}
