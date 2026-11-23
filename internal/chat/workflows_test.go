//go:build !windows

package chat

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// workflows is the last workflows event the window was sent, waiting for one
// that cond holds.
func (r *rig) workflows(what string, cond func([]api.ChatWorkflow) bool) []api.ChatWorkflow {
	r.t.Helper()
	var got []api.ChatWorkflow
	r.eventually(what, func() bool {
		for _, e := range slices.Backward(r.snapshot()) {
			if u, ok := e.data.(api.ChatWorkflowsUpdate); ok && e.name == api.ChatWorkflowsEvent.Name {
				got = u.Workflows
				return cond(got)
			}
		}
		return false
	})
	return got
}

func TestClaudeWorkflowReachesTheWindowAndGoesWithTheProcess(t *testing.T) {
	r := newChat(t, providers.Claude)
	r.prompt("workflow")
	r.turnEnd(0)
	got := r.workflows("the workflow ended", func(w []api.ChatWorkflow) bool {
		return len(w) == 1 && w[0].State == api.WorkflowCompleted
	})
	if len(got[0].Agents) != 2 {
		t.Fatalf("agents = %+v", got[0].Agents)
	}
	listed, err := r.svc.Workflows(t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil || len(listed) != 1 || listed[0].ID != "w11ggbos4" {
		t.Fatalf("Workflows = %+v, %v", listed, err)
	}
	if err := r.setView(api.ViewTerminal); err != nil {
		t.Fatalf("to terminal: %v", err)
	}
	r.workflows("the workflows gone with the process", func(w []api.ChatWorkflow) bool { return len(w) == 0 })
}
