package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
)

// queueFrames is session s1 drawn as a chat whose turn runs.
func queueFrames(t *testing.T) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.client = u.client
	u.model.chat.history("s1", nil)
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frame()
	return u, r, f, u.chats.pane("s1")
}

func (r *frameRig) pressWith(name key.Name, mods key.Modifiers) {
	r.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press}, key.Event{Name: name, Modifiers: mods, State: key.Release})
	r.frame()
}

// Enter while the agent works puts the message in the queue over the box,
// sending nothing.
func TestEnterDuringATurnQueuesTheMessage(t *testing.T) {
	u, r, f, p := queueFrames(t)
	typeIn(r, p, "also the offsets")
	r.press(key.NameReturn)
	r.frame()
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatPrompt.Name); n != 0 {
		t.Errorf("Enter during a turn sent %d prompts", n)
	}
	if q := u.model.chat.queued("s1"); !slices.Equal(queuedTexts(q), []string{"also the offsets"}) {
		t.Fatalf("queue %q, want the message", q)
	}
	if p.editor.Len() != 0 {
		t.Error("the box kept the queued message")
	}
	if !r.shows("also the offsets") {
		t.Errorf("the queued message is not drawn over the box:\n%s", r.drawn())
	}
}

// The queue goes in one message once the turn ends.
func TestTheQueueGoesInOneMessageWhenTheTurnEnds(t *testing.T) {
	u, _, f, _ := queueFrames(t)
	u.model.chat.enqueue("s1", chatMessage{text: "first"})
	u.model.chat.enqueue("s1", chatMessage{text: "second"})
	u.model.flushQueue(context.Background(), api.ChatTurn{SessionID: "s1", StopReason: "end_turn"})
	if body := f.waitCall(t, api.ChatPrompt.Name); !strings.Contains(body, `first\n\nsecond`) {
		t.Errorf("the turn's end sent %s, want both messages in one", body)
	}
	if q := u.model.chat.queued("s1"); len(q) != 0 {
		t.Errorf("queue %q after it was sent", q)
	}
}

// A turn the person stopped keeps its queue for them.
func TestAStoppedTurnKeepsItsQueue(t *testing.T) {
	u, _, f, _ := queueFrames(t)
	u.model.chat.enqueue("s1", chatMessage{text: "first"})
	u.model.flushQueue(context.Background(), api.ChatTurn{SessionID: "s1", StopReason: stopCancelled})
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatPrompt.Name); n != 0 {
		t.Errorf("a stopped turn sent its queue (%d prompts)", n)
	}
	if q := u.model.chat.queued("s1"); len(q) != 1 {
		t.Errorf("queue %q, want it kept", q)
	}
}

// Ctrl+Enter sends the box's text into the turn running.
func TestShortcutEnterSteersIntoTheTurn(t *testing.T) {
	u, r, f, p := queueFrames(t)
	typeIn(r, p, "use cents")
	r.pressWith(key.NameReturn, key.ModShortcut)
	if body := f.waitCall(t, api.ChatSteer.Name); !strings.Contains(body, "use cents") {
		t.Errorf("Ctrl+Enter steered %s", body)
	}
	if q := u.model.chat.queued("s1"); len(q) != 0 {
		t.Errorf("Ctrl+Enter queued %q", q)
	}
}

// Force sends one queued message into the turn running and takes it out of
// the queue.
func TestForceSendsAQueuedMessageIntoTheTurn(t *testing.T) {
	u, r, f, _ := queueFrames(t)
	u.model.chat.enqueue("s1", chatMessage{text: "first"})
	u.model.chat.enqueue("s1", chatMessage{text: "second"})
	r.frame()
	r.click(t, r.at(t, "Force"))
	if body := f.waitCall(t, api.ChatSteer.Name); !strings.Contains(body, "first") {
		t.Errorf("Force steered %s, want the first queued message", body)
	}
	if q := u.model.chat.queued("s1"); !slices.Equal(queuedTexts(q), []string{"second"}) {
		t.Errorf("queue %q, want the other one left", q)
	}
}

// ↑ on an empty box takes the whole queue back into it.
func TestUpTakesTheQueueBackToEdit(t *testing.T) {
	u, r, _, p := queueFrames(t)
	u.model.chat.enqueue("s1", chatMessage{text: "first"})
	u.model.chat.enqueue("s1", chatMessage{text: "second"})
	r.frame()
	if !r.shows("↑ edits the queue") {
		t.Errorf("no hint for the queue:\n%s", r.drawn())
	}
	r.press(key.NameUpArrow)
	if got := p.editor.Text(); got != "first\n\nsecond" {
		t.Errorf("box %q, want the queue", got)
	}
	if q := u.model.chat.queued("s1"); len(q) != 0 {
		t.Errorf("queue %q after ↑", q)
	}
}

// Clicking a queued message takes it out of the queue and back into the box.
func TestClickingAQueuedMessageTakesItBack(t *testing.T) {
	u, r, _, p := queueFrames(t)
	u.model.chat.enqueue("s1", chatMessage{text: "first"})
	r.frame()
	r.click(t, r.at(t, "first"))
	if got := p.editor.Text(); got != "first" {
		t.Errorf("box %q, want the message", got)
	}
	if q := u.model.chat.queued("s1"); len(q) != 0 {
		t.Errorf("queue %q after the click", q)
	}
}

// queuedTexts are the texts of a queue's messages.
func queuedTexts(q []chatMessage) []string {
	texts := make([]string, len(q))
	for i, m := range q {
		texts[i] = m.text
	}
	return texts
}
