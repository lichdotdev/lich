package main

import (
	"image"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// opsLen is how much the last frame drew: what a frame lays out off screen
// adds to it.
func opsLen(r *frameRig) int {
	return len(reflect.ValueOf(&r.ops.Internal).Elem().FieldByName("data").Bytes())
}

// linkedAnswer is an answer of n paragraphs, each with a link: a link is a
// gesture the frame's tree shows where its paragraph is drawn.
func linkedAnswer(n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString("A paragraph of the answer, long enough to wrap in the column, with [link " + strconv.Itoa(i) + "](https://example.com/" + strconv.Itoa(i) + ") in it.\n\n")
	}
	return b.String()
}

func tallAnswer(paragraphs int) []backend.ChatItem {
	return []backend.ChatItem{
		{ID: "u0", Kind: api.ItemUser, Turn: 1, At: novDay(18, 9, 0), Text: "question"},
		{ID: "a0", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 9, 0), Text: linkedAnswer(paragraphs)},
	}
}

// drawnFor settles a chat of items and reports how much its frame drew.
func drawnFor(t *testing.T, items []backend.ChatItem) int {
	u, r := stampFrames(t)
	u.model.chat.history("s1", items)
	r.settle(t)
	return opsLen(r)
}

// A frame lays out the rows in view and a margin around them, and an answer
// taller than the view only the blocks of it in that stretch: ten times the
// conversation, or ten times the answer, draws about the same frame.
func TestOnlyTheViewAndItsMarginAreLaidOut(t *testing.T) {
	for _, tc := range []struct {
		name        string
		short, long []backend.ChatItem
	}{
		{"rows", longChat(100), longChat(1000)},
		{"an answer's blocks", tallAnswer(200), tallAnswer(2000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			short, long := drawnFor(t, tc.short), drawnFor(t, tc.long)
			if long > short*3/2 {
				t.Errorf("ten times the content drew %d bytes, against %d: what is off screen is laid out", long, short)
			}
		})
	}
}

// listBox is the stretch of the window the conversation is drawn in, over the
// composer.
func listBox(t *testing.T, r *frameRig) image.Rectangle {
	return image.Rect(0, 0, r.size.X, composerBox(t, r).Min.Y)
}

// linksIn counts the answer's links drawn in each half of within.
func linksIn(r *frameRig, within image.Rectangle) (upper, lower int) {
	mid := (within.Min.Y + within.Max.Y) / 2
	upper = clickablesIn(r, image.Rect(within.Min.X, within.Min.Y, within.Max.X, mid))
	lower = clickablesIn(r, image.Rect(within.Min.X, mid, within.Max.X, within.Max.Y))
	return upper, lower
}

// idle frames the screen as the window does, until a frame asks for no
// other, and reports what that frame drew.
func idle(t *testing.T, r *frameRig) string {
	t.Helper()
	for range maxSettleFrames {
		r.now = r.now.Add(frameStep)
		if drawn, redraw := r.frame(); !redraw {
			return drawn
		}
	}
	t.Fatal("the screen kept asking for frames")
	return ""
}

// drawnWhole checks that the frame the window rests on draws the answer over
// the whole view, and is what one more frame would draw.
func drawnWhole(t *testing.T, r *frameRig, where string) {
	t.Helper()
	rest := idle(t, r)
	if upper, lower := linksIn(r, listBox(t, r)); upper < 4 || lower < 4 {
		t.Errorf("%s, the answer draws %d links in the view's upper half and %d in its lower, want a paragraph's worth in both", where, upper, lower)
	}
	if again, _ := r.frame(); again != rest {
		t.Errorf("%s, the frame the window rested on was not final: one more drew something else", where)
	}
}

// An answer taller than the view is drawn wherever the view stands on it, in
// the frame the window rests on: at its end when the conversation opens,
// before any of its blocks were measured, after a scroll up, which stays where
// it was scrolled, and after a jump to its middle.
func TestATallAnswerIsDrawnWhereverTheViewStands(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", tallAnswer(2000))
	r.frame()
	drawnWhole(t, r, "at its end")
	p := u.chats.pane("s1")
	for range 30 {
		r.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(300, 300), Scroll: f32.Pt(0, -600)})
		r.now = r.now.Add(frameStep)
		r.frame()
	}
	at := p.list.Position
	if !at.BeforeEnd {
		t.Fatalf("the list did not scroll up: %+v", at)
	}
	drawnWhole(t, r, "scrolled up")
	if p.list.Position.First != at.First || p.list.Position.Offset != at.Offset {
		t.Errorf("the list moved from %+v to %+v after the scroll stopped", at, p.list.Position)
	}
	p.list.Position.First, p.list.Position.Offset = 1, p.heights["a0"]/2
	r.frame()
	drawnWhole(t, r, "after a jump to its middle")
}
