package chat

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// A Claude Code chat's workflows, as measured on 2.1.330: the Workflow tool
// is a task of type local_workflow, reported by system/task_started with its
// workflow_name, then task_progress lines whose workflow_progress lists the
// script's phases and its agents (each whole, by index: label, phase, model,
// state start/done/error, times, tokens, the last tool call, the start of its
// prompt and of its result), and task_updated and task_notification with its
// end. A line may list only some of the agents.

const claudeWorkflowTask = "local_workflow"

// claudeWorkflows is the chat's workflows in the order they started.
type claudeWorkflows struct {
	mu   sync.Mutex
	list []api.ChatWorkflow
}

type claudeWorkflowLine struct {
	TaskID       string `json:"task_id"`
	TaskType     string `json:"task_type"`
	WorkflowName string `json:"workflow_name"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Patch        struct {
		Status  string `json:"status"`
		EndTime int64  `json:"end_time"`
	} `json:"patch"`
	Progress []claudeWorkflowEntry `json:"workflow_progress"`
}

type claudeWorkflowEntry struct {
	Type            string `json:"type"`
	Index           int    `json:"index"`
	Label           string `json:"label"`
	PhaseTitle      string `json:"phaseTitle"`
	Model           string `json:"model"`
	State           string `json:"state"`
	StartedAt       int64  `json:"startedAt"`
	LastProgressAt  int64  `json:"lastProgressAt"`
	Tokens          int    `json:"tokens"`
	ToolCalls       int    `json:"toolCalls"`
	LastToolName    string `json:"lastToolName"`
	LastToolSummary string `json:"lastToolSummary"`
	PromptPreview   string `json:"promptPreview"`
	ResultPreview   string `json:"resultPreview"`
	Error           string `json:"error"`
}

// task reads a system/task_* line, and reports whether it moved a workflow.
func (w *claudeWorkflows) task(subtype string, raw []byte, now time.Time) bool {
	var t claudeWorkflowLine
	if json.Unmarshal(raw, &t) != nil || t.TaskID == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if subtype == "task_started" {
		if t.TaskType != claudeWorkflowTask {
			return false
		}
		name := t.WorkflowName
		if name == "" {
			name = t.Description
		}
		w.list = append(w.list, api.ChatWorkflow{ID: t.TaskID, Name: name, State: api.WorkflowRunning, Started: now.UnixMilli(), Agents: []api.ChatWorkflowAgent{}})
		return true
	}
	i := slices.IndexFunc(w.list, func(wf api.ChatWorkflow) bool { return wf.ID == t.TaskID })
	if i < 0 {
		return false
	}
	wf := &w.list[i]
	for _, e := range t.Progress {
		if e.Type == "workflow_agent" {
			wf.Agents = mergeAgent(wf.Agents, e)
		}
	}
	status := t.Status
	if subtype == "task_updated" {
		status = t.Patch.Status
	}
	if state := claudeWorkflowState(status); state != "" && wf.State == api.WorkflowRunning {
		wf.State, wf.Ended = state, t.Patch.EndTime
		if wf.Ended == 0 {
			wf.Ended = now.UnixMilli()
		}
	}
	return true
}

// mergeAgent puts e in place of the agent of its index, or after the others;
// it keeps the start a line that leaves it out already gave.
func mergeAgent(agents []api.ChatWorkflowAgent, e claudeWorkflowEntry) []api.ChatWorkflowAgent {
	a := api.ChatWorkflowAgent{
		Index: e.Index, Label: e.Label, Phase: e.PhaseTitle, Model: e.Model, State: claudeAgentState(e),
		Started: e.StartedAt, Tokens: e.Tokens, ToolCalls: e.ToolCalls,
		Tool: e.LastToolName, ToolInput: e.LastToolSummary, Prompt: e.PromptPreview, Result: e.ResultPreview, Error: e.Error,
	}
	if a.State == api.WorkflowCompleted || a.State == api.WorkflowFailed {
		a.Ended = e.LastProgressAt
	}
	i := slices.IndexFunc(agents, func(x api.ChatWorkflowAgent) bool { return x.Index == e.Index })
	if i < 0 {
		return append(agents, a)
	}
	if a.Started == 0 {
		a.Started = agents[i].Started
	}
	agents[i] = a
	return agents
}

// claudeAgentState is an agent's state a workflow_agent entry names: start
// is queued until it has a start time.
func claudeAgentState(e claudeWorkflowEntry) string {
	switch e.State {
	case "done":
		return api.WorkflowCompleted
	case "error":
		return api.WorkflowFailed
	}
	if e.StartedAt == 0 {
		return api.WorkflowQueued
	}
	return api.WorkflowRunning
}

// claudeWorkflowState is the workflow's state a task status names, "" for
// one that is no end.
func claudeWorkflowState(status string) string {
	switch status {
	case "completed":
		return api.WorkflowCompleted
	case "failed":
		return api.WorkflowFailed
	case "killed", "stopped":
		return api.WorkflowStopped
	}
	return ""
}

// shown is the workflows, each a copy the reader cannot reach into.
func (w *claudeWorkflows) shown() []api.ChatWorkflow {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]api.ChatWorkflow, len(w.list))
	for i, wf := range w.list {
		wf.Agents = slices.Clone(wf.Agents)
		out[i] = wf
	}
	return out
}

func (d *claudeDriver) workflowTask(subtype string, raw []byte) {
	if !d.workflows.task(subtype, raw, time.Now()) {
		return
	}
	shown := d.workflows.shown()
	for _, wf := range shown {
		for i := range wf.Agents {
			wf.Agents[i].Model = d.modelName(wf.Agents[i].Model)
		}
	}
	d.c.setWorkflows(shown)
}

// Workflows answers api.ChatWorkflows: none for a provider that has no
// workflows to report (docs/ceilings.md).
func (s *Service) Workflows(ctx context.Context, opts api.SessionIDOptions) ([]api.ChatWorkflow, error) {
	c, err := s.ensure(ctx, opts.ID)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.workflows), nil
}

// setWorkflows records the chat's workflows and tells the window when they
// changed.
func (c *chatSession) setWorkflows(workflows []api.ChatWorkflow) {
	c.mu.Lock()
	same := reflect.DeepEqual(c.workflows, workflows)
	c.workflows = workflows
	c.mu.Unlock()
	if !same {
		events.Send(c.svc.hub.Emit, api.ChatWorkflowsEvent, api.ChatWorkflowsUpdate{SessionID: c.id, Workflows: workflows})
	}
}
