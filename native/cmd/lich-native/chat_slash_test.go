package main

import (
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
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
	_, r, f, p := slashFrames(t)
	typeIn(r, p, "/")
	if !r.showsAll("/review", "[pr]", "/deploy", "/compact", "Ship it") {
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
	typeIn(r, p, "/")
	r.press(key.NameDownArrow)
	r.press(key.NameReturn)
	if got := p.editor.Text(); got != "/deploy " {
		t.Errorf("Down and Enter left %q, want /deploy", got)
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
