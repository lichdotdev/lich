package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// The shapes below are Claude Code's own, cut from real transcripts.

const subagentNote = "<task-notification>\n<task-id>a5d3b2dab4b414f74</task-id>\n<tool-use-id>toolu_01TJG7WJ7zFkaUCsgMKVfVMS</tool-use-id>\n" +
	"<output-file>/tmp/claude-1000/x/tasks/a5d3b2dab4b414f74.output</output-file>\n<status>completed</status>\n" +
	"<summary>Agent \"Survey 8 providers for advisor\" finished</summary>\n" +
	"<note>A task-notification fires each time this agent stops with no live background children of its own.</note>\n" +
	"<result>Short version: `lich mcp` reaches all eight providers. A quoted </result> stays in.</result>\n" +
	"<usage><subagent_tokens>93238</subagent_tokens><tool_uses>43</tool_uses><duration_ms>346491</duration_ms></usage>\n" +
	"</task-notification>"

const commandNote = "<task-notification>\n<task-id>bl2ba2mvz</task-id>\n<tool-use-id>toolu_01KP4opuS7hZqrLSvMbJQ1bm</tool-use-id>\n" +
	"<output-file>/tmp/claude-1000/x/bl2ba2mvz.output</output-file>\n<status>failed</status>\n" +
	"<summary>Background command \"go test ./internal/chat/...\" failed with exit code 1</summary>\n</task-notification>"

const monitorNote = "<task-notification>\n<task-id>boc4i2t01</task-id>\n<summary>Monitor event: \"local gate step results\"</summary>\n" +
	"<event>gofmt exit=0</event>\nIf this event is something the user would act on now, send a PushNotification.\n</task-notification>"

const preamble = "[SYSTEM NOTIFICATION - NOT USER INPUT]\nThis is an automated background-task event, NOT a message from the user.\n\n"

func TestParseTaskNotesReadsClaudeCodesShapes(t *testing.T) {
	cases := []struct {
		name, text string
		want       []taskNote
	}{
		{"subagent", subagentNote, []taskNote{{status: "completed", summary: `Agent "Survey 8 providers for advisor" finished`,
			result: "Short version: `lich mcp` reaches all eight providers. A quoted </result> stays in."}}},
		{"command", commandNote, []taskNote{{status: "failed", summary: `Background command "go test ./internal/chat/..." failed with exit code 1`}}},
		{"monitor", monitorNote, []taskNote{{summary: `Monitor event: "local gate step results"`, event: "gofmt exit=0"}}},
		{"preamble", preamble + commandNote, []taskNote{{status: "failed", summary: `Background command "go test ./internal/chat/..." failed with exit code 1`}}},
		{"several", commandNote + "\n" + monitorNote, []taskNote{
			{status: "failed", summary: `Background command "go test ./internal/chat/..." failed with exit code 1`},
			{summary: `Monitor event: "local gate step results"`, event: "gofmt exit=0"},
		}},
	}
	for _, c := range cases {
		got, ok := parseTaskNotes(c.text)
		if !ok || !slices.Equal(got, c.want) {
			t.Errorf("%s: parseTaskNotes = %+v, %v, want %+v", c.name, got, ok, c.want)
		}
	}
}

func TestParseTaskNotesLeavesOtherMessagesAlone(t *testing.T) {
	for _, text := range []string{
		"", "fix the bug", taskNoteOpen,
		"what does this mean? " + commandNote,
		preamble + "no note here",
		"<task-notification>\n<status>completed</status>\n</task-notification>",
	} {
		if _, ok := parseTaskNotes(text); ok {
			t.Errorf("%q read as a task notification", text)
		}
	}
}

var claudeNotes = []backend.ChatItem{
	{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "Survey the providers in the background."},
	{ID: "a/0", Kind: api.ItemAgent, Turn: 1, Text: "Started the survey; I will read its report when it lands."},
	{ID: "u2", Kind: api.ItemUser, Turn: 2, Text: subagentNote},
	{ID: "u3", Kind: api.ItemUser, Turn: 3, Text: preamble + commandNote},
}

// A note is its summary on a row, folded: never its XML, and its result only
// once the row is clicked (the result's Markdown draws no label, so the fold
// says it opened).
func TestATaskNoteIsDrawnFoldedAsItsSummary(t *testing.T) {
	u, r := envelopeRig(t, claudeNotes)
	if !r.showsAll(`Agent "Survey 8 providers for advisor" finished`, `Background command "go test ./internal/chat/..." failed with exit code 1`) {
		t.Fatalf("the notes are not drawn as rows:\n%s", r.drawn())
	}
	for _, raw := range []string{"task-notification", "tool-use-id", "SYSTEM NOTIFICATION", "Short version"} {
		if strings.Contains(r.drawn(), raw) {
			t.Errorf("%q drawn while the notes are folded:\n%s", raw, r.drawn())
		}
	}
	writeShot(t, r, "tasknotes-folded.png")
	r.clickLabel(t, `Agent "Survey 8 providers for advisor" finished`)
	r.settle(t)
	if !u.chats.pane("s1").fold("u2#0").open {
		t.Error("a click did not unfold the result")
	}
	writeShot(t, r, "tasknotes-open.png")
}

func TestTaskNotesStayOutOfRecallAndTurns(t *testing.T) {
	if sent := sentHistory(claudeNotes); !slices.Equal(sent, []string{"Survey the providers in the background."}) {
		t.Errorf("sentHistory = %q, want only the person's message", sent)
	}
	prev, next := chatRow{kind: chatRowItem, item: claudeNotes[2]}, chatRow{kind: chatRowItem, item: claudeNotes[3]}
	if startsTurn(next, prev) {
		t.Error("a note under another started a turn")
	}
}
