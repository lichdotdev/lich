package main

import (
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// workflowFrames is session s1 drawn as a Claude Code chat whose turn ended
// with a workflow of five agents running on: three done, two running. The
// running names shimmer and the clock counts, so the screen never goes still.
func workflowFrames(t *testing.T) (*window, *frameRig, *chatPane) {
	t.Helper()
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go", At: r.now.Add(-5 * time.Minute).UnixMilli()}})
	start := r.now.Add(-252 * time.Second).UnixMilli()
	agent := func(i int, label, state string, ended int64) api.ChatWorkflowAgent {
		return api.ChatWorkflowAgent{Index: i, Label: label, Phase: "Measure", Model: "Sonnet 5.5", State: state, Started: start, Ended: ended}
	}
	opencode := agent(4, "title:opencode", api.WorkflowRunning, 0)
	opencode.ToolCalls, opencode.Tokens, opencode.Tool, opencode.ToolInput = 23, 41200, "Bash", "timeout 40 opencode run"
	opencode.Prompt = "Measure whether lich can name a chat session"
	u.model.chat.setWorkflows("s1", []backend.ChatWorkflow{{
		ID: "w1", Name: "title-probe", State: api.WorkflowRunning, Started: start,
		Agents: []api.ChatWorkflowAgent{
			agent(1, "title:antigravity", api.WorkflowCompleted, start+108_000),
			agent(2, "title:oh-my-pi", api.WorkflowCompleted, start+62_000),
			agent(3, "title:kiro", api.WorkflowCompleted, start+131_000),
			opencode,
			agent(5, "title:cursor", api.WorkflowRunning, 0),
		},
	}})
	r.frames(2)
	return u, r, u.chats.pane("s1")
}

// The line says the workflow runs after the turn ended; a click lists its
// agents under their phase, and an agent opens on a click with its call.
func TestWorkflowLineOpensItsAgents(t *testing.T) {
	_, r, p := workflowFrames(t)
	writeShot(t, r, "workflow-line.png")
	if !r.showsAll("title-probe 3/5", "title:opencode, title:cursor") || !strings.Contains(r.drawn(), `"4m 1`) {
		t.Fatalf("the workflow's line is not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "title-probe 3/5"))
	writeShot(t, r, "workflow-panel.png")
	// Group labels are uppercase since the composer panels share the palette's
	// group heading (contract changed).
	if !r.showsAll("MEASURE", "title:antigravity", "title:kiro", "1m 48s") {
		t.Fatalf("the agents are not listed:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "title:opencode"))
	toolsSettle(r)
	writeShot(t, r, "workflow-agent.png")
	// a fact's value draws no node of its own: its label stands for it
	if !r.showsAll("Phase", "Model", "Time", "Now", "Task") || r.shows("Result") {
		t.Fatalf("the agent's detail is not drawn:\n%s", r.drawn())
	}
	r.press(key.NameEscape)
	if !r.shows("title:kiro") {
		t.Errorf("Escape did not go back to the agents:\n%s", r.drawn())
	}
	r.press(key.NameEscape)
	if p.tools.opened() {
		t.Errorf("a second Escape left the panel open:\n%s", r.drawn())
	}
}

func TestWorkflowLineGoesWhenItEnds(t *testing.T) {
	u, r, _ := workflowFrames(t)
	wf := u.model.chat.workflowsOf("s1")[0]
	wf.State, wf.Ended = api.WorkflowCompleted, r.now.UnixMilli()
	u.model.chat.setWorkflows("s1", []backend.ChatWorkflow{wf})
	r.frames(2)
	if r.shows("title-probe") {
		t.Errorf("a finished workflow still has its line:\n%s", r.drawn())
	}
}

// A failure stays in sight until the person writes again.
func TestFailedWorkflowStaysUntilThePersonWrites(t *testing.T) {
	u, r, _ := workflowFrames(t)
	wf := u.model.chat.workflowsOf("s1")[0]
	wf.State, wf.Ended = api.WorkflowFailed, r.now.UnixMilli()
	u.model.chat.setWorkflows("s1", []backend.ChatWorkflow{wf})
	r.frames(2)
	// the failed glyph says it; the word left the line (contract changed)
	if !r.showsAll("title-probe 3/5") || r.shows("failed") {
		t.Fatalf("the failed workflow is not drawn:\n%s", r.drawn())
	}
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go", At: r.now.Add(-5 * time.Minute).UnixMilli()},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, Text: "and now", At: r.now.Add(time.Second).UnixMilli()},
	})
	r.frames(2)
	if r.shows("title-probe") {
		t.Errorf("the failure outlived the next message:\n%s", r.drawn())
	}
}

func TestWorkflowTipNamesThePhaseAndStart(t *testing.T) {
	now := time.Date(2026, 11, 23, 15, 40, 0, 0, time.Local)
	wf := backend.ChatWorkflow{Started: time.Date(2026, 11, 23, 15, 33, 0, 0, time.Local).UnixMilli(), Agents: []api.ChatWorkflowAgent{
		{Index: 1, Phase: "Measure", State: api.WorkflowRunning},
		{Index: 2, Phase: "Report", State: api.WorkflowQueued},
	}}
	if got, want := workflowTip(wf, now), "Measure · 2 agents · started 15:33"; got != want {
		t.Errorf("workflowTip = %q, want %q", got, want)
	}
	if !strings.Contains(workflowTip(backend.ChatWorkflow{Started: now.UnixMilli()}, now), "0 agents") {
		t.Error("a workflow with no agent yet named a phase")
	}
}
