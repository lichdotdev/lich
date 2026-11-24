package main

import (
	"testing"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
)

// scrolledUp is s1's chat of sixty turns, scrolled up from its end.
func scrolledUp(t *testing.T) (*window, *frameRig, *chatPane) {
	t.Helper()
	u, r := stampFrames(t)
	u.model.chat.history("s1", userTurns(60))
	r.settle(t)
	p := u.chats.pane("s1")
	p.list.Position.First, p.list.Position.Offset, p.list.Position.BeforeEnd = 10, 0, true
	r.settle(t)
	return u, r, p
}

// Scrolled up, the list shows a pill that runs it back to its end, gone once
// there; at the end there is none.
func TestAPillRunsAListScrolledUpBackToItsEnd(t *testing.T) {
	_, r, p := scrolledUp(t)
	latest := i18n.T("chat.view.latest")
	writeShot(t, r, "jump-latest.png")
	if !r.shows(latest) {
		t.Fatalf("no pill over a list scrolled up:\n%s", r.drawn())
	}
	r.click(t, r.at(t, latest))
	r.play(20)
	if p.list.Position.BeforeEnd || !r.shows("q59") {
		t.Errorf("the pill did not run the list to its end: %+v", p.list.Position)
	}
	if r.shows(latest) {
		t.Errorf("the pill stays at the end:\n%s", r.drawn())
	}
}

// While the agent works below a list scrolled up, the pill says so.
func TestThePillSaysWhenTheAgentWorksBelow(t *testing.T) {
	u, r, _ := scrolledUp(t)
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frame()
	writeShot(t, r, "jump-activity.png")
	if !r.shows(i18n.T("chat.view.newActivity")) {
		t.Errorf("no word of the work below:\n%s", r.drawn())
	}
}

// A message typed while a turn runs goes to the queue and still takes the
// list back to its end: the person who sends is following the conversation.
func TestAMessageQueuedFromAboveRunsTheListToItsEnd(t *testing.T) {
	u, r, p := scrolledUp(t)
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frame()
	p.editor.SetText("and then?")
	r.press(key.NameReturn)
	r.play(20)
	if len(u.model.chat.queued("s1")) != 1 || p.list.Position.BeforeEnd {
		t.Errorf("queued %d, list %+v: want the message queued and the list at its end", len(u.model.chat.queued("s1")), p.list.Position)
	}
}
