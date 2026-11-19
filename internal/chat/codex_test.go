//go:build !windows

package chat

import (
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func newCodexChat(t *testing.T) *rig {
	t.Helper()
	r := newRig(t, providers.Codex)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	return r
}

// turnStarts is every turn/start lich sent, as the fake logged it.
func (r *rig) turnStarts() []string {
	var out []string
	for line := range strings.Lines(r.fakeLog()) {
		if strings.Contains(line, `"turn/start"`) {
			out = append(out, line)
		}
	}
	return out
}

// TestCodexTurnStreamsAndEnds: a chat runs app-server, its reasoning and
// answer stream into one item each named by the model and effort, the echo of
// the person's message is not repeated, and the thread is the session's
// conversation.
func TestCodexTurnStreamsAndEnds(t *testing.T) {
	r := newCodexChat(t)
	if log := r.fakeLog(); !strings.Contains(log, "developer_instructions=") || !strings.Contains(log, "app-server") ||
		strings.Contains(log, "--model") || !strings.Contains(log, `"initialized"`) {
		t.Errorf("fake log = %q, want the briefing, app-server initialized and no --model", log)
	}
	r.prompt("hi")
	if turn := r.turnEnd(0); turn.StopReason != stopEndTurn {
		t.Errorf("turn ended %q", turn.StopReason)
	}
	if it := r.mustItem(api.ItemThought, "hmm"); it.Text != "hmm" || it.Model != "GPT A" || it.Effort != "medium" {
		t.Errorf("thought = %+v", it)
	}
	if it := r.mustItem(api.ItemAgent, "Oi!"); it.Text != "Oi!" || it.Turn != 1 || it.Model != "GPT A" {
		t.Errorf("answer = %+v", it)
	}
	users := 0
	for _, it := range r.history() {
		if it.Kind == api.ItemUser {
			users++
		}
	}
	if users != 1 {
		t.Errorf("%d user items, want 1", users)
	}
	if conv, _ := r.db.ProviderSession("s1"); conv != "codex-new" {
		t.Errorf("bound conversation = %q", conv)
	}
}

// TestCodexApprovalsBecomePermissions: a command's approval shows the command,
// a patch's the diff item/started drew, and each answer is the decision Codex
// takes.
func TestCodexApprovalsBecomePermissions(t *testing.T) {
	r := newCodexChat(t)
	r.prompt("tool")
	perm := r.waitPermission()
	if tool := perm.Permission.Tool; tool.Name != codexShellTool || tool.Title != "ls" || len(perm.Permission.Options) != 3 {
		t.Errorf("command permission = %+v", perm.Permission)
	}
	if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1", ItemID: perm.ID, OptionID: codexAccept}); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	r.eventually("the patch's permission", func() bool { return r.waitPermission().ID != perm.ID })
	patch := r.waitPermission()
	if tool := patch.Permission.Tool; tool.Name != codexPatchTool || !strings.Contains(tool.Diff, "--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@") {
		t.Errorf("patch permission = %+v", tool)
	}
	if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1", ItemID: patch.ID, OptionID: codexDecline}); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	r.mustItem(api.ItemAgent, "decisions:accept,decline")
	r.turnEnd(0)
	for id, want := range map[string]string{"ex1": api.ToolDone, "fc1": api.ToolFailed} {
		if it, _ := r.svc.running("s1").conv.get(id); it.Tool == nil || it.Tool.State != want {
			t.Errorf("%s = %+v, want %s", id, it.Tool, want)
		}
	}
}

// TestCodexQuestionsAnsweredAndSteered: request_user_input is a question item
// answered by id, and a message written while it waits declines it and joins
// the turn running.
func TestCodexQuestionsAnsweredAndSteered(t *testing.T) {
	r := newCodexChat(t)
	r.prompt("question")
	q := r.waitingQuestion()
	if len(q.Question.Questions) != 2 || q.Question.Questions[0].Options[1].Label != "Pear" {
		t.Fatalf("question = %+v", q.Question)
	}
	err := r.reply(q.ID, api.ChatQReplied{Answers: []api.ChatAskAnswer{{Picked: []string{"Apple"}}, {Other: "ripe"}}})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	r.mustItem(api.ItemAgent, `answers:{"fruit":{"answers":["Apple"]},"notes":{"answers":["ripe"]}}`)
	r.turnEnd(0)

	r.prompt("question")
	r.waitingQuestion()
	r.prompt("pear please")
	r.mustItem(api.ItemAgent, "steered:pear please")
	if turn := r.turnEnd(1); turn.StopReason != stopEndTurn {
		t.Errorf("turn ended %q", turn.StopReason)
	}
	if !strings.Contains(r.fakeLog(), `"turn/steer"`) {
		t.Error("the message was not steered into the turn")
	}
}

// TestCodexInterruptAndFailure: a cancel interrupts the turn, and a failed
// turn says why.
func TestCodexInterruptAndFailure(t *testing.T) {
	r := newCodexChat(t)
	r.prompt("slow")
	r.mustItem(api.ItemAgent, "counting")
	if _, err := r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if turn := r.turnEnd(0); turn.StopReason != stopCancelled {
		t.Errorf("cancelled turn ended %q", turn.StopReason)
	}
	r.prompt("fail")
	if turn := r.turnEnd(1); turn.StopReason != stopError {
		t.Errorf("failed turn ended %q", turn.StopReason)
	}
	r.mustItem(api.ItemNotice, "The model is not supported")
	r.prompt("die")
	r.turnEnd(2)
	if !r.sent(api.ItemNotice, "codex boom") {
		t.Error("the process's death was not said with its stderr")
	}
}

// TestCodexResumeReadsHistory: a resumed thread's turns come back page by page
// without reaching the window, each answer named by its turn's model from the
// rollout, and a thread Codex cannot find opens a new one with a notice.
func TestCodexResumeReadsHistory(t *testing.T) {
	r := newRig(t, providers.Codex)
	_ = r.db.SetProviderSession("s1", "codex-conv")
	r.writeCodexRollout("codex-conv")
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if strings.Contains(r.fakeLog(), "developer_instructions") {
		t.Error("a resume was handed the spawn briefing")
	}
	if it := r.mustItem(api.ItemAgent, "ok"); it.Turn != 1 || it.Model != "GPT B" || it.Effort != "high" {
		t.Errorf("replayed answer = %+v", it)
	}
	if it := r.mustItem(api.ItemAgent, "second"); it.Turn != 2 || it.Model != "" {
		t.Errorf("second page answer = %+v", it)
	}
	r.mustItem(api.ItemThought, "thinking")
	if it, ok := r.svc.running("s1").conv.get("e1"); !ok || it.Tool.State != api.ToolDone || it.Tool.Output != "a.txt\n" {
		t.Errorf("replayed command = %+v", it.Tool)
	}
	for _, e := range r.snapshot() {
		if _, ok := e.data.(api.ChatItemUpdate); ok {
			t.Errorf("the replay reached the window: %+v", e.data)
		}
	}
	if conv, _ := r.db.ProviderSession("s1"); conv != "codex-conv" {
		t.Errorf("bound conversation = %q", conv)
	}
}

func TestCodexResumeOfALostThreadStartsOver(t *testing.T) {
	r := newRig(t, providers.Codex)
	_ = r.db.SetProviderSession("s1", "gone")
	r.writeCodexRollout("gone")
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.mustItem(api.ItemNotice, "no rollout found")
	if conv, _ := r.db.ProviderSession("s1"); conv != "codex-new" {
		t.Errorf("bound conversation = %q, want the new thread", conv)
	}
}

// TestCodexOptionsApplyOnTheNextTurn: the model, effort and mode come from
// model/list and the thread, a change rides the next turn/start alone, and a
// model that lacks the effort in force falls back to its default.
func TestCodexOptionsApplyOnTheNextTurn(t *testing.T) {
	r := newCodexChat(t)
	r.current(api.OptionMode, codexWorkspace)
	r.current(api.OptionEffort, "medium")
	models, _ := r.option(api.OptionModel)
	if ids := choiceIDs(models); !slices.Equal(ids, []string{"gpt-a", "gpt-b"}) {
		t.Errorf("models = %v, want the hidden one left out", ids)
	}
	mode, _ := r.option(api.OptionMode)
	if !slices.Equal(choiceIDs(mode), codexModes) || !mode.Choices[2].Danger {
		t.Errorf("modes = %+v", mode.Choices)
	}

	r.mustSet(codexModelOption, "gpt-b")
	r.current(api.OptionEffort, "high")
	r.mustSet(codexModeOption, codexFullAccess)
	r.prompt("hi")
	r.turnEnd(0)
	r.prompt("hi")
	r.turnEnd(1)
	starts := r.turnStarts()
	if len(starts) != 2 {
		t.Fatalf("turn/starts = %q", starts)
	}
	for _, want := range []string{`"model":"gpt-b"`, `"effort":"high"`, `"approvalPolicy":"never"`, `"type":"dangerFullAccess"`} {
		if !strings.Contains(starts[0], want) {
			t.Errorf("first turn/start %s lacks %s", starts[0], want)
		}
	}
	if strings.Contains(starts[1], `"model"`) || strings.Contains(starts[1], "approvalPolicy") {
		t.Errorf("the second turn/start carried the overrides again: %s", starts[1])
	}
	r.current(api.OptionMode, codexFullAccess)
	if it := r.mustItem(api.ItemAgent, "Oi!"); it.Model != "GPT B" || it.Effort != "high" {
		t.Errorf("answer after the switch = %+v", it)
	}
}

// TestCodexSkipPermissionsOpensOnFullAccess: a checkout that skips
// permissions opens the thread never asking, outside the sandbox.
func TestCodexSkipPermissionsOpensOnFullAccess(t *testing.T) {
	r := newRig(t, providers.Codex)
	_, _ = r.db.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.codex.skip-permissions", Value: "true"})
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.current(api.OptionMode, codexFullAccess)
	if log := r.fakeLog(); !strings.Contains(log, `"sandbox":"danger-full-access"`) || strings.Contains(log, "--dangerously") {
		t.Errorf("fake log = %q", log)
	}
}

// TestCodexServerRequests: a permission grant echoes what was asked, a form
// elicitation is a question, a request Codex resolved itself stops asking,
// and one lich does not serve is refused.
func TestCodexServerRequests(t *testing.T) {
	r := newCodexChat(t)
	r.prompt("perm")
	perm := r.waitPermission()
	if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1", ItemID: perm.ID, OptionID: codexScopeSession}); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	r.mustItem(api.ItemAgent, `perm:{"permissions":{"fileSystem":null,"network":{"enabled":true}},"scope":"session"}`)
	r.turnEnd(0)

	r.prompt("elicit")
	q := r.waitingQuestion()
	if err := r.reply(q.ID, api.ChatQReplied{Declined: true}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	r.mustItem(api.ItemAgent, `elicit:{"_meta":null,"action":"decline","content":null}`)
	r.turnEnd(1)

	r.prompt("resolved")
	r.turnEnd(2)
	r.eventually("the resolved permission to close", func() bool {
		it, _ := r.svc.running("s1").conv.get("perm-4")
		return it.Permission != nil && it.Permission.Tool.State == api.ToolFailed
	})
	if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1", ItemID: "perm-4", OptionID: codexAccept}); err == nil {
		t.Error("a resolved permission still took an answer")
	}

	r.prompt("dynamic")
	r.mustItem(api.ItemAgent, "-32601")
}
