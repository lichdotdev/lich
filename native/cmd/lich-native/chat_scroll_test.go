package main

import (
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// barShows is whether the list's scrollbar is drawn: the kit draws it when the
// rows drawn do not span the view, which for a list whose rows all stand in it
// is any offset at either end, or a first row that is not the first.
func barShows(p *chatPane) bool {
	pos := p.list.Position
	return pos.First != 0 || pos.Offset != 0 || pos.OffsetLast != 0
}

// A message sent into a conversation that fits never draws the scrollbar, in
// any frame of the row growing in; one that outgrows the view does.
func TestSendingIntoAConversationThatFitsDrawsNoScrollbar(t *testing.T) {
	u, r := stampFrames(t)
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 14, 32), Text: "hi"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 14, 35), Text: "hello"},
	}
	u.model.chat.history("s1", items)
	r.settle(t)
	p := u.chats.pane("s1")
	if barShows(p) {
		t.Fatal("the scrollbar shows before anything is sent")
	}
	u.model.chat.history("s1", append(items, backend.ChatItem{ID: "u2", Kind: api.ItemUser, Turn: 2, At: novDay(18, 14, 40), Text: "and now?"}))
	for i := range 60 {
		r.now = r.now.Add(frameStep)
		r.frame()
		if barShows(p) {
			t.Errorf("the scrollbar shows %d frames after a message was sent into a conversation that fits: %+v", i, p.list.Position)
		}
	}
	r.settle(t)
	if barShows(p) {
		t.Errorf("the scrollbar shows once the message has come in: %+v", p.list.Position)
	}
}

func TestAConversationTallerThanTheViewDrawsTheScrollbar(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", userTurns(5))
	r.settle(t)
	p := u.chats.pane("s1")
	u.model.chat.history("s1", userTurns(60))
	r.settle(t)
	if !barShows(p) {
		t.Errorf("no scrollbar over a conversation taller than the view: %+v", p.list.Position)
	}
}

// When a person scrolls up while a new row is entering or moving, the scroll
// stays where the person scrolled and does not force the list back to its end,
// whether the wheel moved it past a row's edge or by a few pixels inside one.
func TestScrollingUpWhileRowsAreMovingStaysScrolled(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scroll float32
	}{
		{"a notch", -40},
		{"a page", -300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, r := stampFrames(t)
			u.model.chat.history("s1", userTurns(60))
			r.settle(t)
			p := u.chats.pane("s1")
			if p.list.Position.BeforeEnd {
				t.Fatalf("conversation should start at end: %+v", p.list.Position)
			}

			// An item arrives at the end with motion.
			u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "new1", Kind: api.ItemUser, Turn: 61, Text: "incoming", At: novDay(18, 18, 0)}})
			r.frame()
			if !p.rows.Moving() {
				t.Fatal("expected row to be moving")
			}

			// While the row is moving, the person scrolls up.
			r.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(300, 300), Scroll: f32.Pt(0, tc.scroll)})
			r.frame()
			scrolledFirst := p.list.Position.First
			if !p.list.Position.BeforeEnd {
				t.Errorf("list did not scroll up: %+v", p.list.Position)
			}

			// In the next frame, the list must NOT be snapped back to the bottom.
			r.frame()
			if !p.list.Position.BeforeEnd || p.list.Position.First > scrolledFirst {
				t.Errorf("list snapped back to end on subsequent frame: %+v", p.list.Position)
			}
		})
	}
}
