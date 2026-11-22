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

func modeOption(current string, choices ...backend.ChatChoice) backend.ChatOption {
	return backend.ChatOption{ID: "permission_mode", Category: api.OptionMode, Current: current, Choices: choices}
}

func choiceIDs(ids ...string) []backend.ChatChoice {
	out := make([]backend.ChatChoice, len(ids))
	for i, id := range ids {
		out[i] = backend.ChatChoice{ID: id}
	}
	return out
}

func TestNextModeWalksEachProvidersCycle(t *testing.T) {
	bypass := backend.ChatChoice{ID: "bypassPermissions", Danger: true}
	claude := append(choiceIDs("default", "acceptEdits", "plan", "dontAsk"), bypass)
	claudeAuto := append(choiceIDs("default", "acceptEdits", "plan", "auto", "dontAsk"), bypass)
	codex := append(choiceIDs(":read-only", ":workspace"), backend.ChatChoice{ID: ":danger-full-access", Danger: true})
	for name, c := range map[string]struct {
		kind, current string
		choices       []backend.ChatChoice
		want          string
	}{
		"claude default":           {claudeKind, "default", claude, "acceptEdits"},
		"claude accept edits":      {claudeKind, "acceptEdits", claude, "plan"},
		"claude plan, no auto":     {claudeKind, "plan", claude, "default"},
		"claude plan, auto":        {claudeKind, "plan", claudeAuto, "auto"},
		"claude auto":              {claudeKind, "auto", claudeAuto, "default"},
		"claude dontAsk":           {claudeKind, "dontAsk", claude, "default"},
		"claude bypass, auto":      {claudeKind, "bypassPermissions", claudeAuto, "auto"},
		"claude bypass, no auto":   {claudeKind, "bypassPermissions", claude, "default"},
		"codex read-only":          {codexKind, ":read-only", codex, ":workspace"},
		"codex workspace":          {codexKind, ":workspace", codex, ":read-only"},
		"codex full access":        {codexKind, ":danger-full-access", codex, ":read-only"},
		"opencode build":           {opencodeKind, "build", choiceIDs("build", "plan"), "plan"},
		"opencode plan":            {opencodeKind, "plan", choiceIDs("build", "plan"), "build"},
		"kiro agent of the person": {kiroKind, "mine", choiceIDs("kiro_default", "kiro_planner", "mine"), "kiro_default"},
	} {
		if got, ok := nextMode(c.kind, modeOption(c.current, c.choices...)); !ok || got != c.want {
			t.Errorf("%s: %q, %v; want %q", name, got, ok, c.want)
		}
	}
	if got, ok := nextMode(cursorKind, modeOption("agent", choiceIDs("agent")...)); ok {
		t.Errorf("one mode cycled to %q", got)
	}
}

// shellFrames is session s1, of kind, drawn as a chat with Claude Code's
// modes.
func shellFrames(t *testing.T, kind string) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.projects[0].Sessions[0].Kind = kind
	u.model.chat.history("s1", nil)
	u.model.chat.setOptions("s1", []backend.ChatOption{modeOption("default", choiceIDs("default", "acceptEdits", "plan")...)})
	r.frame()
	return u, r, f, u.chats.pane("s1")
}

// Shift+Tab in the box sets the next mode and leaves the keys in the box.
func TestShiftTabCyclesTheMode(t *testing.T) {
	_, r, f, p := shellFrames(t, claudeKind)
	typeIn(r, p, "draft")
	r.pressWith(key.NameTab, key.ModShift)
	if body := f.waitCall(t, api.ChatSetOption.Name); !strings.Contains(body, `"value":"acceptEdits"`) {
		t.Errorf("Shift+Tab set %s", body)
	}
	r.frame()
	r.typeText(p, "!")
	if got := p.editor.Text(); got != "draft!" {
		t.Errorf("the box holds %q after Shift+Tab, want the keys still in it", got)
	}
}

// While the agent works the mode's chip takes no click, and Shift+Tab sets
// nothing either.
func TestShiftTabWaitsForTheTurn(t *testing.T) {
	u, r, f, _ := shellFrames(t, claudeKind)
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frame()
	r.pressWith(key.NameTab, key.ModShift)
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatSetOption.Name); n != 0 {
		t.Errorf("Shift+Tab during a turn set the mode %d times", n)
	}
}

