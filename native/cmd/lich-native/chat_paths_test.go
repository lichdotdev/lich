package main

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui/filetree"
)

const pathsCheckout = "/repo"

var pathsListing = api.FileListing{Files: []string{"app.go", "youtrack/client_test.go", "youtrack/api/client.go"}}

func TestAPathInAnAnswerLinksWhatTheCheckoutHolds(t *testing.T) {
	f := newChatFiles(pathsListing.Files)
	for written, want := range map[string]bool{
		"app.go":                  true,
		"./app.go":                true,
		"/repo/app.go":            true,
		"youtrack/":               true,
		"youtrack":                true,
		"youtrack/client_test.go": true,
		"client_test.go":          true, // the tail of one file, as a terminal's path:line
		"api/":                    false,
		"gone.go":                 false,
		"../app.go":               false,
		"/elsewhere/app.go":       false,
		"~/app.go":                false,
	} {
		if got := f.links(pathsCheckout, written); got != want {
			t.Errorf("%q links %v, want %v", written, got, want)
		}
	}
}

// pathFrames is s1 in pathsCheckout, which lists pathsListing, with one
// answer that says text, the listing read.
func pathFrames(t *testing.T, text string) (*window, *frameRig, *chatBackend) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	f.answers = map[string]any{api.ProjectTree.Name: pathsListing}
	u.model.projects[0].Path = pathsCheckout
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: text}})
	r.frame()
	f.waitCall(t, api.ProjectTree.Name)
	r.settle(t)
	return u, r, f
}

// holdLinkKey presses the link key, or lets it go, with the composer holding
// the keyboard.
func (r *frameRig) holdLinkKey(held bool) {
	state, mods := key.Release, key.Modifiers(0)
	if held {
		state, mods = key.Press, key.ModShortcut
	}
	r.router.Queue(key.Event{Name: linkKey(runtime.GOOS), State: state, Modifiers: mods})
	r.frame()
}

// The link key is heard while the composer holds the keyboard: a chat has
// no terminal grid to read it.
func TestTheLinkKeyIsHeardOverTheComposer(t *testing.T) {
	u, r, _ := pathFrames(t, "Edit `app.go`.")
	r.holdLinkKey(true)
	if !u.terms.linkKeyHeld {
		t.Fatal("the link key held over the composer is not heard")
	}
	r.holdLinkKey(false)
	if u.terms.linkKeyHeld {
		t.Error("the link key let go is still held")
	}
}

// handOver is a point of the answer's first line, under the who line and
// over the copy button, where the pointer takes a hand, the link key held: a
// path that links. Prose has no semantics to find it by.
func handOver(t *testing.T, r *frameRig, mods key.Modifiers) (f32.Point, bool) {
	t.Helper()
	who := r.glyph(t, "claude")
	for y := who.Y + 16; y < who.Y+48; y += 6 {
		for x := who.X - 48; x < who.X+480; x += 6 {
			at := f32.Pt(float32(x), float32(y))
			r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at, Modifiers: mods})
			r.frame()
			if r.router.Cursor() == pointer.CursorPointer {
				return at, true
			}
		}
	}
	return f32.Point{}, false
}

// clickWith clicks at with mods held.
func (r *frameRig) clickWith(at f32.Point, mods key.Modifiers) {
	r.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at, Modifiers: mods},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at, Modifiers: mods},
	)
	r.frame()
}

func (f *chatBackend) callsOf(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if m, _, _ := strings.Cut(c, " "); m == method {
			n++
		}
	}
	return n
}

// A file an answer names opens on a Ctrl+click the way a path clicked in a
// terminal does, and a plain click leaves it as text.
func TestCtrlClickOnAFileInAnAnswerFollowsIt(t *testing.T) {
	u, r, f := pathFrames(t, "Edit `app.go` now.")
	r.holdLinkKey(true)
	at, ok := handOver(t, r, key.ModShortcut)
	if !ok {
		t.Fatalf("no hand over the path with the link key held:\n%s", r.drawn())
	}
	reads := f.callsOf(api.ProjectTree.Name)
	r.clickWith(at, 0)
	time.Sleep(50 * time.Millisecond)
	if f.callsOf(api.ProjectTree.Name) > reads || u.codeAsk.Load() != nil {
		t.Fatal("a plain click on the path followed it")
	}
	r.clickWith(at, key.ModShortcut)
	for deadline := time.Now().Add(5 * time.Second); u.codeAsk.Load() == nil; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("a Ctrl+click on the path opened nothing")
		}
	}
}

// A path the checkout does not hold is no link: no hand over it.
func TestAPathTheCheckoutLacksIsNoLink(t *testing.T) {
	_, r, _ := pathFrames(t, "Edit `gone.go` now.")
	r.holdLinkKey(true)
	if at, ok := handOver(t, r, key.ModShortcut); ok {
		t.Errorf("a hand at %v over a path the checkout lacks", at)
	}
}

// The hand and the link wait for the link key, as a terminal's path does.
func TestAPathTakesNoHandWithoutTheLinkKey(t *testing.T) {
	_, r, _ := pathFrames(t, "Edit `app.go` now.")
	if at, ok := handOver(t, r, 0); ok {
		t.Errorf("a hand at %v over a path with the link key up", at)
	}
}

// A link key let go where the chat did not hear it, on the stage editor a
// click opened, is let go for the chat once the pointer says it is up.
func TestALinkKeyLetGoUnheardTakesNoHand(t *testing.T) {
	u, r, _ := pathFrames(t, "Edit `app.go` now.")
	u.terms.linkKeyHeld = true
	if at, ok := handOver(t, r, 0); ok {
		t.Errorf("a hand at %v over a path with the link key up", at)
	}
}

// A folder an answer names opens in the dock's Code tab, unfolded down to
// it.
func TestCtrlClickOnAFolderInAnAnswerUnfoldsItInTheCodeTab(t *testing.T) {
	u, r, _ := pathFrames(t, "Look in `youtrack/api/` now.")
	u.dock = &dock{code: &codeTab{toggled: map[string]bool{}}}
	u.model.active = "s1" // the session on screen, as the one clicked in is
	r.holdLinkKey(true)
	at, ok := handOver(t, r, key.ModShortcut)
	if !ok {
		t.Fatalf("no hand over the folder with the link key held:\n%s", r.drawn())
	}
	r.clickWith(at, key.ModShortcut)
	if u.dock.tab != dockCode {
		t.Fatalf("a Ctrl+click on the folder did not open the Code tab")
	}
	rows := filetree.VisibleRows(filetree.Build(pathsListing.Files), func(p string) bool { return u.dock.code.toggled[p] })
	shown := map[string]bool{}
	for _, row := range rows {
		shown[row.Node.Path] = true
	}
	if !shown["youtrack/api/client.go"] {
		t.Errorf("the folder is not unfolded down to its files: %v", shown)
	}
	if u.dock.code.checkout != pathsCheckout {
		t.Errorf("the Code tab is on %q, want the session's checkout", u.dock.code.checkout)
	}
}
