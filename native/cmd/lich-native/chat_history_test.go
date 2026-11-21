package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// sentFrames is session s1 drawn as a chat with three messages sent, the
// second two lines long, and running when its turn runs.
func sentFrames(t *testing.T, running bool) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.client = u.client
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "rename the flag"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "Renamed."},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, Text: "fix the totals\nin cents"},
		{ID: "u3", Kind: api.ItemUser, Turn: 3, Text: "ship the docs"},
	})
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: running})
	// a turn running never goes still
	r.frames(3)
	return u, r, f, u.chats.pane("s1")
}

// noCancel fails when anything cancelled the turn.
func noCancel(t *testing.T, f *chatBackend) {
	t.Helper()
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatCancel.Name); n != 0 {
		t.Errorf("Esc cancelled the turn (%d calls)", n)
	}
}

// Esc stops the turn running, as Stop does, and keeps the box's text.
func TestEscStopsTheTurnAndKeepsTheBox(t *testing.T) {
	_, r, f, p := sentFrames(t, true)
	typeIn(r, p, "and the tests")
	r.press(key.NameEscape)
	f.waitCall(t, api.ChatCancel.Name)
	if got := p.editor.Text(); got != "and the tests" {
		t.Errorf("box %q, want the text kept", got)
	}
}

// With no turn running Esc has nothing to stop.
func TestEscWithNoTurnCancelsNothing(t *testing.T) {
	_, r, f, _ := sentFrames(t, false)
	r.press(key.NameEscape)
	noCancel(t, f)
}

// Esc in the wait takes the message back; a second has no turn to stop.
func TestEscTakesAWaitingMessageBackBeforeStopping(t *testing.T) {
	_, r, f, p := holdFrames(t, "rename the flag")
	r.press(key.NameEscape)
	r.press(key.NameEscape)
	if got := p.editor.Text(); got != "rename the flag" {
		t.Errorf("box %q, want the message back", got)
	}
	noCancel(t, f)
}

// Esc on a waiting permission rejects it and leaves the turn running; with
// text in the box it does neither.
func TestEscRejectsAPermissionRatherThanStopping(t *testing.T) {
	u, r, f := permissionFrames(t)
	p := u.chats.pane("s1")
	typeIn(r, p, "wait")
	r.press(key.NameEscape)
	noCancel(t, f)
	if n := f.callsOf(api.ChatAnswer.Name); n != 0 {
		t.Errorf("Esc with text in the box answered the permission (%d calls)", n)
	}
	typeIn(r, p, "")
	r.press(key.NameEscape)
	if body := f.waitCall(t, api.ChatAnswer.Name); !strings.Contains(body, `"optionId":"no"`) {
		t.Errorf("Esc answered %s, want the reject", body)
	}
	noCancel(t, f)
}

// Esc on a waiting question declines it on its card and stops nothing.
func TestEscDeclinesAQuestionRatherThanStopping(t *testing.T) {
	_, r, f := questionFrames(t, nil, askFruit)
	r.press(key.NameEscape)
	wantReply(t, f, `"declined":true`)
	noCancel(t, f)
}

// Esc with the slash list open shuts it and leaves the turn running.
func TestEscShutsTheSlashListRatherThanStopping(t *testing.T) {
	u, r, f, p := slashFrames(t)
	u.model.client = u.client
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	typeIn(r, p, "/re")
	r.press(key.NameEscape)
	if r.shows("Review a pull request") {
		t.Errorf("Esc left the list open:\n%s", r.drawn())
	}
	noCancel(t, f)
}

// Esc in the /mcp panel shuts it and leaves the turn running.
func TestEscShutsTheToolsPanelRatherThanStopping(t *testing.T) {
	u, r, f, _ := toolsFrames(t, "/mcp")
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frame()
	r.press(key.NameEscape)
	if r.shows("MCP servers") {
		t.Errorf("Esc left the panel open:\n%s", r.drawn())
	}
	noCancel(t, f)
}

// Esc with the search open shuts it, the draft back, and leaves the turn
// running.
func TestEscShutsTheSearchRatherThanStopping(t *testing.T) {
	_, r, f, p := sentFrames(t, true)
	typeIn(r, p, "half a thought")
	r.pressWith("R", key.ModCtrl)
	typeIn(r, p, "docs")
	r.press(key.NameEscape)
	if got := p.editor.Text(); got != "half a thought" {
		t.Errorf("box %q, want the draft back", got)
	}
	if r.shows("ship the docs") && p.search.open {
		t.Errorf("Esc left the search open:\n%s", r.drawn())
	}
	noCancel(t, f)
}

// ↑ walks back through the messages sent, newest first, ↓ walks forward, and
// past the newest the draft comes back.
func TestUpAndDownWalkTheMessagesSent(t *testing.T) {
	_, r, _, p := sentFrames(t, false)
	typeIn(r, p, "half a thought")
	steps := []struct {
		key  key.Name
		want string
	}{
		{key.NameUpArrow, "ship the docs"},
		{key.NameUpArrow, "fix the totals\nin cents"},
		{key.NameUpArrow, "rename the flag"},
		{key.NameUpArrow, "rename the flag"},
		{key.NameDownArrow, "fix the totals\nin cents"},
		{key.NameDownArrow, "ship the docs"},
		{key.NameDownArrow, "half a thought"},
	}
	for i, s := range steps {
		r.press(s.key)
		if got := p.editor.Text(); got != s.want {
			t.Fatalf("step %d (%s): box %q, want %q", i, s.key, got, s.want)
		}
	}
}