// A "!" line wears the shell mode and Enter runs it in the shell, with no
// hold and no prompt.
func TestBangLineRunsInTheShell(t *testing.T) {
	u, r, f, p := shellFrames(t, claudeKind)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "user-1", Kind: api.ItemUser, Text: "!git log --oneline -1", Turn: 1},
		{ID: "shell-1", Kind: api.ItemTool, Turn: 1, Tool: &api.ChatToolCall{Name: "Bash", Title: "git log --oneline -1", Output: "1b4df27b Merge branch 'file-mentions'", State: api.ToolDone}},
	})
	r.now = r.now.Add(time.Second)
	r.frames(3)
	typeIn(r, p, "!git status ")
	if !r.shows("1b4df27b Merge branch 'file-mentions'") {
		t.Errorf("the command's output is folded away:\n%s", r.drawn())
	}
	if !r.shows(i18n.T("chat.composer.shell")) {
		t.Errorf("the box does not name the shell mode:\n%s", r.drawn())
	}
	writeShot(t, r, "chat-shell-mode.png")
	r.press(key.NameReturn)
	if body := f.waitCall(t, api.ChatRunShell.Name); !strings.Contains(body, `"command":"git status"`) {
		t.Errorf("Enter ran %s", body)
	}
	r.frame()
	if n := f.callsOf(api.ChatPrompt.Name); n != 0 {
		t.Errorf("the command also went as %d prompts", n)
	}
	if p.editor.Len() != 0 {
		t.Error("the box kept the command")
	}
	if r.shows(i18n.T("chat.composer.shell")) {
		t.Error("the empty box still names the shell mode")
	}
}

// A "!" line while the agent works goes to the shell, which refuses it, and
// never waits in the queue.
func TestBangLineDuringATurnIsNotQueued(t *testing.T) {
	u, r, f, p := shellFrames(t, codexKind)
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	typeIn(r, p, "!ls")
	r.press(key.NameReturn)
	f.waitCall(t, api.ChatRunShell.Name)
	if q := u.model.chat.queued("s1"); len(q) != 0 {
		t.Errorf("queued %q", q)
	}
}

// Where the provider runs no command of the person's, a "!" line is a
// message like any other; and a file attached keeps it one.
func TestBangLineIsTextWhereTheProviderHasNoShell(t *testing.T) {
	_, r, f, p := shellFrames(t, "omp")
	typeIn(r, p, "!ls")
	if r.shows(i18n.T("chat.composer.shell")) {
		t.Error("an oh-my-pi box names the shell mode")
	}
	r.press(key.NameReturn)
	r.now = r.now.Add(chatSendHold + time.Second)
	r.frames(3)
	if body := f.waitCall(t, api.ChatPrompt.Name); !strings.Contains(body, `"text":"!ls"`) {
		t.Errorf("sent %s", body)
	}
	if n := f.callsOf(api.ChatRunShell.Name); n != 0 {
		t.Errorf("ran %d shell commands", n)
	}

	_, r, _, p = shellFrames(t, claudeKind)
	p.attached = []backend.Attachment{{Path: "/tmp/a.png"}}
	typeIn(r, p, "!ls")
	if _, shell := p.shellLine(claudeKind); shell || r.shows(i18n.T("chat.composer.shell")) {
		t.Error("a line with a file attached is a shell command")
	}
}

// The calls of a turn the person ran in the shell stand open; a "!" message
// the agent answered, or any other, keeps the usual fold.
func TestAShellTurnsCallStandsOpen(t *testing.T) {
	call := backend.ChatItem{ID: "c", Kind: api.ItemTool, Turn: 1, Tool: &api.ChatToolCall{Name: "Bash", State: api.ToolDone}}
	for name, c := range map[string]struct {
		items []backend.ChatItem
		want  bool
	}{
		"shell":           {[]backend.ChatItem{{ID: "u", Kind: api.ItemUser, Text: "!ls", Turn: 1}, call}, true},
		"agent answered":  {[]backend.ChatItem{{ID: "u", Kind: api.ItemUser, Text: "!ls", Turn: 1}, call, {ID: "a", Kind: api.ItemAgent, Text: "ok", Turn: 1}}, false},
		"a plain message": {[]backend.ChatItem{{ID: "u", Kind: api.ItemUser, Text: "ls", Turn: 1}, call}, false},
	} {
		for _, r := range chatRows(c.items, false, time.Time{}) {
			if r.kind == chatRowWork && r.shell != c.want {
				t.Errorf("%s: shell %v, want %v", name, r.shell, c.want)
			}
		}
	}
}
