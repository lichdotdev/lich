package main

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/op/paint"
	"gioui.org/text"

	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/gioterm"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// The window reads back every envelope of the contract's fixture.
func TestParseEnvelopesReadsTheFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/hooks/fixtures/notification.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Envelope string }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	read := map[string][]envelope{}
	for _, c := range fixture.Cases {
		envs, ok := parseEnvelopes(c.Envelope)
		if !ok {
			t.Fatalf("%s: not read as an envelope", c.Name)
		}
		read[c.Name] = envs
	}
	task := read["task in a chat, answered by the last message"][0]
	if task.kind != "task" || task.from != "Relay fixes" || task.session != "a1b2" || task.ticket != "4b9bc94e" ||
		task.text["prompt"] != "Fix #412: the relay handshake races on reconnect. Add a test that reproduces it first." {
		t.Errorf("task = %+v", task)
	}
	cut := read["worker finished, result cut in a chat"][0]
	if cut.text["result"] != "The handshake raced on reconnect. What I found, file by file:" || cut.text["status"] != "answered" {
		t.Errorf("cut = %+v", cut)
	}
	escaped := read["escaping: attributes escaped, element text verbatim"][0]
	if escaped.from != `Fix "quotes" & <tags>` || escaped.text["result"] != "Run a && b, then check that </result> stays text." {
		t.Errorf("escaped = %+v", escaped)
	}
	two := read["two envelopes in one message, oldest first, one blank line apart"]
	if len(two) != 2 || two[0].kind != "worker-finished" || two[1].kind != "expired" || two[1].text["status"] != "expired" {
		t.Errorf("two = %+v", two)
	}
	notice := read["notice: scheduled prompt"][0]
	if notice.topic != "scheduled" || notice.text["prompt"] != "Rebase on main and run the gate." {
		t.Errorf("notice = %+v", notice)
	}
}

// A message that only looks like one is the person's.
func TestParseEnvelopesLeavesOtherMessagesAlone(t *testing.T) {
	for _, text := range []string{"", "fix the bug", "<lich-notification version=\"1\">", "say <lich-notification kind=\"x\">\n</lich-notification>"} {
		if _, ok := parseEnvelopes(text); ok {
			t.Errorf("%q read as an envelope", text)
		}
	}
}

// note is an envelope about worker s2, as lich writes one.
func note(kind, body string) string {
	return `<lich-notification version="1" kind="` + kind + `" from="Fix #412" session="s2" ticket="4b9bc94e">` + "\n" + body + "\n</lich-notification>"
}

// envelopeRig is the chat of s1 holding items, drawn on the theme's ground
// with the window's own faces and provider names, so a shot reads as the
// window does. s2 is a Claude Code worker.
func envelopeRig(t *testing.T, items []backend.ChatItem) (*window, *frameRig) {
	t.Helper()
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.providers = &providerBook{machine: providersMachine{list: []providerInfo{{id: "claude", name: "Claude Code"}}}}
	u.model.projects[0].Sessions = append(u.model.projects[0].Sessions, backend.Session{ID: "s2", Kind: "claude", Label: "Fix #412"})
	u.model.chat.history("s1", items)
	mono, err := gioterm.Faces(*flagFont)
	if err != nil {
		t.Fatal(err)
	}
	sans, err := fonts.Faces()
	if err != nil {
		t.Fatal(err)
	}
	r.theme.Shaper = text.NewShaper(text.WithCollection(append(append(gofont.Collection(), sans...), mono...)))
	r.theme.Sans, r.theme.Mono = fonts.Sans, mono[0].Font.Typeface
	draw := r.draw
	r.draw = func(gtx C, th *ui.Theme) {
		paint.Fill(gtx.Ops, th.Background)
		draw(gtx, th)
	}
	r.frame()
	return u, r
}

// asker is the conversation of the session that opened worker s2.
var asker = []backend.ChatItem{
	{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "Fix #412 in a subagent."},
	{ID: "a/0", Kind: api.ItemAgent, Turn: 1, Text: "It runs as its own session; I will pick up its report."},
	{ID: "u2", Kind: api.ItemUser, Turn: 2, Text: note("worker-finished",
		"<status>answered</status>\n<result>The handshake raced on reconnect; fixed in `internal/relay/conn.go`, test added.</result>") +
		"\n\n" + note("worker-waiting", "<status>waiting</status>\n<request>Permission to run: go test ./internal/relay/...</request>")},
	{ID: "u5", Kind: api.ItemUser, Turn: 2, Text: note("worker-failed", "<status>failed</status>\n<error>The worker's last turn ended in an API error.</error>") +
		"\n\n" + note("worker-stopped", "<status>stopped</status>\n<by>user</by>") + "\n\n" + note("expired", "<status>expired</status>")},
	{ID: "u3", Kind: api.ItemUser, Turn: 3, Text: `<lich-notification version="1" kind="notice" topic="checks-failed" pr="1210" branch="fix-412">` +
		"\n<summary>The checks failed on pull request #1210 at commit 3f2a9c1.</summary>\n</lich-notification>"},
}

