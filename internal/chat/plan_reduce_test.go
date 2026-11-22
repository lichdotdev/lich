package chat

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestCodexPlanStatesMapOntoThePlans(t *testing.T) {
	got := codexPlan([]byte(`{"plan":[{"step":"a","status":"inProgress"},{"step":"b","status":"weird"}]}`))
	if got[0].Status != api.PlanInProgress || got[1].Status != api.PlanPending {
		t.Errorf("plan = %+v", got)
	}
}

func TestClaudeTasksUpdateAndDelete(t *testing.T) {
	ts := newClaudeTasks()
	ts.use(claudeBlock{Name: claudeTaskUpdate, Input: []byte(`{"taskId":"7","status":"in_progress","subject":"Ship"}`)})
	ts.use(claudeBlock{Name: claudeTaskUpdate, Input: []byte(`{"taskId":"8"}`)})
	if got := ts.list(); len(got) != 2 || got[0] != (api.ChatPlanEntry{Content: "Ship", Status: api.PlanInProgress}) || got[1].Content != "8" {
		t.Errorf("list = %+v", got)
	}
	if !ts.use(claudeBlock{Name: claudeTaskUpdate, Input: []byte(`{"taskId":"7","status":"deleted"}`)}) || len(ts.list()) != 1 {
		t.Errorf("a deleted task stayed: %+v", ts.list())
	}
	if ts.use(claudeBlock{Name: claudeTaskUpdate, Input: []byte(`{"taskId":"9","status":"deleted"}`)}) {
		t.Error("deleting an unknown task changed the list")
	}
	if ts.use(claudeBlock{Name: "Read", Input: []byte(`{}`)}) || ts.result(claudeBlock{ToolUseID: "x"}) {
		t.Error("another tool changed the list")
	}
}
