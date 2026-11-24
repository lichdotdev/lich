package main

import (
	"image"
	"strconv"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// unevenTurns is a conversation of one-line questions each answered at length:
// rows of very different heights, which layout.List's estimate swings over.
func unevenTurns(n int) []backend.ChatItem {
	var items []backend.ChatItem
	for i := range n {
		id := strconv.Itoa(i)
		items = append(items,
			backend.ChatItem{ID: "u" + id, Kind: api.ItemUser, Turn: i + 1, At: novDay(18, 9, i), Text: "question"},
			backend.ChatItem{ID: "a" + id, Kind: api.ItemAgent, Turn: i + 1, At: novDay(18, 9, i), Text: linkedAnswer(12)})
	}
	return items
}

// thumb is the scrollbar's indicator: the shortest gesture along the list's
// right edge, inside the track that spans it.
func thumb(t *testing.T, r *frameRig) image.Rectangle {
	t.Helper()
	box := listBox(t, r)
	var best image.Rectangle
	for _, n := range r.nodes() {
		b := n.Desc.Bounds
		if n.Desc.Gestures == 0 || b.Max.X != box.Max.X || b.Dx() > 20 || !b.In(box) {
			continue
		}
		if best.Empty() || b.Dy() < best.Dy() {
			best = b
		}
	}
	if best.Empty() {
		t.Fatalf("no scrollbar thumb:\n%s", r.drawn())
	}
	return best
}

// wheel scrolls the chat by dy pixels and lets it settle.
func wheel(t *testing.T, r *frameRig, dy float32) {
	r.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(300, 300), Scroll: f32.Pt(0, dy)})
	idle(t, r)
}

// Once every row was measured, the thumb keeps its length wherever the view
// stands, over a short question or a long answer.
func TestTheChatThumbHoldsItsLengthOverUnevenRows(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", unevenTurns(6))
	r.settle(t)
	p := u.chats.pane("s1")
	for p.list.Position.First > 0 || p.list.Position.Offset > 0 {
		wheel(t, r, -300)
	}
	lo, hi := thumb(t, r).Dy(), 0
	for p.list.Position.BeforeEnd {
		wheel(t, r, 150)
		h := thumb(t, r).Dy()
		lo, hi = min(lo, h), max(hi, h)
	}
	if hi-lo > 2 {
		t.Errorf("the thumb ran from %dpx to %dpx long over the same conversation", lo, hi)
	}
}
