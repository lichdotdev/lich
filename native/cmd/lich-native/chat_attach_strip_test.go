package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"sync"
	"testing"

	"gioui.org/app"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// writePNG writes a small gradient PNG at path, for a card's thumbnail.
func writePNG(t *testing.T, path string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := range 48 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{R: uint8(30 + x), G: uint8(40 + y), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return writeFile(t, path, buf.String())
}

// addSquare is the bounds of the square that ends the composer's strip, and
// whether one is drawn.
func addSquare(r *frameRig) (image.Rectangle, bool) {
	for _, n := range r.nodes() {
		if n.Desc.Description == i18n.T("chat.attach.add") {
			return n.Desc.Bounds, true
		}
	}
	return image.Rectangle{}, false
}

// The strip, and the add square that ends it, stand only while the composer
// holds a file; an emptied composer is the box as it was.
func TestTheStripStandsOnlyWithFiles(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	if _, ok := addSquare(r); ok {
		t.Fatal("the add square is drawn with no file attached")
	}
	dir := t.TempDir()
	shot := writePNG(t, filepath.Join(dir, "screen.png"))
	spec := writeFile(t, filepath.Join(dir, "spec-v2.pdf"), "%PDF")
	u.chats.attachIn("s1", []backend.Attachment{{Path: shot}, {Path: spec}})
	r.settle(t)
	writeShot(t, r, "strip-rest.png")
	sq, ok := addSquare(r)
	if !ok {
		t.Fatalf("no add square after a file was attached:\n%s", r.drawn())
	}
	p := u.chats.pane("s1")
	p.attached = nil
	r.settle(t)
	if _, ok := addSquare(r); ok {
		t.Errorf("the add square at %v stays once the files are gone", sq)
	}
}

// dragFrames is cardFrames with the whole rig a drop target for s1, as the
// stage records a session's bounds.
func dragFrames(t *testing.T) (*window, *frameRig, *chatBackend, *chatPane, string) {
	t.Helper()
	u, r, f, p, notes := cardFrames(t)
	u.drop.add("s1", image.Rectangle{Max: r.size})
	return u, r, f, p, notes
}

// A file dragged over a chat with a file in it opens a ghost card after the
// card, before the add square, dashes the box and says the drop attaches,
// with no terminal hint; dropped, the file's card takes the ghost's place
// and nothing in the strip moves.
func TestADragShowsAGhostAndTheDropKeepsItsPlace(t *testing.T) {
	u, r, _, p, notes := dragFrames(t)
	u.chats.attachIn("s1", []backend.Attachment{{Path: notes}})
	r.settle(t)
	cardX := r.at(t, "notes").X
	rest, ok := addSquare(r)
	if !ok {
		t.Fatalf("no add square with a file attached:\n%s", r.drawn())
	}
	step := centre(rest).X - cardX
	u.onDrag(enter(300, 300, uriListMIME), r.invalidate)
	r.settle(t)
	drag, _ := addSquare(r)
	writeShot(t, r, "strip-drag-one.png")
	if got := centre(drag).X - cardX; got != 2*step {
		t.Errorf("the add square stands %v right of the card during the drag, want %v: one ghost between them", got, 2*step)
	}
	if !p.dragging || !r.shows(i18n.T("chat.attach.drop")) {
		t.Errorf("the drag does not say the drop attaches:\n%s", r.drawn())
	}
	if r.shows(i18n.T("terminal.dropHint.pasted")) {
		t.Error("a chat draws the terminal's drop hint")
	}
	sales := writeFile(t, filepath.Join(filepath.Dir(notes), "q4-sales.csv"), "q,total")
	u.onDrag(drop(300, 300, fileURI(sales)), r.invalidate)
	waitCards(t, r, p, notes, sales)
	r.settle(t)
	if got := r.at(t, "q4-sales").X - cardX; got != step {
		t.Errorf("the dropped card stands %v right of the first, want %v: the ghost's place", got, step)
	}
	if landed, _ := addSquare(r); landed != drag {
		t.Errorf("the add square moved from %v to %v on the drop", drag, landed)
	}
	if r.shows(i18n.T("chat.attach.drop")) {
		t.Error("the drop hint stays on the chips after the drop")
	}
}

// While a drop resolves its ghost holds its place, and the files take it in
// the frame that ends the landing.
func TestTheGhostHoldsWhileTheDropResolves(t *testing.T) {
	u, r, _, p, notes := dragFrames(t)
	u.chats.land("s1")
	r.settle(t)
	held, ok := addSquare(r)
	if !ok || !p.landing {
		t.Fatalf("no strip while a drop resolves:\n%s", r.drawn())
	}
	u.chats.landed("s1", []backend.Attachment{{Path: notes}})
	r.settle(t)
	if sq, _ := addSquare(r); sq != held || p.landing {
		t.Errorf("the add square moved from %v to %v when the ghost became the card", held, sq)
	}
	u.chats.land("s1")
	r.settle(t)
	u.chats.landed("s1", nil)
	r.settle(t)
	if sq, _ := addSquare(r); sq.Min.X >= held.Min.X+held.Dx() {
		t.Errorf("a drop that resolved to nothing left its ghost: the add square at %v", sq)
	}
}

// A drag that leaves the session takes its ghost along, and the strip it
// opened on an empty composer closes.
func TestADragThatLeavesClearsTheStrip(t *testing.T) {
	u, r, _, p, _ := dragFrames(t)
	u.onDrag(enter(300, 300, uriListMIME), r.invalidate)
	r.settle(t)
	if _, ok := addSquare(r); !ok {
		t.Fatalf("a drag over an empty composer opens no strip:\n%s", r.drawn())
	}
	writeShot(t, r, "strip-drag-none.png")
	u.onDrag(app.DropEvent{Kind: app.DropLeave}, r.invalidate)
	r.settle(t)
	if _, ok := addSquare(r); ok || p.dragging || r.shows(i18n.T("chat.attach.drop")) {
		t.Errorf("the strip stays after the drag left:\n%s", r.drawn())
	}
}

// The chips' "+" stands whatever the footer's layout holds, and it and the
// add square each open the picker, whose file lands as a card; on every
// provider the rig fakes, whose chips follow the "+".
func TestThePlusAndTheAddSquareOpenThePicker(t *testing.T) {
	for _, kind := range []string{claudeKind, codexKind, opencodeKind} {
		t.Run(kind, func(t *testing.T) {
			u, r, _ := chatFrames(t, api.ViewChat)
			p := u.chats.pane("s1")
			notes := writeFile(t, filepath.Join(t.TempDir(), "notes.md"), "# notes")
			u.model.projects[0].Sessions[0].Kind = kind
			u.model.chat.kinds = []string{kind}
			var mu sync.Mutex
			var writes []string
			u.footer.client = pickBackend(t, backend.Attachment{Path: notes}, &mu, &writes)
			u.footer.attachAt = u.attachAtPrompt
			u.footer.layout = footerLayout{Left: []string{"changes"}}
			r.settle(t)
			r.click(t, r.smallestClickIn(t, boxPlus))
			waitCards(t, r, p, notes)
			r.settle(t)
			other := writeFile(t, filepath.Join(filepath.Dir(notes), "spec-v2.pdf"), "%PDF")
			u.footer.client = pickBackend(t, backend.Attachment{Path: other}, &mu, &writes)
			sq, ok := addSquare(r)
			if !ok {
				t.Fatalf("no add square with a file attached:\n%s", r.drawn())
			}
			r.click(t, centre(sq))
			waitCards(t, r, p, notes, other)
		})
	}
}
