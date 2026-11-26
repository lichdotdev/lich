package main

import (
	"image"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// fileURI is path as a file manager lists it in a drag's text/uri-list.
func fileURI(path string) string {
	if filepath.VolumeName(path) != "" {
		path = string(filepath.Separator) + path
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String() + "\r\n"
}

// A file and an image dropped on a session drawn as a chat land in its
// composer as cards, in drop order, leaving the text as it was; the drag over
// it is followed, for the composer to show.
func TestFilesDroppedOnAChatLandInItsComposer(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	dir := t.TempDir()
	file := writeFile(t, filepath.Join(dir, "notes.md"), "# notes")
	shot := writeFile(t, filepath.Join(dir, "screen shot.png"), "\x89PNG")
	p := u.chats.pane("s1")
	p.editor.SetText("look at ")
	u.drop.add("s1", image.Rectangle{Max: r.size})
	u.onDrag(enter(300, 300, uriListMIME), r.invalidate)
	if !u.drop.hinting("s1") {
		t.Error("a file dragged over the chat is not followed")
	}
	u.onDrag(drop(300, 300, fileURI(file)+fileURI(shot)), r.invalidate)
	waitCards(t, r, p, file, shot)
	if p.editor.Text() != "look at " {
		t.Errorf("the composer reads %q, want the text it had", p.editor.Text())
	}
	if u.drop.hinting("s1") {
		t.Error("the drag is still followed after the drop")
	}
}
