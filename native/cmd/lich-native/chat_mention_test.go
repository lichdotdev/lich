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

func TestAnAtOpensAtTheStartOfAWordAndNotInAnEmail(t *testing.T) {
	for _, c := range []struct {
		text     string
		caret    int
		query    string
		from, to int
		ok       bool
	}{
		{"@", -1, "", 0, 1, true},
		{"@src/ma", -1, "src/ma", 0, 7, true},
		{"look at @rea", -1, "rea", 8, 12, true},
		{"line\n@a", -1, "a", 5, 7, true},
		{"看看。@a", -1, "a", 3, 5, true},
		{"@app.go now", 4, "app.go", 0, 7, true},
		{"me@example.com", -1, "", 0, 0, false},
		{"foo@bar", -1, "", 0, 0, false},
		{"@app.go now", -1, "", 0, 0, false},
		{"@a b", -1, "", 0, 0, false},
		{"", -1, "", 0, 0, false},
	} {
		caret := c.caret
		if caret < 0 {
			caret = len([]rune(c.text))
		}
		query, from, to, ok := mentionQuery(c.text, caret)
		if query != c.query || from != c.from || to != c.to || ok != c.ok {
			t.Errorf("%q at %d: %q [%d,%d) %v; want %q [%d,%d) %v", c.text, caret, query, from, to, ok, c.query, c.from, c.to, c.ok)
		}
	}
}

// The orders Claude Code 2.1.330's own file index gives for this listing, its
// search run under node on the same paths.
func TestTheAtListRanksAsClaudeCodeDoes(t *testing.T) {
	f := newChatFiles([]string{"app.go", "youtrack/client_test.go", "youtrack/api/client.go",
		"native/cmd/lich-native/chat_slash.go", "native/cmd/lich-native/chat_slash_test.go",
		"internal/shellintegration/shellintegration.go", "docs/hooks/README.md", "README.md",
		"README.pt-BR.md", "src/UserCard.tsx", "src/usercard_old.ts"})
	x := newMentionIndex(f.files, f.dirs)
	for query, want := range map[string][]string{
		"client":   {"youtrack/client_test.go", "youtrack/api/client.go"},
		"slash.go": {"native/cmd/lich-native/chat_slash.go", "native/cmd/lich-native/chat_slash_test.go", "internal/shellintegration/shellintegration.go"},
		"readme":   {"README.md", "README.pt-BR.md", "docs/hooks/README.md"},
		"UC":       {"src/UserCard.tsx"},
		"uc":       {"src/UserCard.tsx", "src/usercard_old.ts", "youtrack/", "youtrack/api/", "youtrack/client_test.go", "youtrack/api/client.go"},
		"ytc":      {"youtrack/", "youtrack/api/", "youtrack/client_test.go", "youtrack/api/client.go"},
		"api/":     {"youtrack/api/", "youtrack/api/client.go"},
		"./hooks":  {"docs/hooks/", "docs/hooks/README.md"},
		"zzz":      {},
	} {
		if got := x.search(query, mentionLimit); !slices.Equal(got, want) {
			t.Errorf("%q: %q, want %q", query, got, want)
		}
	}
	if got, want := x.search("", mentionLimit), []string{"src/", "docs/", "app.go", "native/", "README.md", "internal/", "youtrack/", "README.pt-BR.md"}; !slices.Equal(got, want) {
		t.Errorf("a bare @ offers %q, want %q", got, want)
	}
}

// mentionFrames is a chat in pathsCheckout with its listing read, the
// composer focused.
func mentionFrames(t *testing.T) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := pathFrames(t, "hello")
	u.model.chat.setCommands("s1", []backend.ChatCommand{{Text: "/review", Description: "Review a pull request"}})
	r.settle(t)
	return u, r, f, u.chats.pane("s1")
}

// "@" opens the checkout's folders and files, typing filters them, and Enter
// or a click puts the path in place of what was typed, with a space, unsent.
func TestTheAtListOpensFiltersAndPicks(t *testing.T) {
	u, r, f, p := mentionFrames(t)
	typeIn(r, p, "look at @")
	if !r.showsAll("app.go", "youtrack/") {
		t.Fatalf("no paths on @:\n%s", r.drawn())
	}
	typeIn(r, p, "look at @cli")
	if !r.showsAll("client.go", "client_test.go", "youtrack/api/") || r.shows("app.go") {
		t.Fatalf("@cli lists more than the clients:\n%s", r.drawn())
	}
	r.press(key.NameDownArrow)
	r.press(key.NameReturn)
	if got, want := p.editor.Text(), "look at @"+rankedAt(t, "cli", 1)+" "; got != want {
		t.Errorf("Down and Enter left %q, want %q", got, want)
	}
	r.frame()
	if r.shows("client_test.go") {
		t.Error("the list stays open once the path is in")
	}
	if u.model.chat.isRunning("s1") {
		t.Error("Enter on the list holds a message to send")
	}
	time.Sleep(50 * time.Millisecond)
	if f.callsOf(api.ChatPrompt.Name) > 0 {
		t.Error("Enter on the list sent the message")
	}
	typeIn(r, p, "@app")
	r.clickLabel(t, "app.go")
	if got := p.editor.Text(); got != "@app.go " {
		t.Errorf("a click left %q", got)
	}
}

