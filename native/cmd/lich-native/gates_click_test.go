package main

import (
	"encoding/json"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/coder/websocket"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// gateBackend answers every RPC the startup gates make and reports each one
// as "method args". It also answers /ping and holds /events and /ws open, so
// a window can attach to it and lose it again.
type gateBackend struct {
	calls  chan string
	state  []backend.Project
	answer map[string]any
	srv    *httptest.Server

	// refuseEvents and refuseTerminal make that socket's upgrade fail.
	refuseEvents, refuseTerminal atomic.Bool
	// eventsOpened and terminalOpened count the sockets a window attached;
	// socketsLive those of them still open.
	eventsOpened, terminalOpened, socketsLive atomic.Int32
	// failing holds the RPC methods that answer with an error.
	failing sync.Map

	dropEvents     chan struct{}
	dropEventsOnce sync.Once
}

func newGateBackend(t *testing.T) *gateBackend {
	t.Helper()
	b := &gateBackend{calls: make(chan string, 64), answer: map[string]any{}, dropEvents: make(chan struct{}),
		state: []backend.Project{{ID: "p", Name: "repo", Path: "/repo"}}}
	b.srv = httptest.NewServer(http.HandlerFunc(b.serve))
	t.Cleanup(b.srv.Close)
	return b
}

func (b *gateBackend) serve(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ping":
		w.WriteHeader(http.StatusNoContent)
		return
	case "/events":
		b.hold(w, r, &b.refuseEvents, &b.eventsOpened, b.dropEvents)
		return
	case "/ws":
		b.hold(w, r, &b.refuseTerminal, &b.terminalOpened, nil)
		return
	}
	body, _ := io.ReadAll(r.Body)
	method := strings.TrimPrefix(r.URL.Path, "/rpc/")
	b.calls <- method + " " + string(body)
	if _, fails := b.failing.Load(method); fails {
		http.Error(w, `{"error":"failing"}`, http.StatusInternalServerError)
		return
	}
	result := b.answer[method]
	if method == "store.LoadState" {
		result = b.state
	}
	_ = json.NewEncoder(w).Encode(result)
}

// hold keeps a socket open until the client leaves or drop closes.
func (b *gateBackend) hold(w http.ResponseWriter, r *http.Request, refuse *atomic.Bool, opened *atomic.Int32, drop <-chan struct{}) {
	if refuse.Load() {
		http.Error(w, "refused", http.StatusServiceUnavailable)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	opened.Add(1)
	b.socketsLive.Add(1)
	defer b.socketsLive.Add(-1)
	gone := conn.CloseRead(r.Context())
	select {
	case <-drop:
		conn.CloseNow()
	case <-gone.Done():
	}
}

// dropEventStream ends /events under the window, as a backend that gave its
// sockets to another window does.
func (b *gateBackend) dropEventStream() { b.dropEventsOnce.Do(func() { close(b.dropEvents) }) }

// goAway stops the backend the way a quit does: nothing answers, and the
// sockets end.
func (b *gateBackend) goAway() {
	b.srv.Close()
	b.dropEventStream()
}

func newGateWindow(t *testing.T) (*window, *gateBackend) {
	t.Helper()
	b := newGateBackend(t)
	client := clientOf(b.srv)
	u := &window{ctx: t.Context(), client: client, win: new(app.Window),
		theme: &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Canvas}}
	u.model = newModel(client, func() {})
	u.model.launched, u.model.projects = true, b.state
	u.model.setActiveProject("p")
	u.terms = &terminals{client: client, byID: map[string]*termView{}}
	u.settings = newSettingsScreen(u.ctx, client, &u.root, func() {})
	u.settings.theme = &themePicker{toaster: &u.toaster}
	return u, b
}

// await is the next call to method, failing the test when none comes.
func (b *gateBackend) await(t *testing.T, method string) string {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case call := <-b.calls:
			if args, ok := strings.CutPrefix(call, method+" "); ok {
				return args
			}
		case <-deadline:
			t.Fatalf("no %s call", method)
			return ""
		}
	}
}

// dialogDriver frames one dialog through a real input router, the way the
// kit's dialog rig does, and presses its buttons from the keyboard: opening
// focuses the panel, and Tab walks its controls.
type dialogDriver struct {
	router input.Router
	ops    op.Ops
	now    time.Time
	draw   func(gtx C)
}

