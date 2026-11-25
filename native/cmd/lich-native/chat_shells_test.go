package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// shellsFrames is session s1 drawn as a Claude Code chat with two shells
// running and one ended, the backend answering a shell's output.
func shellsFrames(t *testing.T) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	f.answers = map[string]any{api.ChatShellOutput.Name: "ready on :5173\nwatching for changes\n"}
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}})
	start := r.now.Add(-75 * time.Second).UnixMilli()
	u.model.chat.setShells("s1", []backend.ChatShell{
		{ID: "b1", Command: "npm run dev", Description: "Start the dev server", State: api.ShellRunning, Started: start},
		{ID: "b2", Command: "go test ./...", State: api.ShellFailed, Started: start, Ended: start + 4000},
		{ID: "b3", Command: "tail -f log.txt", State: api.ShellRunning, Started: start},
	})
	r.settle(t)
	return u, r, f, u.chats.pane("s1")
}

// The chip counts the shells running and opens them, each with its state and
// running time; one opens on a click with its output, and Stop stops it.
func TestShellsChipOpensTheShellsAndStopStopsOne(t *testing.T) {
	_, r, f, p := shellsFrames(t)
	writeShot(t, r, "shells-chip.png")
	r.click(t, r.at(t, "2 shells"))
	writeShot(t, r, "shells-list.png")
	if !r.showsAll("Background shells", "npm run dev", "tail -f log.txt", "go test ./...", "failed", " · 4s") || !strings.Contains(r.drawn(), `"1m `) {
		t.Fatalf("the shells are not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "npm run dev"))
	toolsSettle(r)
	if !r.showsAll("Start the dev server", "ready on :5173", "watching for changes", "Stop") {
		t.Fatalf("the shell's detail is not drawn:\n%s", r.drawn())
	}
	writeShot(t, r, "shells-detail.png")
	if body := f.waitCall(t, api.ChatShellOutput.Name); !strings.Contains(body, `"shell":"b1"`) {
		t.Errorf("read the output of %s", body)
	}
	r.click(t, r.at(t, "Stop"))
	if body := f.waitCall(t, api.ChatStopShell.Name); !strings.Contains(body, `"shell":"b1"`) {
		t.Errorf("Stop asked %s", body)
	}
	r.press(key.NameEscape)
	if !r.shows("Background shells") {
		t.Errorf("Escape did not go back to the list:\n%s", r.drawn())
	}
	r.press(key.NameEscape)
	if p.tools.opened() {
		t.Errorf("a second Escape left the panel open:\n%s", r.drawn())
	}
}

// The list groups the shells that run apart from those that ended, a failure
// names its exit code where the provider gave one, and the × shuts the panel.
func TestShellsGroupNameAnExitCodeAndClose(t *testing.T) {
	u, r, _, p := shellsFrames(t)
	code := 3
	shells := u.model.chat.shellsOf("s1")
	shells[1].ExitCode = &code
	u.model.chat.setShells("s1", shells)
	r.frames(2)
	r.click(t, r.at(t, "2 shells"))
	if !r.showsAll("RUNNING", "ENDED", "2 running", "exit 3", "close") || r.shows("failed") {
		t.Fatalf("the groups or the exit code are not drawn:\n%s", r.drawn())
	}
	// the panel grows in before its × takes a click (contract changed: it opens
	// with a motion)
	r.frames(1)
	r.click(t, r.controlOf(t, "2 running"))
	if p.tools.opened() {
		t.Errorf("the × left the panel open:\n%s", r.drawn())
	}
}

// A Monitor's watch is counted apart from the shells, as Claude Code's TUI
// counts it, and named a monitor in the list.
func TestMonitorsAreCountedApartAndNamed(t *testing.T) {
	u, r, _, _ := shellsFrames(t)
	start := r.now.Add(-5 * time.Second).UnixMilli()
	u.model.chat.setShells("s1", append(u.model.chat.shellsOf("s1"),
		backend.ChatShell{ID: "m1", Command: "tail -f build.log", State: api.ShellRunning, Started: start, Monitor: true}))
	r.frames(2)
	r.click(t, r.at(t, "2 shells · 1 monitor"))
	if !r.shows("tail -f build.log") || !strings.Contains(r.drawn(), `"monitor · `) {
		t.Fatalf("the monitor is not named in the list:\n%s", r.drawn())
	}
	u.model.chat.setShells("s1", []backend.ChatShell{{ID: "m1", Command: "tail -f build.log", State: api.ShellRunning, Started: start, Monitor: true}})
	r.press(key.NameEscape)
	r.frames(2)
	if !r.shows("1 monitor") || r.shows("shell") {
		t.Errorf("a monitor alone is not counted as one:\n%s", r.drawn())
	}
}

// A shell that ended offers no Stop, and with none running there is no chip.
func TestEndedShellsHaveNoStopAndNoChip(t *testing.T) {
	u, r, _, _ := shellsFrames(t)
	r.click(t, r.at(t, "2 shells"))
	r.click(t, r.at(t, "go test ./..."))
	toolsSettle(r)
	if r.shows("Stop") {
		t.Errorf("an ended shell offers Stop:\n%s", r.drawn())
	}
	u.model.chat.setShells("s1", []backend.ChatShell{{ID: "b2", Command: "go test ./...", State: api.ShellFailed, Started: 1, Ended: 2}})
	r.press(key.NameEscape)
	r.press(key.NameEscape)
	r.frames(2)
	if r.shows("shell") {
		t.Errorf("a chip counts no shell running:\n%s", r.drawn())
	}
}

// /tasks, typed as in Claude Code, opens the shells instead of reaching the
// agent.
func TestTasksCommandOpensTheShells(t *testing.T) {
	u, r, f, p := shellsFrames(t)
	typeIn(r, p, "/tasks")
	r.press(key.NameReturn)
	toolsSettle(r)
	if !r.shows("Background shells") || u.model.chat.isRunning("s1") || f.callsOf(api.ChatPrompt.Name) != 0 {
		t.Errorf("/tasks did not open the shells:\n%s", r.drawn())
	}
}

// Opened from the chip, the panel leaves what was typed in the box, and typing
// more shuts it.
func TestTypingShutsTheShells(t *testing.T) {
	_, r, _, p := shellsFrames(t)
	typeIn(r, p, "draft")
	r.click(t, r.at(t, "2 shells"))
	if !p.tools.opened() || p.editor.Text() != "draft" {
		t.Fatalf("the chip did not open the panel over the draft %q", p.editor.Text())
	}
	typeIn(r, p, "!")
	if p.tools.opened() {
		t.Error("typing left the panel open")
	}
}

// A shell the agent leaves running after its turn ends reaches the chip
// through /events, the way the backend sends it.
func TestShellsEventThroughTheStreamShowsTheChip(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}})
	events := make(chan backend.Event, 1)
	events <- chatEventOf(t, api.ChatShellsEvent, api.ChatShellsUpdate{SessionID: "s1", Shells: []backend.ChatShell{
		{ID: "bkjocc3vb", Command: "sleep 300", State: api.ShellRunning, Started: r.now.UnixMilli()},
	}})
	close(events)
	u.model.follow(t.Context(), events)
	r.frames(2)
	if !r.shows("1 shell") {
		t.Errorf("no chip for the shell the event brought:\n%s", r.drawn())
	}
}