// rankedAt is what the @ list ranks i-th for query over pathsListing.
func rankedAt(t *testing.T, query string, i int) string {
	t.Helper()
	f := newChatFiles(pathsListing.Files)
	got := newMentionIndex(f.files, f.dirs).search(query, mentionLimit)
	if i >= len(got) {
		t.Fatalf("%q ranks %q", query, got)
	}
	return got[i]
}

// Tab puts in what every path shown starts with and keeps the list open on
// it, then picks once nothing longer is shared, as Claude Code's Tab does.
func TestTabCompletesWhatThePathsShare(t *testing.T) {
	_, r, _, p := mentionFrames(t)
	typeIn(r, p, "@youtrack/a")
	r.press(key.NameTab)
	if got := p.editor.Text(); got != "@youtrack/api/" {
		t.Fatalf("Tab left %q, want the shared folder", got)
	}
	r.frame()
	if !r.shows("client.go") {
		t.Fatalf("the list shut on the folder:\n%s", r.drawn())
	}
	// the folder itself leads, as in Claude Code's index
	r.press(key.NameDownArrow)
	r.press(key.NameTab)
	if got := p.editor.Text(); got != "@youtrack/api/client.go " {
		t.Errorf("Down and Tab left %q", got)
	}
}

// mentionSentFrames is sentFrames with pathsListing as its checkout's listing.
func mentionSentFrames(t *testing.T, running bool) (*frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	f.answers = map[string]any{api.ProjectTree.Name: pathsListing}
	u.model.projects[0].Path = pathsCheckout
	u.model.client = u.client
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "ship the docs"}})
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: running})
	r.frames(3)
	f.waitCall(t, api.ProjectTree.Name)
	r.frames(3)
	return r, f, u.chats.pane("s1")
}

// Escape shuts the list and keeps it shut while the word stays, and stops no
// turn; the next Escape, with the list shut, does. The next character opens
// the list again.
func TestEscapeShutsTheAtList(t *testing.T) {
	r, f, p := mentionSentFrames(t, true)
	typeIn(r, p, "@ap")
	if !r.shows("app.go") {
		t.Fatalf("no list on @ap:\n%s", r.drawn())
	}
	r.press(key.NameEscape)
	r.frame()
	if r.shows("app.go") {
		t.Fatalf("Escape left the list open:\n%s", r.drawn())
	}
	if got := p.editor.Text(); got != "@ap" {
		t.Errorf("Escape changed the text to %q", got)
	}
	noCancel(t, f)
	typeIn(r, p, "@app")
	if !r.shows("app.go") {
		t.Error("typing on after Escape does not open the list again")
	}
	r.press(key.NameEscape)
	r.press(key.NameEscape)
	f.waitCall(t, api.ChatCancel.Name)
}

// The arrows walk the open list, not the messages sent; with the list shut,
// ↑ walks them again.
func TestTheArrowsWalkTheAtListNotTheHistory(t *testing.T) {
	r, _, p := mentionSentFrames(t, false)
	typeIn(r, p, "@")
	r.press(key.NameUpArrow)
	if got := p.editor.Text(); got != "@" {
		t.Errorf("Up on the list put %q in the box", got)
	}
	r.press(key.NameEscape)
	r.frame()
	r.press(key.NameUpArrow)
	if got := p.editor.Text(); got != "ship the docs" {
		t.Errorf("Up with the list shut left %q, want the message sent", got)
	}
}

// A slash command and an @ live in one line: the slash list opens on the
// command, the @ list once the command has its space.
func TestTheAtListAndTheSlashListTakeTurns(t *testing.T) {
	_, r, _, p := mentionFrames(t)
	typeIn(r, p, "/rev")
	if !r.shows("/review") || r.shows("app.go") {
		t.Fatalf("/rev shows the wrong list:\n%s", r.drawn())
	}
	typeIn(r, p, "/review @ap")
	if r.shows("Review a pull request") || !r.shows("app.go") {
		t.Fatalf("/review @ap shows the wrong list:\n%s", r.drawn())
	}
}

// An @ in an email or mid-word opens nothing, and Enter sends the message as
// typed, the @path in it: the agent reads the path itself.
func TestNoAtListLeavesEnterToSend(t *testing.T) {
	for _, text := range []string{"mail me@app.go", "see @zzz", "read @app.go "} {
		_, r, f, p := mentionFrames(t)
		typeIn(r, p, text)
		if r.shows("youtrack/") {
			t.Errorf("%q opened the list:\n%s", text, r.drawn())
		}
		r.press(key.NameReturn)
		if body := f.waitCall(t, api.ChatPrompt.Name); !strings.Contains(body, strings.TrimSpace(text)) {
			t.Errorf("%q sent %s", text, body)
		}
	}
}
