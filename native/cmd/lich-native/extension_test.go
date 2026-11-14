package main

import (
	"context"
	"encoding/json"
	"image"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func extensionEv(t *testing.T, name string, payload any) backend.Event {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return backend.Event{Name: name, Data: data}
}

func messageEv(t *testing.T, id string, msg extension.Message) backend.Event {
	t.Helper()
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return extensionEv(t, api.ExtensionMessageEvent.Name, api.ExtensionMessage{ExtensionID: id, Message: raw})
}

func TestTheExtensionStoreReadsTheListThenFollowsStatusInPlace(t *testing.T) {
	stub := &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		if method == api.ExtensionsList.Name {
			return []backend.Extension{{ID: "a", Status: api.ExtensionRunning}, {ID: "b", Status: api.ExtensionRunning}}, http.StatusOK
		}
		return failing("unexpected " + method)
	}}
	m := newTestModel(stub.client(t))
	if err := m.extensions.load(context.Background(), stub.client(t)); err != nil {
		t.Fatal(err)
	}
	m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, backend.Extension{ID: "a", Status: api.ExtensionFailed, Error: "boom"}))
	m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, backend.Extension{ID: "c", Status: api.ExtensionRunning}))
	var got []string
	for _, e := range m.extensions.all() {
		got = append(got, e.ID+":"+e.Status)
	}
	if want := []string{"a:failed", "b:running", "c:running"}; !slices.Equal(got, want) {
		t.Errorf("list = %v, want %v", got, want)
	}
}

func TestARenderLandsInTheStoreAndEveryOtherMessageReachesTheWindow(t *testing.T) {
	m := newTestModel(nil)
	var heard []string
	m.extensionMessage = func(id string, msg extension.Message) { heard = append(heard, id+":"+msg.Kind) }
	tree := extension.RenderData{Surface: "main", Root: extension.Node{Type: "stack"}, Shortcuts: []extension.Shortcut{{Keys: "r", Handler: "refresh"}}}
	m.extensionEvent(messageEv(t, "a", extension.Render.New(tree)))
	m.extensionEvent(messageEv(t, "a", extension.ShowToast.New(extension.ToastData{Title: "hi"})))
	r, ok := m.extensions.render(extensionKey{"a", "main"})
	if !ok || r.Root.Type != "stack" || len(r.Shortcuts) != 1 || r.Shortcuts[0].Handler != "refresh" {
		t.Errorf("render = %+v, %v; want the stack and its shortcut", r, ok)
	}
	if !slices.Equal(heard, []string{"a:toast"}) {
		t.Errorf("the window heard %v, want only the toast", heard)
	}
}

func TestSurfaceButtonsSkipInvalidExtensionsAndKeepTheListsOrder(t *testing.T) {
	list := []backend.Extension{
		{ID: "z", Status: api.ExtensionRunning, Surfaces: []backend.ExtensionSurface{{ID: "one", Title: "One", Icon: "rocket"}, {ID: "two", Title: "Two"}}},
		{ID: "bad", Status: api.ExtensionInvalid, Surfaces: []backend.ExtensionSurface{{ID: "x"}}},
		{ID: "a", Status: api.ExtensionFailed, Surfaces: []backend.ExtensionSurface{{ID: "main", Title: "Main"}}},
	}
	var got []extensionKey
	for _, b := range surfaceButtons(list) {
		got = append(got, b.key)
	}
	want := []extensionKey{{"z", "one"}, {"z", "two"}, {"a", "main"}}
	if !slices.Equal(got, want) {
		t.Errorf("buttons = %v, want %v", got, want)
	}
}

func TestTheSurfaceDotShowsRestartingAndFailed(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	for status, want := range map[string]struct {
		c  any
		ok bool
	}{
		api.ExtensionRestarting: {th.ToneWait, true},
		api.ExtensionFailed:     {th.Destructive, true},
		api.ExtensionRunning:    {nil, false},
		api.ExtensionStopped:    {nil, false},
	} {
		c, ok := surfaceDot(th, status)
		if ok != want.ok || (ok && c != want.c) {
			t.Errorf("%s: dot %v, %v; want %v, %v", status, c, ok, want.c, want.ok)
		}
	}
}

func TestASurfaceIconTheSetLacksIsThePuzzle(t *testing.T) {
	if surfaceGlyph("") != icExtension || surfaceGlyph("no-such-icon") != icExtension {
		t.Error("a missing or unknown icon should draw the puzzle")
	}
	if ic := surfaceGlyph("rocket"); ic == nil || ic == icExtension {
		t.Error("a known icon should draw itself")
	}
}