// ↑ off the box's first line moves the caret, not the walk; on it, it walks.
func TestUpWalksOnlyFromTheFirstLine(t *testing.T) {
	_, r, _, p := sentFrames(t, false)
	typeIn(r, p, "one\ntwo")
	r.press(key.NameUpArrow)
	if got := p.editor.Text(); got != "one\ntwo" {
		t.Fatalf("↑ on the second line walked: box %q", got)
	}
	p.editor.SetCaret(0, 0)
	r.frame()
	r.press(key.NameUpArrow)
	if got := p.editor.Text(); got != "ship the docs" {
		t.Errorf("↑ on the first line: box %q, want the newest message", got)
	}
}

// A message recalled and then edited is a draft of its own: ↓ no longer walks.
func TestEditingARecalledMessageEndsTheWalk(t *testing.T) {
	_, r, _, p := sentFrames(t, false)
	r.press(key.NameUpArrow)
	typeIn(r, p, "ship the docs today")
	r.press(key.NameDownArrow)
	if got := p.editor.Text(); got != "ship the docs today" {
		t.Errorf("↓ after an edit: box %q, want the edit kept", got)
	}
}

// ↑ on an empty box with a queue edits the queue, not the messages sent.
func TestTheQueueComesBeforeTheMessagesSent(t *testing.T) {
	u, r, _, p := sentFrames(t, true)
	u.model.chat.enqueue("s1", "queued one")
	r.frame()
	r.press(key.NameUpArrow)
	if got := p.editor.Text(); got != "queued one" {
		t.Errorf("box %q, want the queue", got)
	}
}

// Ctrl+R lists the messages sent over the box, typing filters them, and Enter
// puts the one picked in the box without sending it.
func TestCtrlRSearchesTheMessagesSent(t *testing.T) {
	_, r, f, p := sentFrames(t, false)
	r.pressWith("R", key.ModCtrl)
	if !r.showsAll(i18n.T("chat.search.placeholder"), "ship the docs", "rename the flag", "fix the totals in cents") {
		t.Fatalf("Ctrl+R lists no messages:\n%s", r.drawn())
	}
	typeIn(r, p, "THE")
	r.press(key.NameDownArrow)
	r.press(key.NameReturn)
	if got := p.editor.Text(); got != "fix the totals\nin cents" {
		t.Errorf("box %q, want the second match", got)
	}
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatPrompt.Name); n != 0 || p.search.open {
		t.Errorf("the pick was sent (%d prompts) or the list stayed open", n)
	}
	r.pressWith("R", key.ModCtrl)
	typeIn(r, p, "nothing like it")
	if !r.shows(i18n.T("chat.search.empty")) {
		t.Errorf("no word for a query nothing holds:\n%s", r.drawn())
	}
	r.press(key.NameReturn)
	if !p.search.open {
		t.Error("Enter with no match shut the search")
	}
}

// A message clicked in the list goes in the box.
func TestClickingASearchedMessagePicksIt(t *testing.T) {
	_, r, _, p := sentFrames(t, false)
	r.pressWith("R", key.ModCtrl)
	var row image.Rectangle
	for _, n := range r.nodes() {
		// the conversation draws the message too; the list is drawn over it
		if n.Desc.Label == "rename the flag" {
			row, _ = r.smallestClickAt(n.Desc.Bounds.Min.Add(image.Pt(2, 2)))
		}
	}
	if row.Empty() {
		t.Fatalf("no row to click:\n%s", r.drawn())
	}
	r.click(t, centre(row))
	if got := p.editor.Text(); got != "rename the flag" {
		t.Errorf("box %q, want the message clicked", got)
	}
}

// ↓ straight after ↑ brings the draft back: the walk asks for ↓ in the frame
// that began it.
func TestDownRightAfterUpBringsTheDraftBack(t *testing.T) {
	_, r, _, p := sentFrames(t, false)
	typeIn(r, p, "half a thought")
	r.press(key.NameUpArrow)
	r.press(key.NameDownArrow)
	if got := p.editor.Text(); got != "half a thought" {
		t.Errorf("box %q, want the draft back", got)
	}
}

// The ↑ that takes the caret to the first line is the editor's, the next one
// walks.
func TestUpFromTheSecondLineMovesThenWalks(t *testing.T) {
	_, r, _, p := sentFrames(t, false)
	typeIn(r, p, "one\ntwo")
	r.press(key.NameUpArrow)
	if got := p.editor.Text(); got != "one\ntwo" {
		t.Fatalf("the first ↑ walked: box %q", got)
	}
	r.press(key.NameUpArrow)
	if got := p.editor.Text(); got != "ship the docs" {
		t.Errorf("the second ↑: box %q, want the newest message", got)
	}
}
