package main

import (
	"testing"

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
