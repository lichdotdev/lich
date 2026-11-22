package main

import (
	"fmt"
	"image"
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

// shellListing is n lines of a command's output, as an ls prints them.
func shellListing(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("file-%02d.go", i+1)
	}
	return strings.Join(lines, "\n")
}

// A shell command's output is under it whole, as Claude Code prints it, and
// not its first line beside the check; an agent's own call keeps its one line.
func TestAShellCommandsOutputIsShownWhole(t *testing.T) {
	u, r, _, _ := shellFrames(t, claudeKind)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "user-1", Kind: api.ItemUser, Text: "!ls", Turn: 1},
		{ID: "shell-1", Kind: api.ItemTool, Turn: 1, Tool: &api.ChatToolCall{Name: "Bash", Title: "ls", Output: "AGENTS.md\nCLAUDE.md\n  README.md", State: api.ToolDone}},
		{ID: "user-2", Kind: api.ItemUser, Text: "and now?", Turn: 2},
		{ID: "tool-2", Kind: api.ItemTool, Turn: 2, Tool: &api.ChatToolCall{Name: "Bash", Title: "git status", Output: "On branch main\nnothing to commit", State: api.ToolDone}},
		{ID: "agent-2", Kind: api.ItemAgent, Text: "Clean.", Turn: 2},
	})
	r.now = r.now.Add(time.Second)
	r.frames(3)
	if !r.showsAll("AGENTS.md", "CLAUDE.md", "  README.md") {
		t.Errorf("the command's output is not shown whole:\n%s", r.drawn())
	}
	writeShot(t, r, "chat-shell-output.png")
	r.clickLabel(t, i18n.T("chat.view.worked", i18n.Args{"duration": chatTook(0), "count": 1}))
	r.frames(3)
	if r.shows("nothing to commit") {
		t.Errorf("an agent's call shows more than its line:\n%s", r.drawn())
	}
}

// Long output shows its head and a line that opens the rest.
func TestALongShellOutputOpensTheRest(t *testing.T) {
	u, r, _, _ := shellFrames(t, claudeKind)
	n := chatShellLines + 5
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "user-1", Kind: api.ItemUser, Text: "!ls", Turn: 1},
		{ID: "shell-1", Kind: api.ItemTool, Turn: 1, Tool: &api.ChatToolCall{Name: "Bash", Title: "ls", Output: shellListing(n), State: api.ToolDone}},
	})
	r.now = r.now.Add(time.Second)
	r.frames(3)
	last, more := fmt.Sprintf("file-%02d.go", n), i18n.T("chat.view.shellMore", i18n.Args{"count": 5})
	if !r.shows(fmt.Sprintf("file-%02d.go", chatShellLines)) || r.shows(last) || !r.shows(more) {
		t.Fatalf("the head and its way to the rest are not shown:\n%s", r.drawn())
	}
	r.clickLabel(t, more)
	r.frames(3)
	if !r.shows(last) || !r.shows(i18n.T("chat.view.shellLess")) {
		t.Errorf("the rest did not open:\n%s", r.drawn())
	}
}

// In shell mode the mode's name takes the chips' place, which do not apply
// to a command; deleting the "!" brings them back.
func TestShellModeNamesItselfInPlaceOfTheChips(t *testing.T) {
	_, r, _, p := shellFrames(t, claudeKind)
	mode := modeLabel(claudeKind, backend.ChatChoice{ID: "default"})
	if !r.shows(mode) {
		t.Fatalf("the box has no mode chip to begin with:\n%s", r.drawn())
	}
	typeIn(r, p, "!ls")
	if !r.shows(i18n.T("chat.composer.shell")) || r.shows(mode) {
		t.Errorf("shell mode is not named in the chips' place:\n%s", r.drawn())
	}
	p.editor.SetText("ls")
	r.frames(2)
	if r.shows(i18n.T("chat.composer.shell")) || !r.shows(mode) {
		t.Errorf("out of shell mode the chips are not back:\n%s", r.drawn())
	}
}

// The mode's name lines up with the text over it, and a narrow box cuts it
// short of Send rather than running it under the button.
func TestShellModeNameLinesUpAndFits(t *testing.T) {
	for _, width := range []int{1200, 320} {
		u, r, _ := chatFramesIn(t, api.ViewChat, image.Pt(width, 800), busySession())
		u.model.projects[0].Sessions[0].Kind = claudeKind
		u.model.chat.history("s1", nil)
		r.frame()
		p := u.chats.pane("s1")
		typeIn(r, p, "!ls")
		box := composerBox(t, r)
		var name image.Rectangle
		for _, n := range r.nodes() {
			if strings.HasPrefix(i18n.T("chat.composer.shell"), strings.TrimSuffix(n.Desc.Label, "…")) && n.Desc.Label != "" {
				name = n.Desc.Bounds
			}
		}
		if name.Empty() {
			t.Fatalf("%dpx: the mode is not named:\n%s", width, r.drawn())
		}
		if name.Min.X != box.Min.X+16 {
			t.Errorf("%dpx: the name starts at x=%d, want %d: 16px inside the box, as the text", width, name.Min.X, box.Min.X+16)
		}
		if send := box.Max.X - 34; name.Max.X > send {
			t.Errorf("%dpx: the name ends at x=%d, under Send at x=%d", width, name.Max.X, send)
		}
	}
}
