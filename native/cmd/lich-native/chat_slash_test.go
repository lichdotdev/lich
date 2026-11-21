package main

import (
	"image"
	"strconv"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestASlashOpensOnlyAtTheStartOfALineAndNotOnAPath(t *testing.T) {
	for _, c := range []struct {
		text  string
		query string
		from  int
		ok    bool
	}{
		{"/", "", 0, true},
		{"/rev", "rev", 0, true},
		{"look at\n/rev", "rev", 8, true},
		{"/review now", "", 0, false},
		{"hello /rev", "", 0, false},
		{"/home/me/a.md", "", 0, false},
		{"", "", 0, false},
	} {
		query, from, ok := slashQuery(c.text, len([]rune(c.text)))
		if query != c.query || from != c.from || ok != c.ok {
			t.Errorf("%q: %q at %d, %v; want %q at %d, %v", c.text, query, from, ok, c.query, c.from, c.ok)
		}
	}
}

// slashFrames is a chat whose provider lists three commands, one of them a
// skill, the composer focused.
func slashFrames(t *testing.T) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.setCommands("s1", []backend.ChatCommand{
		{Text: "/review", Description: "Review a pull request", Hint: "[pr]"},
		{Text: "/deploy", Description: "Ship it", Skill: true},
		{Text: "/compact", Description: "Compact the conversation"},
	})
	r.settle(t)
	return u, r, f, u.chats.pane("s1")
}

// typeIn puts text in the composer with the caret at its end, as typing it
// would, and draws the frame that reads it.
func typeIn(r *frameRig, p *chatPane, text string) {
	p.editor.SetText(text)
	n := p.editor.Len()
	p.editor.SetCaret(n, n)
	r.frame()
}

func (r *frameRig) showsAll(labels ...string) bool {
	for _, l := range labels {
		if !r.shows(l) {
			return false
		}
	}
	return true
}

// "/" opens the list of the provider's commands, typing filters it, Tab
// completes the one under the cursor without sending it, and the arrows and
// Enter pick another.
func TestTheSlashListOpensFiltersAndCompletes(t *testing.T) {
	u, r, f, p := slashFrames(t)
	typeIn(r, p, "/")
	if !r.showsAll("/new", "/review", "[pr]", "/deploy", "/compact", "Ship it") {
		t.Fatalf("no list of commands on /:\n%s", r.drawn())
	}
	typeIn(r, p, "/co")
	if !r.shows("/compact") || r.shows("/review") || r.shows("/deploy") {
		t.Errorf("/co lists more than /compact:\n%s", r.drawn())
	}
	r.press(key.NameTab)
	if got := p.editor.Text(); got != "/compact " {
		t.Errorf("Tab left %q, want the command and a space", got)
	}
	r.frame()
	if r.shows("Compact the conversation") {
		t.Error("the list stays open once the command is in")
	}
	// lich's own /new, /mcp, /skills and /plugin lead the provider's commands.
	typeIn(r, p, "/")
	for range 5 {
		r.press(key.NameDownArrow)
	}
	r.press(key.NameReturn)
	if got := p.editor.Text(); got != "/deploy " {
		t.Errorf("Down and Enter left %q, want /deploy", got)
	}
	if u.model.chat.isRunning("s1") {
		t.Error("Enter on the list holds a message to send")
	}
	time.Sleep(50 * time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.HasPrefix(c, api.ChatPrompt.Name) {
			t.Errorf("Enter on the list sent %s", c)
		}
	}
}

// Escape shuts the list and keeps it shut while the same start stays; the
// next character opens it again.
func TestEscapeShutsTheSlashList(t *testing.T) {
	_, r, _, p := slashFrames(t)
	typeIn(r, p, "/re")
	r.press(key.NameEscape)
	r.frame()
	if r.shows("/review") {
		t.Fatalf("Escape left the list open:\n%s", r.drawn())
	}
	if got := p.editor.Text(); got != "/re" {
		t.Errorf("Escape changed the text to %q", got)
	}
	typeIn(r, p, "/rev")
	if !r.shows("/review") {
		t.Error("typing on after Escape does not open the list again")
	}
}

// A path at the start of the line, a slash mid-line or a start that matches
// nothing opens no list, and Enter sends the message as it is.
func TestNoSlashListLeavesEnterToSend(t *testing.T) {
	for _, text := range []string{"/home/me/a.md", "see /review", "/zzz"} {
		_, r, f, p := slashFrames(t)
		typeIn(r, p, text)
		if r.shows("Review a pull request") {
			t.Errorf("%q opened the list:\n%s", text, r.drawn())
		}
		r.press(key.NameReturn)
		if body := f.waitCall(t, api.ChatPrompt.Name); !strings.Contains(body, text) {
			t.Errorf("%q sent %s", text, body)
		}
	}
}

// Typed through the keyboard, the frame that took a keystroke draws the list
// it leaves, not the one before it.
func TestTheFrameThatTookTheKeyDrawsTheList(t *testing.T) {
	_, r, _, p := slashFrames(t)
	// A platform follows each edit with where the caret went.
	r.router.Queue(key.EditEvent{Text: "/"}, key.SelectionEvent{Start: 1, End: 1})
	r.frame()
	if p.editor.Text() != "/" || !r.shows("/review") {
		t.Fatalf("the frame that took / draws no list (text %q):\n%s", p.editor.Text(), r.drawn())
	}
	r.router.Queue(key.EditEvent{Range: key.Range{Start: 1, End: 1}, Text: "c"}, key.SelectionEvent{Start: 2, End: 2})
	r.frame()
	if !r.shows("/compact") || r.shows("/review") {
		t.Errorf("the frame that took c draws the list before it:\n%s", r.drawn())
	}
}

// A low pane holds the list under its own top: a long list scrolls in the
// room over the composer instead of running past the pane.
func TestTheSlashListStaysInALowPane(t *testing.T) {
	u, r, _ := chatFramesIn(t, api.ViewChat, image.Pt(1200, 400), busySession())
	var cmds []backend.ChatCommand
	for i := range 40 {
		cmds = append(cmds, backend.ChatCommand{Text: "/cmd" + strconv.Itoa(i), Description: "a command"})
	}
	u.model.chat.setCommands("s1", cmds)
	r.settle(t)
	typeIn(r, u.chats.pane("s1"), "/")
	top := r.size.Y
	for _, n := range r.nodes() {
		if strings.HasPrefix(n.Desc.Label, "/cmd") {
			top = min(top, n.Desc.Bounds.Min.Y)
		}
	}
	if top == r.size.Y {
		t.Fatalf("no list in a low pane:\n%s", r.drawn())
	}
	if top < 0 {
		t.Errorf("the list's first row at y=%d, above the pane", top)
	}
}

// /new is lich's own command: sent, it starts an empty conversation in the
// chat instead of reaching the agent as text.
func TestNewStartsAConversationInsteadOfSending(t *testing.T) {
	u, r, f, p := slashFrames(t)
	typeIn(r, p, "/ne")
	if !r.showsAll("/new", i18n.T("chat.slash.new")) {
		t.Fatalf("no /new in the list:\n%s", r.drawn())
	}
	r.press(key.NameTab)
	r.press(key.NameReturn)
	if body := f.waitCall(t, api.ChatNew.Name); !strings.Contains(body, `"id":"s1"`) {
		t.Errorf("chat.New for %s", body)
	}
	if u.model.chat.isRunning("s1") {
		t.Error("/new holds a message to send")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.HasPrefix(c, api.ChatPrompt.Name) {
			t.Errorf("/new reached the agent: %s", c)
		}
	}
}