func (d *dialogDriver) frame() {
	d.ops.Reset()
	gtx := layout.Context{Ops: &d.ops, Now: d.now, Source: d.router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(1000, 700))}
	d.draw(gtx)
	d.router.Frame(&d.ops)
}

// open runs the dialog's transition in to its end.
func (d *dialogDriver) open() {
	d.router.Queue(key.FocusEvent{})
	d.frame()
	d.now = d.now.Add(time.Second)
	d.frame()
}

// maxFocusStops bounds the Tab walk to a button: far more than any gate's
// dialog holds.
const maxFocusStops = 16

// press walks the focus to button with Tab and presses it with Space.
func (d *dialogDriver) press(t *testing.T, button *widget.Clickable) {
	t.Helper()
	for range maxFocusStops {
		d.router.MoveFocus(key.FocusForward)
		d.frame()
		if d.router.Source().Focused(button) {
			d.router.Queue(key.Event{Name: key.NameSpace, State: key.Press}, key.Event{Name: key.NameSpace, State: key.Release})
			d.frame()
			return
		}
	}
	t.Fatal("Tab never reached the button")
}

func gitDialog(u *window) *dialogDriver {
	u.gitMissing.missing.Store(true)
	return &dialogDriver{now: time.Unix(0, 0), draw: func(gtx C) { u.layoutGitMissing(gtx, u.theme) }}
}

func TestGitMissingNotNowClosesTheDialog(t *testing.T) {
	u, _ := newGateWindow(t)
	d := gitDialog(u)
	d.open()
	d.press(t, &u.gitMissing.notNow)
	if u.gitMissing.missing.Load() {
		t.Error("Not now left the dialog up")
	}
}

func TestGitMissingCheckAgainRereadsThePathThenVerifies(t *testing.T) {
	u, b := newGateWindow(t)
	b.answer["providers.Verify"] = backend.BinaryCheck{Path: "/usr/bin/git", Status: backend.CheckOK}
	d := gitDialog(u)
	d.open()
	d.press(t, &u.gitMissing.again)
	b.await(t, "providers.RefreshPath")
	if args := b.await(t, "providers.Verify"); args != `["git"]` {
		t.Errorf("verified %s, want git", args)
	}
	for u.gitMissing.checking.Load() {
		time.Sleep(time.Millisecond)
	}
	if u.gitMissing.missing.Load() {
		t.Error("git found on the second look and the dialog is still up")
	}
}

func TestGitMissingInstallOpensTheDownloadPage(t *testing.T) {
	u, b := newGateWindow(t)
	d := gitDialog(u)
	d.open()
	d.press(t, &u.gitMissing.install)
	if args := b.await(t, "system.OpenExternal"); args != `["`+vcsGitURL+`"]` {
		t.Errorf("opened %s, want %s", args, vcsGitURL)
	}
	if u.gitMissing.missing.Load() {
		t.Error("Install git left the dialog up")
	}
}

func pluginDialog(u *window) *dialogDriver {
	u.plugin.started = true
	u.plugin.offer = []backend.PluginStatus{{Provider: "claude", Name: "Claude Code"}, {Provider: "codex", Name: "Codex"}}
	return &dialogDriver{now: time.Unix(0, 0), draw: func(gtx C) { u.layoutPluginGate(gtx, u.theme) }}
}

func pluginOffered(u *window) bool {
	u.plugin.mu.Lock()
	defer u.plugin.mu.Unlock()
	return len(u.plugin.offer) > 0
}

func TestPluginGateDontAskRemembersAndCloses(t *testing.T) {
	u, b := newGateWindow(t)
	d := pluginDialog(u)
	d.open()
	d.press(t, &u.plugin.dontAsk)
	if args := b.await(t, "store.SetSetting"); args != `["`+pluginInstallDismissedKey+`","","1"]` {
		t.Errorf("wrote %s, want the install dismissed", args)
	}
	if pluginOffered(u) {
		t.Error("Don't ask again left the dialog up")
	}
}

func TestPluginGateNotNowClosesWithoutRemembering(t *testing.T) {
	u, b := newGateWindow(t)
	d := pluginDialog(u)
	d.open()
	d.press(t, &u.plugin.notNow)
	if pluginOffered(u) {
		t.Error("Not now left the dialog up")
	}
	select {
	case call := <-b.calls:
		t.Errorf("Not now called %s", call)
	default:
	}
}