// worker is the conversation of a worker its task opened (mockup section B).
var worker = []backend.ChatItem{
	{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: `<lich-notification version="1" kind="task" from="Relay fixes" session="s3" ticket="7c1d" answer="last-message">` +
		"\n<prompt>Fix #412: the relay handshake races on reconnect. Add a test that reproduces it first.</prompt>\n</lich-notification>"},
	{ID: "a/0", Kind: api.ItemAgent, Turn: 1, Text: "Reproduced it with a test that reconnects mid-handshake; working on the fix."},
}

func envelopeFrames(t *testing.T) (*window, *frameRig) {
	t.Helper()
	return envelopeRig(t, asker)
}

// An envelope is drawn as what it says, never as its XML: a worker's outcome as
// a row with its result under it, a notice as a muted line, a task as the
// person's box naming who sent it.
func TestAnEnvelopeIsDrawnAsItsRows(t *testing.T) {
	_, r := envelopeFrames(t)
	if !r.showsAll("Fix #412 finished", "answered", "Fix #412 is waiting", "Permission to run: go test ./internal/relay/...",
		"The checks failed on pull request #1210 at commit 3f2a9c1.",
		"Fix #412 failed", "Fix #412 was stopped", "by you", "Fix #412 did not answer in time") {
		t.Fatalf("the envelopes are not drawn as rows:\n%s", r.drawn())
	}
	if r.shows("lich-notification") {
		t.Errorf("an envelope was drawn as its XML:\n%s", r.drawn())
	}
	writeShot(t, r, "envelopes.png")
}

// A click on a worker's row opens its card.
func TestAClickOnAWorkersRowOpensItsCard(t *testing.T) {
	u, r := envelopeFrames(t)
	r.clickLabel(t, "Fix #412 finished")
	if sv, ok := u.model.activeSession(); !ok || sv.ID != "s2" {
		t.Errorf("active session = %q, want the worker s2", sv.ID)
	}
}

// A worker's chat opens on its task as the person's box, naming who sent it.
func TestATaskOpensTheWorkersChatAsTheSendersBox(t *testing.T) {
	_, r := envelopeRig(t, worker)
	if !r.showsAll("From Relay fixes", "Fix #412: the relay handshake races on reconnect. Add a test that reproduces it first.") {
		t.Fatalf("the task is not drawn as the sender's box:\n%s", r.drawn())
	}
	if r.shows("lich-notification") || r.shows("last-message") {
		t.Errorf("the task was drawn as its XML:\n%s", r.drawn())
	}
	writeShot(t, r, "envelope-task.png")
}

// Copying a notification copies its rows' words, and walking back through the
// messages sent offers a task's prompt and skips the other notes.
func TestCopyAndRecallReadTheRows(t *testing.T) {
	envs, _ := parseEnvelopes(asker[2].Text)
	if got, want := envelopesText(envs), "Fix #412 finished · answered\n\nThe handshake raced on reconnect; fixed in `internal/relay/conn.go`, test added.\n\n"+
		"Fix #412 is waiting\n\nPermission to run: go test ./internal/relay/..."; got != want {
		t.Errorf("envelopesText = %q, want %q", got, want)
	}
	sent := sentHistory(append(slices.Clone(worker), asker...))
	want := []string{"Fix #412 in a subagent.", "Fix #412: the relay handshake races on reconnect. Add a test that reproduces it first."}
	if !slices.Equal(sent, want) {
		t.Errorf("sentHistory = %q, want %q", sent, want)
	}
}

// Notifications right under each other read as one list: no turn's gap
// between them.
func TestNotificationsUnderEachOtherStartNoTurn(t *testing.T) {
	prev, next := chatRow{kind: chatRowItem, item: asker[2]}, chatRow{kind: chatRowItem, item: asker[3]}
	if startsTurn(next, prev) {
		t.Error("a notification under another started a turn")
	}
	if !startsTurn(prev, chatRow{kind: chatRowItem, item: asker[1]}) {
		t.Error("a notification under the agent's answer did not start a turn")
	}
}