// The shells event replaces the session's shells in the store.
func TestShellsEventReachesTheStore(t *testing.T) {
	u, r, _, _ := shellsFrames(t)
	u.model.chatEvent(t.Context(), chatEventOf(t, api.ChatShellsEvent, api.ChatShellsUpdate{SessionID: "s1", Shells: []backend.ChatShell{}}))
	r.frames(2)
	if len(u.model.chat.shellsOf("s1")) != 0 || r.shows("2 shells") {
		t.Errorf("the event left %v and the chip:\n%s", u.model.chat.shellsOf("s1"), r.drawn())
	}
}

// A turn waiting on a command Claude Code can move shows its Ctrl+B hint in
// the live line, and Ctrl+B in the box moves it; a call that cannot move
// shows no hint and leaves the key alone.
func TestCtrlBMovesTheCommandTheTurnWaitsOn(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	call := backend.ChatItem{ID: "toolu_1", Kind: api.ItemTool, Turn: 1, At: r.now.Add(-3 * time.Second).UnixMilli(),
		Tool: &backend.ChatToolCall{Name: "Bash", Title: "npm test", State: api.ToolRunning, Movable: true}}
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go", At: r.now.Add(-12 * time.Second).UnixMilli()}, call})
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frames(3)
	writeShot(t, r, "chat-background-hint.png")
	if !r.shows("· Ctrl+B to run in background") {
		t.Fatalf("no hint while the turn waits on a command:\n%s", r.drawn())
	}
	r.pressWith("B", key.ModCtrl)
	f.waitCall(t, api.ChatBackground.Name)

	// An earlier turn's call left unended is not the turn in flight's.
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}, call, {ID: "u2", Kind: api.ItemUser, Turn: 2, Text: "and now"}})
	r.frames(2)
	if r.shows("Ctrl+B") {
		t.Errorf("a hint for an earlier turn's call:\n%s", r.drawn())
	}
	call.Tool.Movable = false
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}, call})
	r.frames(2)
	if r.shows("Ctrl+B") {
		t.Errorf("a hint for a call that cannot move:\n%s", r.drawn())
	}
	r.pressWith("B", key.ModCtrl)
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatBackground.Name); n != 1 {
		t.Errorf("Ctrl+B with nothing to move asked %d times in all", n)
	}
}

// Ctrl+B is also New worktree's default chord, read before the panes: while
// the box holds it for a command to move, the chord is the chat's.
func TestCtrlBInTheComposerIsNotNewWorktree(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	u.settings = new(settingsScreen)
	var fired []hotkeyID
	draw := r.draw
	r.draw = func(gtx C, th *ui.Theme) {
		// the window reads its chords before it lays the panes out
		keeps := u.chatKeeps(gtx, "linux", false)
		fired = append(fired, pollChords(gtx, &u.settings.hotkeys, "linux", false, func(id hotkeyID) bool {
			return id == hotkeyNewWorktree && !keeps(id)
		})...)
		draw(gtx, th)
	}
	call := backend.ChatItem{ID: "toolu_1", Kind: api.ItemTool, Turn: 1, At: r.now.Add(-3 * time.Second).UnixMilli(),
		Tool: &backend.ChatToolCall{Name: "Bash", Title: "npm test", State: api.ToolRunning, Movable: true}}
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}, call})
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frames(3)
	r.pressWith("B", key.ModCtrl)
	f.waitCall(t, api.ChatBackground.Name)
	if slices.Contains(fired, hotkeyNewWorktree) {
		t.Errorf("Ctrl+B in the box also ran New worktree: %v", fired)
	}
}
