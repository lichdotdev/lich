package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// A workflow the agent left running (api.ChatWorkflows) stands over the
// composer as one line, where the task list stands: its name, how many of its
// agents are done and the ones running, which outlive the turn that started
// them and would otherwise run out of sight. A click opens a panel in the
// slash list's place listing its agents by phase, and an agent's detail.

// chatWorkflowsPanel is the panel's kind in chatTools.
const chatWorkflowsPanel = "workflows"

var icWorkflow = icons.Lucide("workflow")

// chatWorkflowState is the line's click and hover.
type chatWorkflowState struct {
	toggle ui.Clickable
	tip    ui.Tooltip
}

// workflowShown is the workflow the line stands for: the newest that runs,
// else the newest that failed since the person last wrote, so a failure stays
// in sight until they answer it.
//
// shortcut: one workflow at a time; two running at once show the newer, list
// them all if the agent ever runs several.
func workflowShown(workflows []backend.ChatWorkflow, items []backend.ChatItem) (backend.ChatWorkflow, bool) {
	for _, wf := range slices.Backward(workflows) {
		if wf.State == api.WorkflowRunning {
			return wf, true
		}
	}
	wrote := int64(0)
	for _, it := range items {
		if it.Kind == api.ItemUser {
			wrote = max(wrote, it.At)
		}
	}
	for _, wf := range slices.Backward(workflows) {
		if wf.State == api.WorkflowFailed && wf.Ended >= wrote {
			return wf, true
		}
	}
	return backend.ChatWorkflow{}, false
}

// workflowDone counts the agents that ended, done or failed.
func workflowDone(wf backend.ChatWorkflow) int {
	n := 0
	for _, a := range wf.Agents {
		if agentEnded(a) {
			n++
		}
	}
	return n
}

func agentEnded(a api.ChatWorkflowAgent) bool {
	return a.State == api.WorkflowCompleted || a.State == api.WorkflowFailed
}

// workflowRunning is the labels of the agents that run.
func workflowRunning(wf backend.ChatWorkflow) string {
	var labels []string
	for _, a := range wf.Agents {
		if a.State == api.WorkflowRunning {
			labels = append(labels, a.Label)
		}
	}
	return strings.Join(labels, ", ")
}

// workflowPhase is the phase the newest agent to start runs in.
func workflowPhase(wf backend.ChatWorkflow) string {
	for _, a := range slices.Backward(wf.Agents) {
		if a.State != api.WorkflowQueued && a.Phase != "" {
			return a.Phase
		}
	}
	return ""
}

// took is how long something started at started has run, or ran when ended
// is set, ticking while it runs; "" with no start.
func took(gtx C, started, ended int64) string {
	if started == 0 {
		return ""
	}
	start := time.UnixMilli(started)
	if ended == 0 {
		chatTick(gtx, start)
		return chatElapsed(gtx.Now.Sub(start))
	}
	return chatElapsed(time.UnixMilli(ended).Sub(start))
}

// toggleWorkflows opens the workflow's panel, or shuts it when it is open.
func (p *chatPane) toggleWorkflows() {
	t := &p.tools
	if t.kind == chatWorkflowsPanel {
		t.shut()
		return
	}
	t.open(chatWorkflowsPanel)
	t.mu.Lock()
	t.loaded, t.base = true, p.editor.Len()
	t.mu.Unlock()
}

// workflowActs runs the panel: a row opens its agent, Back goes back.
func (u *window) workflowActs(gtx C, p *chatPane, id string) (int, func(int)) {
	t := &p.tools
	wf, ok := workflowShown(u.model.chat.workflowsOf(id), u.model.chat.of(id))
	if !ok {
		t.shut()
		return 0, func(int) {}
	}
	if t.detail == "" {
		return len(wf.Agents), func(i int) { t.detail = agentKey(wf.Agents[i]) }
	}
	if t.back.Clicked(gtx) || !slices.ContainsFunc(wf.Agents, func(a api.ChatWorkflowAgent) bool { return agentKey(a) == t.detail }) {
		t.detail = ""
	}
	return 0, func(int) {}
}

// agentKey names an agent in the panel's detail, by its index in the script.
func agentKey(a api.ChatWorkflowAgent) string { return strconv.Itoa(a.Index) }
