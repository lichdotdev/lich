package main

import (
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
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
	r.click(t, r.at(t, "2 shells"))
	if !r.showsAll("Background shells", "npm run dev", "tail -f log.txt", "go test ./...", "failed · 4s") || !strings.Contains(r.drawn(), `"running · 1m `) {
		t.Fatalf("the shells are not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "npm run dev"))
	toolsSettle(r)
	if !r.showsAll("Start the dev server", "ready on :5173", "watching for changes", "Stop") {
		t.Fatalf("the shell's detail is not drawn:\n%s", r.drawn())
	}
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
