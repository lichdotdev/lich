package main

import (
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
	// the keys are the box's until a click hands them back to the composer
	r.router.Source().Execute(key.FocusCmd{Tag: &p.editor})
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

// Esc while searching keeps the match in the box and leaves the turn running.
func TestEscKeepsTheSearchedMessageRatherThanStopping(t *testing.T) {
	_, r, f, p := sentFrames(t, true)
	typeIn(r, p, "half a thought")
	r.pressWith("R", key.ModCtrl)
	// typed as the window system does, to whatever has the focus
	r.router.Queue(key.EditEvent{Text: "docs"})
	r.frame()
	r.press(key.NameEscape)
	if got := p.editor.Text(); got != "ship the docs" || p.search.open {
		t.Errorf("box %q, search open %v: want the match kept and the search shut", got, p.search.open)
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
	u.model.chat.enqueue("s1", chatMessage{text: "queued one"})
	r.frame()
	r.press(key.NameUpArrow)
	if got := p.editor.Text(); got != "queued one" {
		t.Errorf("box %q, want the queue", got)
	}
}

// searchFor types query in the search's field and draws the frame that reads it.
func searchFor(r *frameRig, p *chatPane, query string) {
	p.search.query.SetText(query)
	r.frame()
}

// Ctrl+R searches the messages sent in the box itself: the newest that holds
// the query is shown with the match selected, Ctrl+R goes on to older ones,
// and past the oldest the field says nothing more matches.
func TestCtrlRShowsTheNewestMatchAndGoesBack(t *testing.T) {
	_, r, f, p := sentFrames(t, false)
	typeIn(r, p, "half a thought")
	r.pressWith("R", key.ModCtrl)
	if !r.shows(i18n.T("chat.search.label")) || p.editor.Text() != "half a thought" {
		t.Fatalf("Ctrl+R: box %q, want the draft under the search's label:\n%s", p.editor.Text(), r.drawn())
	}
	searchFor(r, p, "THE")
	if got := p.editor.Text(); got != "ship the docs" {
		t.Fatalf("box %q, want the newest match", got)
	}
	if a, b := p.editor.Selection(); min(a, b) != 5 || max(a, b) != 8 {
		t.Errorf("selection %d..%d, want the match, 5..8", a, b)
	}
	for _, want := range []string{"fix the totals\nin cents", "rename the flag", "rename the flag"} {
		r.pressWith("R", key.ModCtrl)
		if got := p.editor.Text(); got != want {
			t.Fatalf("Ctrl+R again: box %q, want %q", got, want)
		}
	}
	if !r.shows(i18n.T("chat.search.failed")) {
		t.Errorf("past the oldest match the field does not say so:\n%s", r.drawn())
	}
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatPrompt.Name); n != 0 {
		t.Errorf("searching sent %d prompts", n)
	}
}

// Esc or Tab keep the match in the box to edit, the caret where it matched.
func TestEscOrTabKeepsTheMatchToEdit(t *testing.T) {
	for _, k := range []key.Name{key.NameEscape, key.NameTab} {
		_, r, _, p := sentFrames(t, false)
		r.pressWith("R", key.ModCtrl)
		searchFor(r, p, "totals")
		r.press(k)
		if got := p.editor.Text(); got != "fix the totals\nin cents" || p.search.open {
			t.Fatalf("%s: box %q, search open %v", k, got, p.search.open)
		}
		if a, b := p.editor.Selection(); a != 8 || b != 8 {
			t.Errorf("%s: caret %d..%d, want 8 where it matched", k, a, b)
		}
		if u := p.editor.PaintSelectionWhenUnfocused; u {
			t.Errorf("%s: the box still paints its selection unfocused", k)
		}
	}
}

// Enter sends the match.
func TestEnterSendsTheMatch(t *testing.T) {
	u, r, _, p := sentFrames(t, false)
	r.pressWith("R", key.ModCtrl)
	searchFor(r, p, "flag")
	r.press(key.NameReturn)
	if p.editor.Len() != 0 || !u.model.chat.holding("s1") {
		t.Errorf("box %q, holding %v: want the match sent", p.editor.Text(), u.model.chat.holding("s1"))
	}
}

// Enter on a query nothing holds sends nothing and ends the search.
func TestEnterWithNoMatchSendsNothing(t *testing.T) {
	u, r, _, p := sentFrames(t, false)
	typeIn(r, p, "half a thought")
	r.pressWith("R", key.ModCtrl)
	searchFor(r, p, "nothing like it")
	if !r.shows(i18n.T("chat.search.failed")) {
		t.Errorf("no word for a query nothing holds:\n%s", r.drawn())
	}
	r.press(key.NameReturn)
	if p.search.open || u.model.chat.holding("s1") || p.editor.Text() != "half a thought" {
		t.Errorf("box %q, open %v, holding %v: want the draft, shut, unsent", p.editor.Text(), p.search.open, u.model.chat.holding("s1"))
	}
}

// Ctrl+C, or Backspace on an empty query, puts the draft back.
func TestCtrlCOrBackspaceCancelsTheSearch(t *testing.T) {
	for name, cancel := range map[string]func(r *frameRig, p *chatPane){
		"Ctrl+C": func(r *frameRig, _ *chatPane) { r.pressWith("C", key.ModCtrl) },
		"Backspace": func(r *frameRig, p *chatPane) {
			searchFor(r, p, "")
			r.press(key.NameDeleteBackward)
		},
	} {
		_, r, _, p := sentFrames(t, false)
		typeIn(r, p, "half a thought")
		r.pressWith("R", key.ModCtrl)
		searchFor(r, p, "docs")
		cancel(r, p)
		if got := p.editor.Text(); got != "half a thought" || p.search.open {
			t.Errorf("%s: box %q, search open %v: want the draft back", name, got, p.search.open)
		}
	}
}