func TestTheScreensStateFollowsStatusAndTheFirstRender(t *testing.T) {
	cases := []struct {
		status string
		tree   bool
		want   extensionView
	}{
		{api.ExtensionRunning, false, extensionStarting},
		{api.ExtensionRunning, true, extensionLive},
		{api.ExtensionRestarting, true, extensionRestarting},
		{api.ExtensionRestarting, false, extensionStarting},
		{api.ExtensionFailed, true, extensionFailed},
		{api.ExtensionInvalid, false, extensionFailed},
	}
	for _, c := range cases {
		if got := extensionViewOf(c.status, c.tree); got != c.want {
			t.Errorf("%s, tree %v: %v, want %v", c.status, c.tree, got, c.want)
		}
	}
}

// sentLog records what a screen or a toast sends, as kind:surface or
// kind:handler.
type sentLog struct {
	mu   sync.Mutex
	sent []string
}

func (l *sentLog) send(id string, msg extension.Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	line := id + " " + msg.Kind
	switch d := msg.Data.(type) {
	case extension.SurfaceData:
		line += " " + d.Surface
	case extension.EventData:
		line += " " + string(d.Handler)
	case extension.ContextData:
		line += " " + d.Path
	}
	l.sent = append(l.sent, line)
}

func TestOpeningAndLeavingSurfacesTellsTheirExtensions(t *testing.T) {
	var log sentLog
	s := newExtensionScreen(&extensionStore{}, nil, log.send)
	s.show(extensionKey{"a", "main"})
	s.show(extensionKey{"a", "main"})
	s.show(extensionKey{"b", "board"})
	s.hide()
	s.hide()
	want := []string{"a surface.opened main", "a surface.closed main", "b surface.opened board", "b surface.closed board"}
	if !slices.Equal(log.sent, want) {
		t.Errorf("sent %v, want %v", log.sent, want)
	}
	if s.isShown(extensionKey{"b", "board"}) {
		t.Error("a hidden surface is still shown")
	}
}

func TestAnExtensionToastMapsEveryField(t *testing.T) {
	var log sentLog
	got := extensionToast("a", extension.ToastData{
		ID: "sync", Variant: extension.ToastWarning, Title: "T", Description: "D",
		Action: &extension.ToastButton{Label: "Go", OnPress: "go"}, Cancel: &extension.ToastButton{Label: "No"},
		Icon: "rocket", IconColor: "destructive", Spin: true,
		Lines:    []extension.ToastLine{{Icon: "nope", Text: "line"}},
		Progress: &extension.ToastProgress{Percent: 40}, DurationMs: 1500, OnDismiss: "gone",
	}, log.send)
	if got.ID != "extension:a:sync" || got.Variant != ui.ToastWarning || got.Title != "T" || got.Description != "D" || !got.Spin {
		t.Errorf("toast = %+v", got)
	}
	if got.Icon == nil || got.IconColor == nil || got.Duration != 1500*time.Millisecond || got.Progress.Percent != 40 {
		t.Errorf("icon, color, duration or progress lost: %+v", got)
	}
	if len(got.Lines) != 1 || got.Lines[0].Text != "line" || got.Lines[0].Icon != nil {
		t.Errorf("lines = %+v, want one with no glyph for an unknown icon", got.Lines)
	}
	if got.Cancel.OnClick != nil || got.Secondary != nil {
		t.Error("a button with no handler fires nothing, and an absent one stays absent")
	}
	got.Action.OnClick()
	got.OnDismiss()
	if want := []string{"a event go", "a event gone"}; !slices.Equal(log.sent, want) {
		t.Errorf("sent %v, want %v", log.sent, want)
	}
	if forever := extensionToast("a", extension.ToastData{DurationMs: extension.ToastForever}, log.send); forever.Duration != ui.ToastForever || forever.ID != "" {
		t.Errorf("forever toast = %+v, want no ID and no end", forever)
	}
}

func TestTheWindowRaisesDismissesAndCopiesForAnExtension(t *testing.T) {
	var log sentLog
	u := &window{model: newTestModel(nil), extOutbox: newExtensionOutbox()}
	u.extension = newExtensionScreen(&u.model.extensions, nil, log.send)
	u.extensionMessage("a", extension.ShowToast.New(extension.ToastData{ID: "t", Title: "hi"}))
	if !u.toaster.Live("extension:a:t") {
		t.Fatal("the extension's toast is not up")
	}
	u.extensionMessage("a", extension.DismissToast.New(extension.DismissToastData{ID: "t"}))
	if u.toaster.Live("extension:a:t") {
		t.Error("toast.dismiss left the toast up")
	}
	u.extensionMessage("a", extension.Clipboard.New(extension.ClipboardData{Text: "copied"}))
	if text := u.extension.clip.Load(); text == nil || *text != "copied" {
		t.Errorf("pending copy = %v, want copied", text)
	}
}

