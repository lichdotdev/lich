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

const testSuggestion = "run the tests"

// suggestFrames is session s1 after a finished turn, its box empty, with a
// suggestion heard.
func suggestFrames(t *testing.T) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f, p := shellFrames(t, claudeKind)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "fix the bug"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "Fixed."},
	})
	u.model.chat.suggest(api.ChatSuggestion{SessionID: "s1", Text: testSuggestion})
	r.settle(t)
	return u, r, f, p
}

// placeholderText is the box's own placeholder: the rig lists no providers,
// so it names the kind.
func placeholderText() string {
	return i18n.T("chat.composer.placeholder", i18n.Args{"provider": claudeKind})
}

// shownInstead reports whether the box draws the suggestion and not its
// placeholder.
func shownInstead(r *frameRig) bool {
	return r.shows(testSuggestion) && !r.shows(placeholderText())
}

func TestTheIdleBoxShowsTheSuggestionWhereItsPlaceholderWas(t *testing.T) {
	u, r, _, _ := suggestFrames(t)
	if !shownInstead(r) {
		t.Fatalf("the box does not show the suggestion in place of its placeholder:\n%s", r.drawn())
	}
	box := composerBox(t, r)
	if in := labelLeft(t, r, testSuggestion) - box.Min.X; in < 14 || in > 17 {
		t.Errorf("the suggestion starts %dpx in from the box's edge, want the placeholder's 16", in)
	}
	u.model.chat.suggest(api.ChatSuggestion{SessionID: "s1"})
	r.frame()
	if r.shows(testSuggestion) {
		t.Error("an empty suggestion did not clear it")
	}
}

func TestTheSuggestionWaitsForAnIdleBox(t *testing.T) {
	for name, busy := range map[string]func(*window, *chatPane){
		"a turn runs":     func(u *window, _ *chatPane) { u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true}) },
		"a message waits": func(u *window, _ *chatPane) { u.model.chat.hold("s1", chatMessage{text: "go"}, time.Now()) },
		"the queue holds": func(u *window, _ *chatPane) { u.model.chat.enqueue("s1", chatMessage{text: "next"}) },
		"a file is attached": func(_ *window, p *chatPane) {
			p.attached = []backend.Attachment{{Path: "/home/me/repo/notes.md"}}
		},
	} {
		u, r, _, p := suggestFrames(t)
		busy(u, p)
		r.frame()
		if r.shows(testSuggestion) {
			t.Errorf("%s: the box still shows the suggestion", name)
		}
	}
}

func TestTabPutsTheSuggestionInTheBoxAndEnterSendsIt(t *testing.T) {
	_, r, f, p := suggestFrames(t)
	r.press(key.NameTab)
	if got := p.editor.Text(); got != testSuggestion {
		t.Fatalf("after Tab the box holds %q, want the suggestion", got)
	}
	r.typeText(p, " again")
	r.press(key.NameReturn)
	r.now = r.now.Add(chatSendHold + time.Second)
	r.frame()
	if body := f.waitCall(t, api.ChatPrompt.Name); !strings.Contains(body, `"text":"run the tests again"`) {
		t.Errorf("Enter sent %s", body)
	}
}

func TestEnterOnTheEmptyBoxDoesNotSendTheSuggestion(t *testing.T) {
	_, r, f, _ := suggestFrames(t)
	r.press(key.NameReturn)
	r.now = r.now.Add(chatSendHold + time.Second)
	r.frame()
	time.Sleep(50 * time.Millisecond)
	if n := f.callsOf(api.ChatPrompt.Name); n != 0 {
		t.Errorf("Enter on the empty box sent %d messages", n)
	}
	if !r.shows(testSuggestion) {
		t.Error("Enter took the suggestion away")
	}
}

// Typing hides the suggestion and erasing brings it back: a start given up
// does not cost it. A message that goes, sent or queued, does.
func TestTheSuggestionOutlivesTypingButNotAMessage(t *testing.T) {
	u, r, _, p := suggestFrames(t)
	for range 3 {
		r.typeText(p, "x")
		r.frame()
		if r.shows(testSuggestion) {
			t.Fatalf("the box shows the suggestion over its text:\n%s", r.drawn())
		}
		p.editor.SetText("")
		r.frame()
		if !shownInstead(r) {
			t.Fatalf("emptied again, the box lost the suggestion:\n%s", r.drawn())
		}
	}
	for name, send := range map[string]func(){
		"sent":   func() { u.model.chat.hold("s1", chatMessage{text: "go"}, time.Now()) },
		"queued": func() { u.model.chat.enqueue("s1", chatMessage{text: "next"}) },
	} {
		u.model.chat.suggest(api.ChatSuggestion{SessionID: "s1", Text: testSuggestion})
		send()
		if got := u.model.chat.suggestionOf("s1"); got != "" {
			t.Errorf("a message %s kept the suggestion %q", name, got)
		}
	}
}

func TestShiftTabCyclesTheModeOverASuggestion(t *testing.T) {
	_, r, f, p := suggestFrames(t)
	r.pressWith(key.NameTab, key.ModShift)
	if body := f.waitCall(t, api.ChatSetOption.Name); !strings.Contains(body, `"value":"acceptEdits"`) {
		t.Errorf("Shift+Tab set %s", body)
	}
	if p.editor.Len() != 0 || !r.shows(testSuggestion) {
		t.Errorf("Shift+Tab took the suggestion: the box holds %q", p.editor.Text())
	}
}

func TestANewConversationOrAnotherProcessDropsTheSuggestion(t *testing.T) {
	for name, drop := range map[string]func(*chatStore){
		"history read again": func(s *chatStore) { s.history("s1", nil) },
		"view switched":      func(s *chatStore) { s.switched("s1") },
	} {
		var s chatStore
		s.suggest(api.ChatSuggestion{SessionID: "s1", Text: testSuggestion})
		drop(&s)
		if got := s.suggestionOf("s1"); got != "" {
			t.Errorf("%s: the suggestion %q stayed", name, got)
		}
	}
}

// The suggestion reaches the box through /events, the way the backend sends it.
func TestSuggestionEventThroughTheStreamShowsInTheBox(t *testing.T) {
	u, r, _, _ := shellFrames(t, claudeKind)
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "fix it"}})
	events := make(chan backend.Event, 1)
	events <- chatEventOf(t, api.ChatSuggestionEvent, api.ChatSuggestion{SessionID: "s1", Text: testSuggestion})
	close(events)
	u.model.follow(t.Context(), events)
	r.frames(2)
	if !shownInstead(r) {
		t.Errorf("the suggestion the event brought is not in the box:\n%s", r.drawn())
	}
}
