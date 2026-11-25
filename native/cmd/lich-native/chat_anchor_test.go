package main

import (
	"slices"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// sendFrames is s1's chat of n turns, idle at its end, once the composer sent
// text and the list played the 220ms run.
func sendFrames(t *testing.T, n int, text string) (*window, *frameRig, *chatPane) {
	t.Helper()
	u, r := stampFrames(t)
	u.model.chat.history("s1", userTurns(n))
	r.settle(t)
	if n == 60 {
		writeShot(t, r, "anchor-before.png")
	}
	p := u.chats.pane("s1")
	typeIn(r, p, text)
	r.press(key.NameReturn)
	r.play(30)
	return u, r, p
}

// answer is the agent's answer to the message sendFrames sent, heard after
// the backend's copy of that message, as a turn runs.
func answer(u *window, text string) {
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "user-61", Kind: api.ItemUser, Turn: 61, At: novDay(18, 18, 0), Text: "open the PR"}})
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "a61", Kind: api.ItemAgent, Turn: 61, At: novDay(18, 18, 1), Text: text}})
}

// anchoredTop is where the person's last message sits under the top of the
// list.
func anchoredTop(t *testing.T, p *chatPane) int {
	t.Helper()
	i := slices.Index(p.keys, lastUserRow(p.keys, p.seen))
	if i < 0 {
		t.Fatalf("no message of the person among %v", p.keys)
	}
	return p.rowTop(p.keys, i)
}

// A message that starts a turn rises to chatSendPeek under the top of the
// view, what came before showing over it and room under it for the answer,
// and the room draws no Latest pill.
func TestAMessageThatStartsATurnSitsNearTheTopOfTheView(t *testing.T) {
	_, r, p := sendFrames(t, 60, "open the PR")
	if got := anchoredTop(t, p); got != int(chatSendPeek) {
		t.Errorf("message %dpx under the top of the list, want %v: %+v", got, chatSendPeek, p.list.Position)
	}
	writeShot(t, r, "anchor-sent.png")
	if !r.shows("q59") {
		t.Errorf("nothing of what came before shows over the message:\n%s", r.drawn())
	}
	if p.list.Position.BeforeEnd || r.shows(i18n.T("chat.view.latest")) || r.shows(i18n.T("chat.view.newActivity")) {
		t.Errorf("the room counts as the list scrolled up: %+v", p.list.Position)
	}
}

// The answer takes the room as it grows, and once it fills it the list
// follows its end as before.
func TestAnAnswerFillsTheRoomThenTheListFollowsItsEnd(t *testing.T) {
	u, r, p := sendFrames(t, 60, "open the PR")
	answer(u, "Opened.")
	r.play(30)
	writeShot(t, r, "anchor-answered.png")
	if got := anchoredTop(t, p); got != int(chatSendPeek) {
		t.Errorf("a short answer moved the message to %dpx, want %v", got, chatSendPeek)
	}
	answer(u, strings.Repeat("A line of the answer.\n\n", 60))
	r.play(30)
	if !p.anchored.at.IsZero() || p.list.Position.BeforeEnd {
		t.Errorf("an answer taller than the room did not hand the list back to its end: anchor %+v, %+v", p.anchored, p.list.Position)
	}
	if got := anchoredTop(t, p); got >= 0 {
		t.Errorf("the message stays %dpx in view over an answer that outgrew the room", got)
	}
}

// Scrolled up while the room is there, the room is given up, and the answer
// coming in does not bring it back.
func TestScrollingUpGivesTheRoomUp(t *testing.T) {
	u, r, p := sendFrames(t, 60, "open the PR")
	r.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(300, 300), Scroll: f32.Pt(0, -40)})
	r.frame()
	if !p.anchored.at.IsZero() {
		t.Fatalf("the room stays once the list was scrolled up: %+v", p.list.Position)
	}
	answer(u, "Opened.")
	r.play(30)
	if !p.anchored.at.IsZero() {
		t.Errorf("the room came back with the answer: %+v", p.anchored)
	}
}

// A message taller than the view shows from its end, as any row does.
func TestAMessageTallerThanTheViewShowsFromItsEnd(t *testing.T) {
	_, _, p := sendFrames(t, 60, strings.Repeat("a long line\n", 120))
	if !p.anchored.at.IsZero() || p.list.Position.BeforeEnd {
		t.Errorf("a message taller than the view is held: anchor %+v, %+v", p.anchored, p.list.Position)
	}
}

// A conversation that fits the view rests on the composer: what is sent into
// it comes in at the foot, with no room under it.
func TestAMessageSentIntoAConversationThatFitsRestsOnTheComposer(t *testing.T) {
	_, _, p := sendFrames(t, 2, "open the PR")
	if !p.anchored.at.IsZero() || p.list.ScrollToEnd {
		t.Errorf("a conversation that fits took room: anchor %+v, to end %v", p.anchored, p.list.ScrollToEnd)
	}
}

// A message queued while a turn runs leaves the view where it is.
func TestAQueuedMessageDoesNotMoveTheView(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", userTurns(60))
	r.settle(t)
	p := u.chats.pane("s1")
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.play(30)
	before := anchoredTop(t, p)
	typeIn(r, p, "and then?")
	r.press(key.NameReturn)
	r.play(30)
	if len(u.model.chat.queued("s1")) != 1 {
		t.Fatalf("queued %d, want the message queued", len(u.model.chat.queued("s1")))
	}
	if !p.anchored.at.IsZero() || p.list.Position.BeforeEnd {
		t.Errorf("a queued message took room: anchor %+v, %+v", p.anchored, p.list.Position)
	}
	if got := anchoredTop(t, p); got > before {
		t.Errorf("the last message went from %dpx to %dpx: the view moved down", before, got)
	}
}

// lastUserRow is the key of the person's last message among shown, "" for
// none.
func lastUserRow(shown []string, seen map[string]chatRow) string {
	for _, s := range slices.Backward(shown) {
		if r := seen[s]; r.kind == chatRowItem && r.item.Kind == api.ItemUser {
			return s
		}
	}
	return ""
}
