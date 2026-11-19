package main

import (
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
)

// composerRig is a chat whose composer has the keyboard.
func composerRig(t *testing.T) (*window, *frameRig, *chatPane) {
	t.Helper()
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	p := u.chats.pane("s1")
	r.router.Source().Execute(key.FocusCmd{Tag: &p.editor})
	r.frames(2)
	return u, r, p
}

// insert types text at the caret, over the selection, as the window system's
// edit followed by its caret move does.
func (r *frameRig) insert(p *chatPane, text string) {
	p.editor.Insert(text)
	r.now = r.now.Add(10 * time.Millisecond)
	r.frame()
}

func (r *frameRig) typeText(p *chatPane, s string) {
	for _, c := range s {
		r.insert(p, string(c))
	}
}

func (r *frameRig) chord(name key.Name, mods key.Modifiers) {
	r.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	r.now = r.now.Add(10 * time.Millisecond)
	r.frame()
}

// pause is long enough to end a run of typing.
func (r *frameRig) pause() { r.now = r.now.Add(2 * time.Second); r.frame() }

func wantComposer(t *testing.T, p *chatPane, text string, caret int, what string) {
	t.Helper()
	if got := p.editor.Text(); got != text {
		t.Errorf("%s: the composer holds %q, want %q", what, got, text)
	}
	if start, end := p.editor.Selection(); start != caret || end != caret {
		t.Errorf("%s: the caret is at %d..%d, want %d", what, start, end, caret)
	}
}

func TestUndoGroupsATypedRunAndBringsBackTheCaret(t *testing.T) {
	_, r, p := composerRig(t)
	r.typeText(p, "hello")
	r.pause()
	r.typeText(p, " world")
	wantComposer(t, p, "hello world", 11, "typed")

	r.chord("Z", key.ModShortcut)
	wantComposer(t, p, "hello", 5, "undo of the second run, which is one step, not six")
	r.chord("Z", key.ModShortcut)
	wantComposer(t, p, "", 0, "undo of the first run")
	r.chord("Z", key.ModShortcut)
	wantComposer(t, p, "", 0, "undo with nothing left")

	r.chord("Z", key.ModShortcut|key.ModShift)
	wantComposer(t, p, "hello", 5, "redo with Shift+Z")
	r.chord("Y", key.ModShortcut)
	wantComposer(t, p, "hello world", 11, "redo with Y")
}

func TestUndoBringsBackWhatWasDeletedOrPasted(t *testing.T) {
	_, r, p := composerRig(t)
	r.typeText(p, "hello")
	r.pause()
	for range 3 {
		r.chord(key.NameDeleteBackward, 0)
	}
	wantComposer(t, p, "he", 2, "three backspaces")
	r.chord("Z", key.ModShortcut)
	wantComposer(t, p, "hello", 5, "undo of the three backspaces, one step")

	r.pause()
	r.insert(p, " pasted text")
	wantComposer(t, p, "hello pasted text", 17, "pasted")
	r.chord("Z", key.ModShortcut)
	wantComposer(t, p, "hello", 5, "undo of the paste")
}

func TestUndoRestoresTheSelectionTheEditStartedFrom(t *testing.T) {
	_, r, p := composerRig(t)
	r.typeText(p, "hello world")
	r.pause()
	p.editor.SetCaret(0, 5) // "hello" selected
	r.frame()
	r.insert(p, "bye")
	if p.editor.Text() != "bye world" {
		t.Fatalf("typed over the selection: %q", p.editor.Text())
	}
	r.chord("Z", key.ModShortcut)
	if start, end := p.editor.Selection(); p.editor.Text() != "hello world" || start != 0 || end != 5 {
		t.Errorf("undo: %q with the selection %d..%d, want hello world with 0..5", p.editor.Text(), start, end)
	}
}

func TestSendingForgetsTheHistory(t *testing.T) {
	u, r, p := composerRig(t)
	r.typeText(p, "question")
	u.sendChat(p, "s1")
	r.frames(2)
	r.chord("Z", key.ModShortcut)
	wantComposer(t, p, "", 0, "undo after sending")
	r.chord("Y", key.ModShortcut)
	wantComposer(t, p, "", 0, "redo after sending")
}