func TestPluginGateInstallInstallsEveryTickedCLI(t *testing.T) {
	u, b := newGateWindow(t)
	d := pluginDialog(u)
	d.open()
	d.press(t, &u.plugin.install)
	for _, want := range []string{`["claude"]`, `["codex"]`} {
		if args := b.await(t, "agentplugin.Install"); args != want {
			t.Errorf("installed %s, want %s", args, want)
		}
	}
	for pluginOffered(u) {
		time.Sleep(time.Millisecond)
	}
}

func TestUpdateToastLaterRemembersTheRelease(t *testing.T) {
	u, b := newGateWindow(t)
	toast := u.appUpdateToast(backend.AppUpdateStatus{UpdateAvailable: true, LatestVersion: "1.4.0", ReleaseURL: "https://r"})
	toast.Cancel.OnClick()
	if args := b.await(t, "store.SetSetting"); args != `["`+appUpdateDismissedKey+`","","1.4.0"]` {
		t.Errorf("wrote %s, want 1.4.0 dismissed", args)
	}
}

func TestUpdateToastViewReleaseOpensItsPage(t *testing.T) {
	u, b := newGateWindow(t)
	toast := u.appUpdateToast(backend.AppUpdateStatus{LatestVersion: "1.4.0", ReleaseURL: "https://r", InstallCommand: "yay -S lich"})
	toast.Secondary.OnClick()
	if args := b.await(t, "system.OpenExternal"); args != `["https://r"]` {
		t.Errorf("opened %s, want the release page", args)
	}
}

func TestUpdateToastInstallTypesTheCommandIntoANewShell(t *testing.T) {
	u, b := newGateWindow(t)
	toast := u.appUpdateToast(backend.AppUpdateStatus{LatestVersion: "1.4.0", ReleaseURL: "https://r", InstallCommand: "yay -S lich"})
	toast.Action.OnClick()
	args := b.await(t, "store.AddSessionInFolder")
	var call []any
	if err := json.Unmarshal([]byte(args), &call); err != nil || call[0] != "p" || call[3] != "shell" {
		t.Fatalf("added %s, want a shell in the project in view", args)
	}
	b.await(t, "store.SetActiveSession")
	u.terms.mu.Lock()
	defer u.terms.mu.Unlock()
	if typed := u.terms.typeahead[call[1].(string)]; typed != "yay -S lich" {
		t.Errorf("queued %q for the shell, want the install command", typed)
	}
}

func TestUpdateToastUpdateAndInstallRunsTheInstaller(t *testing.T) {
	u, b := newGateWindow(t)
	toast := u.appUpdateToast(backend.AppUpdateStatus{LatestVersion: "1.4.0", CanSelfApply: true})
	toast.Action.OnClick()
	b.await(t, "appupdate.Apply")
}

func TestPluginUpdateToastButtons(t *testing.T) {
	u, b := newGateWindow(t)
	toast := u.pluginUpdateToast(pluginGateAction{kind: pluginActionUpdate, version: "0.9.0",
		providers: []backend.PluginStatus{{Provider: "claude"}}})
	toast.Action.OnClick()
	if args := b.await(t, "agentplugin.Update"); args != `["claude"]` {
		t.Errorf("updated %s, want claude", args)
	}
	toast.Cancel.OnClick()
	if args := b.await(t, "store.SetSetting"); args != `["`+pluginUpdateDismissedKey+`","","0.9.0"]` {
		t.Errorf("wrote %s, want 0.9.0 dismissed", args)
	}
}

func TestUncleanExitOpenLogRevealsTheLog(t *testing.T) {
	u, b := newGateWindow(t)
	toast := uncleanExitToast(u.settings.revealLog)
	toast.Action.OnClick()
	b.await(t, "system.RevealLog")
	if toast.Cancel == nil || toast.Cancel.OnClick != nil {
		t.Error("Dismiss should only take the toast down")
	}
}

func TestPluginGateUntickedCLIIsLeftOut(t *testing.T) {
	u, b := newGateWindow(t)
	d := pluginDialog(u)
	d.open()
	d.press(t, u.plugin.rows["claude"])
	d.press(t, &u.plugin.install)
	if args := b.await(t, "agentplugin.Install"); args != `["codex"]` {
		t.Errorf("installed %s, want only the CLI still ticked", args)
	}
}