func TestTheOutboxDeliversInTheOrderItWasSent(t *testing.T) {
	o := newExtensionOutbox()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan string, 8)
	go o.run(ctx, func(_ context.Context, id string, msg extension.Message) error {
		got <- msg.Data.(extension.EventData).Text
		return nil
	}, func(error) {})
	for _, text := range []string{"a", "ab", "abc"} {
		o.send("x", extension.FireEvent.New(extension.EventData{Handler: "type", Text: text}))
	}
	for _, want := range []string{"a", "ab", "abc"} {
		select {
		case text := <-got:
			if text != want {
				t.Fatalf("delivered %q, want %q", text, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the outbox never delivered")
		}
	}
}

func TestParseShortcut(t *testing.T) {
	cases := map[string]struct {
		want combo
		ok   bool
	}{
		"r":            {combo{Key: "r"}, true},
		"shift+g":      {combo{Shift: true, Key: "g"}, true},
		"ctrl+k":       {combo{Mod: true, Key: "k"}, true},
		"Ctrl+Shift+P": {combo{Mod: true, Shift: true, Key: "p"}, true},
		"alt+enter":    {combo{Alt: true, Key: "Enter"}, true},
		"escape":       {combo{Key: "Escape"}, true},
		"space":        {combo{Key: " "}, true},
		"hyper+k":      {combo{}, false},
		"ctrl+nosuch":  {combo{}, false},
		"":             {combo{}, false},
	}
	for keys, c := range cases {
		got, ok := parseShortcut(keys)
		if ok != c.ok || got != c.want {
			t.Errorf("%q: %+v, %v; want %+v, %v", keys, got, ok, c.want, c.ok)
		}
	}
}

// shortcutRig polls an extension's shortcuts the way the screen does, with a
// field that holds the focus when typing.
type shortcutRig struct {
	router input.Router
	field  bool
}

func (r *shortcutRig) frame(list []extension.Shortcut, typing bool) []extension.Handler {
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: r.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(100, 100))}
	fired := pollShortcuts(gtx, list, !gtx.Source.Focused(nil))
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: &r.field}); !ok {
			break
		}
	}
	if typing {
		gtx.Execute(key.FocusCmd{Tag: &r.field})
	} else {
		gtx.Execute(key.FocusCmd{})
	}
	event.Op(ops, &r.field)
	r.router.Frame(ops)
	return fired
}

func TestAPlainShortcutWaitsWhileAFieldTypesAndACtrlChordDoesNot(t *testing.T) {
	list := []extension.Shortcut{{Keys: "r", Handler: "refresh"}, {Keys: "ctrl+k", Handler: "palette"}, {Keys: "shift+g", Handler: "bottom"}}
	r := new(shortcutRig)
	r.frame(list, false)
	r.frame(list, false)
	r.router.Queue(chord("R", 0), chord("K", key.ModCtrl), chord("G", key.ModShift))
	if got := r.frame(list, false); !slices.Equal(got, []extension.Handler{"refresh", "palette", "bottom"}) {
		t.Errorf("with nothing focused fired %v, want all three", got)
	}
	r.frame(list, true)
	r.frame(list, true)
	r.router.Queue(chord("R", 0), chord("K", key.ModCtrl), chord("G", key.ModShift))
	if got := r.frame(list, true); !slices.Equal(got, []extension.Handler{"palette"}) {
		t.Errorf("while a field types fired %v, want only the ctrl chord", got)
	}
}

// pressScreen lays the screen out, clicks its top-left corner and lays it out
// again, as two frames of a window would.
func pressScreen(s *extensionScreen) {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
	router := new(input.Router)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300)), Source: router.Source()}
		s.layout(gtx, th, false)
		router.Frame(&ops)
	}
	frame()
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(5, 5)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(5, 5)},
	)
	frame()
}

func TestAPressOnALiveTreeIsAnEventAndARestartingOneTakesNone(t *testing.T) {
	store := &extensionStore{list: []backend.Extension{{ID: "a", Name: "Board", Status: api.ExtensionRunning}}}
	tree := extension.Box.New(extension.BoxProps{}, extension.Button.New(extension.ButtonProps{Text: "Run", OnPress: "run"}))
	store.setRender("a", extension.RenderData{Surface: "main", Root: tree})
	var log sentLog
	s := newExtensionScreen(store, nil, log.send)
	s.show(extensionKey{"a", "main"})
	pressScreen(s)
	if want := []string{"a surface.opened main", "a event run"}; !slices.Equal(log.sent, want) {
		t.Fatalf("sent %v, want %v", log.sent, want)
	}
	store.setStatus(backend.Extension{ID: "a", Name: "Board", Status: api.ExtensionRestarting})
	pressScreen(s)
	if len(log.sent) != 2 {
		t.Errorf("a restarting tree sent %v", log.sent[2:])
	}
}
