package main

import (
	"image"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"github.com/lichdotdev/godemirror"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

const (
	selectAnswer = "Run **the bold part** and `go test ./...` before you push, so the long tail of this answer wraps onto a second line of the column."
	selectAsk    = "Can you check `app.go` before **lunch**?"
)

// selectFrames is s1 with one turn: the person's message and an answer.
func selectFrames(t *testing.T) (*window, *frameRig) {
	t.Helper()
	u, r, _ := chatFramesIn(t, api.ViewChat, image.Pt(720, 560), busySession())
	r.theme.Code = godemirror.DarkTheme
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: selectAsk},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: selectAnswer},
	})
	r.settle(t)
	return u, r
}

// textStart is the first point under the who line, left to right, where the
// pointer takes the text cursor: the answer's first glyph.
func textStart(t *testing.T, r *frameRig, from image.Point) f32.Point {
	t.Helper()
	for y := from.Y; y < from.Y+60; y += 4 {
		for x := from.X - 48; x < from.X+240; x += 2 {
			at := f32.Pt(float32(x), float32(y))
			r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at})
			r.frame()
			if r.router.Cursor() == pointer.CursorText {
				return at.Add(f32.Pt(1, 4))
			}
		}
	}
	t.Fatalf("no text cursor under %v:\n%s", from, r.drawn())
	return f32.Point{}
}

func (r *frameRig) dragAcross(from, to f32.Point) {
	r.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: from})
	r.frame()
	for k := 1; k <= 6; k++ {
		at := from.Add(to.Sub(from).Mul(float32(k) / 6))
		r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at})
		r.frame()
	}
	r.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: to})
	r.frame()
}

func (r *frameRig) copiedText() string {
	r.router.Queue(key.Event{Name: "C", Modifiers: key.ModShortcut, State: key.Press})
	r.frame()
	_, data, ok := r.router.WriteClipboard()
	if !ok {
		return ""
	}
	return string(data)
}

// A drag over an answer selects its text, and Ctrl+C copies what was read
// while the composer holds the keyboard; Escape lets the selection go first.
func TestADragOverAnAnswerSelectsTextCtrlCCopies(t *testing.T) {
	u, r := selectFrames(t)
	start := textStart(t, r, r.glyph(t, "claude").Add(image.Pt(0, 16)))
	// from the first word, across "the bold part" and the inline code, to
	// the second line
	r.dragAcross(start, start.Add(f32.Pt(120, 36)))
	writeShot(t, r, "shot-chat-answer-selection.png")
	if !r.router.Source().Focused(&u.chats.pane("s1").editor) {
		t.Fatal("the composer lost the keyboard to the selection")
	}
	got := r.copiedText()
	read := "Run the bold part and go test ./... before you push, so the long tail of this answer wraps onto a second line of the column."
	if len(got) < 20 || !strings.HasPrefix(read, got) {
		t.Fatalf("Ctrl+C copied %q, want the answer's opening as read, from %q", got, read)
	}
	r.router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	r.frame()
	if again := r.copiedText(); again != "" {
		t.Errorf("after Escape, Ctrl+C copied %q", again)
	}
}

func TestTheMessageBoxSelectsToo(t *testing.T) {
	_, r := selectFrames(t)
	box := labelBox(t, r, selectAsk)
	start := textStart(t, r, image.Pt(box.Min.X+48, box.Min.Y))
	r.dragAcross(start, f32.Pt(float32(box.Max.X-2), start.Y))
	if got := r.copiedText(); len(got) < 10 || !strings.HasPrefix("Can you check app.go before lunch?", got) {
		t.Errorf("Ctrl+C copied %q from the person's message", got)
	}
}
