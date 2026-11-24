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

// Send now sends the whole queue into the turn running in one message and
// empties it. (It replaced a Force per message: the contract changed.)
func TestSendNowSendsTheWholeQueueIntoTheTurn(t *testing.T) {
	u, r, f, _ := queueFrames(t)
	u.model.chat.enqueue("s1", chatMessage{text: "first"})
	u.model.chat.enqueue("s1", chatMessage{text: "second"})
	r.frame()
	if !r.showsAll("2 queued", "Send now") {
		t.Fatalf("no queue head:\n%s", r.drawn())
	}
	writeShot(t, r, "queue-send-now.png")
	r.click(t, r.at(t, "Send now"))
	if body := f.waitCall(t, api.ChatSteer.Name); !strings.Contains(body, `first\n\nsecond`) {
		t.Errorf("Send now steered %s, want the whole queue in one", body)
	}
	if q := u.model.chat.queued("s1"); len(q) != 0 {
		t.Errorf("queue %q after Send now", q)
	}
}

// Ctrl+Enter with an empty box sends the queue into the turn, and with text
// in the box sends the queue and the text after it.
func TestShortcutEnterSendsTheQueueAndTheBox(t *testing.T) {
	u, r, f, p := queueFrames(t)
	u.model.chat.enqueue("s1", chatMessage{text: "first"})
	r.frame()
	r.pressWith(key.NameReturn, key.ModShortcut)
	if body := f.waitCall(t, api.ChatSteer.Name); !strings.Contains(body, `"first"`) {
		t.Errorf("Ctrl+Enter on an empty box steered %s, want the queue", body)
	}
	u.model.chat.enqueue("s1", chatMessage{text: "second"})
	r.frame()
	typeIn(r, p, "use cents")
	r.pressWith(key.NameReturn, key.ModShortcut)
	deadline := time.Now().Add(2 * time.Second)
	for f.callsOf(api.ChatSteer.Name) < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	f.mu.Lock()
	last := f.calls[len(f.calls)-1]
	f.mu.Unlock()
	if !strings.Contains(last, `second\n\nuse cents`) {
		t.Errorf("Ctrl+Enter with text steered %s, want the queue then the text", last)
	}
	if q := u.model.chat.queued("s1"); len(q) != 0 || p.editor.Len() != 0 {
		t.Errorf("queue %q and box %q left after Ctrl+Enter", q, p.editor.Text())
	}
}

// After a stopped turn the kept queue goes as the next message on Ctrl+Enter.
func TestShortcutEnterSendsAKeptQueueAsTheNextMessage(t *testing.T) {
	u, r, f, _ := queueFrames(t)
	u.model.chat.enqueue("s1", chatMessage{text: "first"})
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", StopReason: stopCancelled})
	r.frame()
	r.pressWith(key.NameReturn, key.ModShortcut)
	r.now = r.now.Add(chatSendHold + time.Second)
	r.frame()
	if body := f.waitCall(t, api.ChatPrompt.Name); !strings.Contains(body, "first") {
		t.Errorf("Ctrl+Enter after a stop sent %s, want the kept queue", body)
	}
	if n := f.callsOf(api.ChatSteer.Name); n != 0 {
		t.Errorf("a turn not running was steered %d times", n)
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

// A tool starting is the turn's next chance to read a message: the queue goes
// into the turn then on a provider that takes one mid-turn, and waits for the
// turn's end on one that would stop for it (ACP).
func TestAToolStartingSteersTheQueueIntoTheTurn(t *testing.T) {
	tool := func(state string) api.ChatItemUpdate {
		return api.ChatItemUpdate{SessionID: "s1", Item: api.ChatItem{ID: "t1", Kind: api.ItemTool, Tool: &api.ChatToolCall{Name: "Bash", State: state}}}
	}
	for _, c := range []struct {
		kind  string
		steer bool
	}{{"claude", true}, {"codex", true}, {"opencode", false}} {
		t.Run(c.kind, func(t *testing.T) {
			u, _, f, _ := queueFrames(t)
			u.model.projects[0].Sessions[0].Kind = c.kind
			u.model.chat.enqueue("s1", chatMessage{text: "first"})
			u.model.chat.enqueue("s1", chatMessage{text: "second"})
			u.model.steerQueue(context.Background(), tool(api.ToolDone))
			if q := u.model.chat.queued("s1"); len(q) != 2 {
				t.Fatalf("a tool that ended took the queue: %q left", q)
			}
			u.model.steerQueue(context.Background(), tool(api.ToolRunning))
			if !c.steer {
				time.Sleep(20 * time.Millisecond)
				if n := f.callsOf(api.ChatSteer.Name); n != 0 || len(u.model.chat.queued("s1")) != 2 {
					t.Errorf("%s steered %d times, want the queue kept for the turn's end", c.kind, n)
				}
				return
			}
			if body := f.waitCall(t, api.ChatSteer.Name); !strings.Contains(body, `first\n\nsecond`) {
				t.Errorf("the tool's start steered %s, want both messages in one", body)
			}
			if q := u.model.chat.queued("s1"); len(q) != 0 {
				t.Errorf("queue %q after it was steered", q)
			}
		})
	}
}
