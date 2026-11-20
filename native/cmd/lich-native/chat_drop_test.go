package main

import (
	"image"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// fileURI is path as a file manager lists it in a drag's text/uri-list.
func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String() + "\r\n"
}

// A file and an image dropped on a session drawn as a chat land in its
// composer as the terminal would paste them, quoted where they need it and
// at the caret, after what was already typed; the drag over it shows the
// terminal's hint. An image is a path too: the chat sends the agent text.
func TestFilesDroppedOnAChatLandInItsComposer(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	dir := t.TempDir()
	file := writeFile(t, filepath.Join(dir, "notes.md"), "# notes")
	shot := writeFile(t, filepath.Join(dir, "screen shot.png"), "\x89PNG")
	p := u.chats.pane("s1")
	p.editor.SetText("look at ")
	p.editor.SetCaret(p.editor.Len(), p.editor.Len())
	u.drop.add("s1", image.Rectangle{Max: r.size})
	u.onDrag(enter(300, 300, uriListMIME), r.invalidate)
	if !u.drop.hinting("s1") {
		t.Error("a file dragged over the chat does not light the drop hint")
	}
	u.onDrag(drop(300, 300, fileURI(file)+fileURI(shot)), r.invalidate)
	want := "look at " + file + " '" + shot + "' "
	for deadline := time.Now().Add(5 * time.Second); p.editor.Text() != want; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the composer reads %q, want %q", p.editor.Text(), want)
		}
		r.now = r.now.Add(frameStep)
		r.frame()
	}
	if strings.ContainsAny(p.editor.Text(), "\x1b") {
		t.Errorf("the composer took the paste's brackets: %q", p.editor.Text())
	}
	if u.drop.hinting("s1") {
		t.Error("the hint stays up after the drop")
	}
}
