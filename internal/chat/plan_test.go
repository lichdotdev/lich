//go:build !windows

package chat

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// plan is the latest plan item of the conversation, waiting for one.
func (r *rig) plan(want int) []api.ChatPlanEntry {
	r.t.Helper()
	var got []api.ChatPlanEntry
	r.eventually("a plan item", func() bool {
		for _, it := range slices.Backward(r.history()) {
			if it.Kind == api.ItemPlan && it.Plan != nil {
				got = it.Plan.Entries
				return len(got) == want
			}
		}
		return false
	})
	return got
}

func TestClaudeTasksBecomeThePlan(t *testing.T) {
	r := newChat(t, providers.Claude)
	r.prompt("tasks")
	r.turnEnd(0)
	got := r.plan(2)
	want := []api.ChatPlanEntry{
		{Content: "Read the parser", ActiveForm: "Reading the parser", Status: api.PlanCompleted},
		{Content: "Fix it", ActiveForm: "Fixing it", Status: api.PlanInProgress},
	}
	if !slices.Equal(got, want) {
		t.Errorf("plan = %+v, want %+v", got, want)
	}
	if _, ok := r.item(api.ItemTool, ""); !ok {
		t.Error("the task calls are no longer tool items")
	}
}

func TestClaudeTodoWriteBecomesThePlan(t *testing.T) {
	r := newChat(t, providers.Claude)
	r.prompt("todos")
	r.turnEnd(0)
	got := r.plan(2)
	if got[1] != (api.ChatPlanEntry{Content: "Fix it", ActiveForm: "Fixing it", Status: api.PlanInProgress}) {
		t.Errorf("plan = %+v", got)
	}
}

func TestCodexPlanUpdateBecomesThePlan(t *testing.T) {
	r := newCodexChat(t)
	r.prompt("plan")
	r.turnEnd(0)
	got := r.plan(3)
	want := []api.ChatPlanEntry{
		{Content: "Read the parser", Status: api.PlanCompleted},
		{Content: "Fix it", Status: api.PlanInProgress},
		{Content: "Test it", Status: api.PlanPending},
	}
	if !slices.Equal(got, want) {
		t.Errorf("plan = %+v, want %+v", got, want)
	}
}

func TestACPPlanBecomesThePlanAndAnEmptyOneClearsIt(t *testing.T) {
	r := newChat(t, providers.OMP)
	r.prompt("plan")
	r.turnEnd(0)
	if got := r.plan(2); got[1] != (api.ChatPlanEntry{Content: "Fix it", Status: api.PlanInProgress}) {
		t.Errorf("plan = %+v", got)
	}
	r.prompt("plan-cleared")
	r.turnEnd(1)
	r.plan(0)
}
