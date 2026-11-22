package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/transfer"

	"github.com/lichdotdev/lich/api"
)

// pasteChord queues Ctrl+V (Cmd+V on macOS) and frames it.
func (r *frameRig) pasteChord() {
	r.router.Queue(key.Event{Name: "V", Modifiers: key.ModShortcut, State: key.Press}, key.Event{Name: "V", Modifiers: key.ModShortcut, State: key.Release})
	r.frame()
}

// clipboardText answers the window's clipboard read with text and frames it.
func (r *frameRig) clipboardText(t *testing.T, text string) {
	t.Helper()
	if !r.router.ClipboardRequested() {
		t.Fatal("the paste read no clipboard")
	}
	r.router.Queue(transfer.DataEvent{Type: "application/text", Open: func() io.ReadCloser { return io.NopCloser(strings.NewReader(text)) }})
	r.frame()
}

// An image on the clipboard pasted into a chat's composer lands as a card,
// its copy stored through /drop, the text left as it was.
func TestAPastedImageLandsAsACard(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	f.answers = map[string]any{"/drop": map[string]string{"path": "/cfg/dropped/s1/pasted-image.png", "notice": "[lich] copy of pasted-image.png"}}
	u.chats.readImage = func(context.Context) (clipImage, error) { return clipImage{data: []byte("\x89PNG"), ext: ".png"}, nil }
	p := u.chats.pane("s1")
	p.editor.SetText("look ")
	r.settle(t)
	r.pasteChord()
	r.clipboardText(t, "")
	waitCards(t, r, p, "/cfg/dropped/s1/pasted-image.png")
	if p.attached[0].Notice != "" {
		t.Errorf("the card carries the notice %q, for a copy with no original", p.attached[0].Notice)
	}
	if p.editor.Text() != "look " {
		t.Errorf("the composer reads %q, want the text it had", p.editor.Text())
	}
	f.waitCall(t, "/drop")
}

// Text on the clipboard pastes at the caret as the editor always has, with
// no card.
func TestPastedTextStillTypes(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	read := make(chan struct{}, 1)
	u.chats.readImage = func(context.Context) (clipImage, error) { read <- struct{}{}; return clipImage{}, nil }
	p := u.chats.pane("s1")
	r.settle(t)
	r.pasteChord()
	r.clipboardText(t, "hello")
	if p.editor.Text() != "hello" {
		t.Errorf("the composer reads %q, want the pasted text", p.editor.Text())
	}
	<-read
	r.frames(2)
	if len(p.attached) != 0 {
		t.Errorf("a text paste added cards %q", filePaths(p.attached))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.HasPrefix(c, "/drop") {
			t.Errorf("a text paste uploaded %s", c)
		}
	}
}

// A machine with no tool to read an image says so once, not on every paste.
func TestAPasteWithNoToolSaysSoOnce(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	reads := make(chan struct{}, 2)
	u.chats.readImage = func(context.Context) (clipImage, error) { reads <- struct{}{}; return clipImage{}, errNoClipboardTool }
	u.toaster.Invalidate = r.invalidate
	r.settle(t)
	paste := func() {
		r.pasteChord()
		r.clipboardText(t, "")
		<-reads
		time.Sleep(20 * time.Millisecond)
	}
	paste()
	if !u.toaster.Live(pasteToolToast) {
		t.Fatal("a paste with no tool said nothing")
	}
	u.toaster.Dismiss(pasteToolToast)
	paste()
	if u.toaster.Live(pasteToolToast) {
		t.Error("the second paste said it again")
	}
}
