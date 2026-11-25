package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// holdFrames is session s1 drawn as a chat with no turn running, a message
// sent from its composer.
func holdFrames(t *testing.T, text string) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", nil)
	r.settle(t)
	p := u.chats.pane("s1")
	typeIn(r, p, text)
	r.press(key.NameReturn)
	// The start screen gives way to the conversation.
	r.now = r.now.Add(time.Second)
	r.frames(3)
	return u, r, f, p
}

// A message sent is drawn sent at once, the turn running under it, while it
// waits in the window.
func TestASentMessageIsDrawnSentWhileItWaits(t *testing.T) {
	u, r, f, _ := holdFrames(t, "rename the flag")
	if n := f.callsOf(api.ChatPrompt.Name); n != 0 {
		t.Errorf("the message went at once (%d prompts)", n)
	}
	if !r.showsAll("rename the flag", chatVerb("", "s1", 1)) {
		t.Errorf("the waiting message and its turn are not drawn:\n%s", r.drawn())
	}
	if !u.model.chat.isRunning("s1") {
		t.Error("a waiting message does not run the turn")
	}
}

// Esc in the wait takes the message back to the box, and it never goes.
func TestEscTakesAWaitingMessageBack(t *testing.T) {
	u, r, f, p := holdFrames(t, "rename the flag")
	r.press(key.NameEscape)
	if got := p.editor.Text(); got != "rename the flag" {
		t.Errorf("box %q, want the message back", got)
	}
	r.now = r.now.Add(time.Second)
	r.frames(3)
	if r.shows(chatVerb("", "s1", 1)) || u.model.chat.isRunning("s1") {
		t.Errorf("the turn still runs once the message was taken back:\n%s", r.drawn())
	}
	time.Sleep(chatSendHold + 100*time.Millisecond)
	if n := f.callsOf(api.ChatPrompt.Name); n != 0 {
		t.Errorf("a message taken back went (%d prompts)", n)
	}
}

// Taken back, the message goes ahead of what was typed in the wait.
func TestATakenBackMessageKeepsWhatWasTyped(t *testing.T) {
	_, r, _, p := holdFrames(t, "rename the flag")
	typeIn(r, p, "and the docs")
	r.press(key.NameEscape)
	if got := p.editor.Text(); got != "rename the flag\n\nand the docs" {
		t.Errorf("box %q, want the message then what was typed", got)
	}
}

// Stop in the wait takes the message back, with no turn to cancel.
func TestStopTakesAWaitingMessageBack(t *testing.T) {
	_, r, f, p := holdFrames(t, "rename the flag")
	r.click(t, r.smallestClickIn(t, boxFoot))
	if got := p.editor.Text(); got != "rename the flag" {
		t.Errorf("box %q, want the message back", got)
	}
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatCancel.Name); n != 0 {
		t.Errorf("Stop cancelled a turn not begun (%d calls)", n)
	}
}

// The wait up, the message goes once, with no frame drawn, and Esc no longer
// reaches it.
func TestAWaitingMessageGoesOnceTheWaitIsUp(t *testing.T) {
	_, r, f, p := holdFrames(t, "rename the flag")
	if body := f.waitCall(t, api.ChatPrompt.Name); !strings.Contains(body, "rename the flag") {
		t.Errorf("sent %s", body)
	}
	r.press(key.NameEscape)
	if p.editor.Len() != 0 {
		t.Errorf("Esc took back a message gone: box %q", p.editor.Text())
	}
	if n := f.callsOf(api.ChatPrompt.Name); n != 1 {
		t.Errorf("%d prompts, want 1", n)
	}
}

// Enter or Ctrl+Enter in the wait queues: steered, the text would reach the
// agent before the message waiting.
func TestWritingInTheWaitQueues(t *testing.T) {
	u, r, f, p := holdFrames(t, "rename the flag")
	typeIn(r, p, "also the docs")
	r.pressWith(key.NameReturn, key.ModShortcut)
	typeIn(r, p, "and the tests")
	r.press(key.NameReturn)
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatSteer.Name); n != 0 {
		t.Errorf("Ctrl+Enter in the wait steered (%d calls)", n)
	}
	if q := u.model.chat.queued("s1"); len(q) != 2 {
		t.Errorf("queue %q, want both messages", q)
	}
}

// The message heard back replaces the one drawn, whatever id it carries.
func TestTheMessageHeardBackReplacesTheOneWaiting(t *testing.T) {
	for _, id := range []string{"user-1", "user-7"} {
		var s chatStore
		s.history("s1", nil)
		h := s.hold("s1", chatMessage{text: "rename the flag"}, time.Now())
		s.dispatch("s1", h)
		s.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: id, Kind: api.ItemUser, Text: "rename the flag", Turn: 1}})
		users := slices.DeleteFunc(s.of("s1"), func(it backend.ChatItem) bool { return it.Kind != api.ItemUser })
		if len(users) != 1 || users[0].ID != id {
			t.Errorf("heard back as %s: %v, want it alone", id, users)
		}
	}
}

// A message the backend refused goes back ahead of what was typed in the wait.
func TestARefusedMessageKeepsWhatWasTyped(t *testing.T) {
	_, r, _, p := holdFrames(t, "rename the flag")
	typeIn(r, p, "and the docs")
	p.mu.Lock()
	p.unsent = chatMessage{text: "rename the flag"}
	p.mu.Unlock()
	r.frame()
	if got := p.editor.Text(); got != "rename the flag\n\nand the docs" {
		t.Errorf("box %q, want the refused message then what was typed", got)
	}
}

// A message gone and never heard back stops running the turn once the turn
// ends or the history is read again.
func TestAMessageGoneEndsWithItsTurn(t *testing.T) {
	for name, end := range map[string]func(*chatStore){
		"turn":    func(s *chatStore) { s.turn(api.ChatTurn{SessionID: "s1", StopReason: "end_turn"}) },
		"history": func(s *chatStore) { s.history("s1", nil) },
	} {
		var s chatStore
		s.history("s1", nil)
		s.dispatch("s1", s.hold("s1", chatMessage{text: "rename the flag"}, time.Now()))
		end(&s)
		if s.isRunning("s1") || len(s.of("s1")) != 0 {
			t.Errorf("%s: the message gone still runs the turn", name)
		}
	}
}
