package chat

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// claudeWorkflowLines is the system/task_* lines a two-agent workflow wrote
// on Claude Code 2.1.330's stream (measured), with the ends of its subagents'
// own commands among them.
//
//go:embed testdata/claude_workflow.jsonl
var claudeWorkflowLines []byte

// workflowReplay feeds the first n lines of the measured workflow to a
// reader, every line when n is negative.
func workflowReplay(t *testing.T, n int) []api.ChatWorkflow {
	t.Helper()
	var w claudeWorkflows
	lines := bytes.Split(bytes.TrimSpace(claudeWorkflowLines), []byte("\n"))
	if n < 0 {
		n = len(lines)
	}
	now := time.UnixMilli(1795460277000)
	for _, line := range lines[:n] {
		var head struct {
			Subtype string `json:"subtype"`
		}
		if err := json.Unmarshal(line, &head); err != nil {
			t.Fatalf("fixture line: %v", err)
		}
		w.task(head.Subtype, line, now)
	}
	return w.shown()
}

func TestClaudeWorkflowStartsRunningWithNoAgents(t *testing.T) {
	got := workflowReplay(t, 1)
	if len(got) != 1 {
		t.Fatalf("workflows = %+v", got)
	}
	if wf := got[0]; wf.ID != "w11ggbos4" || wf.Name != "sleep-echo-probe" || wf.State != api.WorkflowRunning || wf.Started != 1795460277000 || len(wf.Agents) != 0 {
		t.Fatalf("workflow = %+v", wf)
	}
}

// The first progress line lists both agents: a started, b still queued.
func TestClaudeWorkflowAgentsAreQueuedUntilTheyStart(t *testing.T) {
	got := workflowReplay(t, 2)[0]
	if len(got.Agents) != 2 {
		t.Fatalf("agents = %+v", got.Agents)
	}
	if a := got.Agents[0]; a.Label != "a" || a.Phase != "Run" || a.Model != "claude-haiku-5-5" || a.State != api.WorkflowRunning || a.Started != 1795460277665 || a.Ended != 0 {
		t.Fatalf("agent a = %+v", a)
	}
	if b := got.Agents[1]; b.Label != "b" || b.State != api.WorkflowQueued || b.Started != 0 {
		t.Fatalf("agent b = %+v", b)
	}
}

func TestClaudeWorkflowEndsWithEveryAgentsResult(t *testing.T) {
	got := workflowReplay(t, -1)
	if len(got) != 1 {
		t.Fatalf("workflows = %+v; the subagents' shells are no workflow", got)
	}
	wf := got[0]
	if wf.State != api.WorkflowCompleted || wf.Ended != 1795460287756 {
		t.Fatalf("workflow = %+v, want completed at the patch's end_time", wf)
	}
	if len(wf.Agents) != 2 || wf.Agents[0].Label != "a" || wf.Agents[1].Label != "b" {
		t.Fatalf("agents = %+v", wf.Agents)
	}
	a := wf.Agents[0]
	want := api.ChatWorkflowAgent{
		Index: 1, Label: "a", Phase: "Run", Model: "claude-haiku-5-5", State: api.WorkflowCompleted,
		Started: 1795460277665, Ended: 1795460287732, Tokens: 17977, ToolCalls: 1,
		Tool: "Bash", ToolInput: "sleep 5; echo done",
		Prompt: "Run the shell command `sleep 5; echo done` and reply with only its output.", Result: "done",
	}
	if a != want {
		t.Errorf("agent a = %+v\nwant %+v", a, want)
	}
}

func TestClaudeAgentStates(t *testing.T) {
	for _, c := range []struct {
		e    claudeWorkflowEntry
		want string
	}{
		{claudeWorkflowEntry{State: "start"}, api.WorkflowQueued},
		{claudeWorkflowEntry{State: "start", StartedAt: 1}, api.WorkflowRunning},
		{claudeWorkflowEntry{State: "done", StartedAt: 1}, api.WorkflowCompleted},
		{claudeWorkflowEntry{State: "error", Error: "blocked"}, api.WorkflowFailed},
	} {
		if got := claudeAgentState(c.e); got != c.want {
			t.Errorf("claudeAgentState(%+v) = %q, want %q", c.e, got, c.want)
		}
	}
}

// A line that leaves the start out keeps the one an earlier line gave.
func TestClaudeWorkflowAgentKeepsItsStart(t *testing.T) {
	agents := mergeAgent(nil, claudeWorkflowEntry{Index: 1, Label: "a", State: "start", StartedAt: 5})
	agents = mergeAgent(agents, claudeWorkflowEntry{Index: 1, Label: "a", State: "error", Error: "boom", LastProgressAt: 9})
	if len(agents) != 1 || agents[0].Started != 5 || agents[0].Ended != 9 || agents[0].State != api.WorkflowFailed || agents[0].Error != "boom" {
		t.Fatalf("agents = %+v", agents)
	}
}

func TestClaudeWorkflowStoppedAndUnknownLines(t *testing.T) {
	var w claudeWorkflows
	now := time.UnixMilli(100)
	if w.task("task_progress", []byte(`{"task_id":"nope"}`), now) {
		t.Error("a line for no workflow moved one")
	}
	if w.task("task_started", []byte(`{"task_id":"b1","task_type":"local_bash"}`), now) {
		t.Error("a shell was read as a workflow")
	}
	w.task("task_started", []byte(`{"task_id":"w1","task_type":"local_workflow","description":"Probe"}`), now)
	w.task("task_notification", []byte(`{"task_id":"w1","status":"killed"}`), time.UnixMilli(200))
	got := w.shown()
	if len(got) != 1 || got[0].Name != "Probe" || got[0].State != api.WorkflowStopped || got[0].Ended != 200 {
		t.Fatalf("workflows = %+v", got)
	}
}
