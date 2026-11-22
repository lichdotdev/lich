package chat

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"sync"

	"github.com/lichdotdev/lich/api"
)

// Claude Code keeps its task list with TaskCreate and TaskUpdate, or with
// TodoWrite where tasks are off (CLAUDE_CODE_ENABLE_TASKS=0), and offers
// either only to the models that have it: on 2.1.330 the Claude 5 models get
// neither unless CLAUDE_CODE_ENABLE_TODO_TOOLS=1 (measured), which every spawn
// is given (internal/terminal/claude_todo.go). claudeTasks reads
// the list off the calls the way Claude Code's own SDK does.
const (
	claudeTodoTool   = "TodoWrite"
	claudeTaskCreate = "TaskCreate"
	claudeTaskUpdate = "TaskUpdate"
	claudeTaskGone   = "deleted"
)

// claudeTaskCreated is TaskCreate's result, which names the id it gave the
// task (Claude Code's own pattern).
var claudeTaskCreated = regexp.MustCompile(`^Task #(\S+) created successfully`)

type claudeTasks struct {
	mu sync.Mutex
	// ids orders tasks; pending holds a TaskCreate until its result names it.
	ids     []string
	tasks   map[string]api.ChatPlanEntry
	pending map[string]api.ChatPlanEntry
	created []string
}

func newClaudeTasks() *claudeTasks {
	return &claudeTasks{tasks: map[string]api.ChatPlanEntry{}, pending: map[string]api.ChatPlanEntry{}}
}

// use reads a tool call, true when it changed the list.
func (t *claudeTasks) use(b claudeBlock) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch b.Name {
	case claudeTodoTool:
		var in struct {
			Todos []api.ChatPlanEntry `json:"todos"`
		}
		if json.Unmarshal(b.Input, &in) != nil {
			return false
		}
		t.ids, t.tasks, t.pending, t.created = nil, map[string]api.ChatPlanEntry{}, map[string]api.ChatPlanEntry{}, nil
		for i, e := range in.Todos {
			id := strconv.Itoa(i)
			t.ids = append(t.ids, id)
			t.tasks[id] = e
		}
		return true
	case claudeTaskCreate:
		var in struct {
			Subject    string `json:"subject"`
			ActiveForm string `json:"activeForm"`
		}
		if json.Unmarshal(b.Input, &in) != nil || in.Subject == "" {
			return false
		}
		t.pending[b.ID] = api.ChatPlanEntry{Content: in.Subject, ActiveForm: in.ActiveForm, Status: api.PlanPending}
		t.created = append(t.created, b.ID)
		return true
	case claudeTaskUpdate:
		return t.update(b.Input)
	}
	return false
}

func (t *claudeTasks) update(raw json.RawMessage) bool {
	var in struct {
		ID         string `json:"taskId"`
		Status     string `json:"status"`
		Subject    string `json:"subject"`
		ActiveForm string `json:"activeForm"`
	}
	if json.Unmarshal(raw, &in) != nil || in.ID == "" {
		return false
	}
	e, known := t.tasks[in.ID]
	if in.Status == claudeTaskGone {
		if !known {
			return false
		}
		delete(t.tasks, in.ID)
		t.ids = slices.DeleteFunc(t.ids, func(id string) bool { return id == in.ID })
		return true
	}
	if !known {
		e = api.ChatPlanEntry{Content: in.ID, Status: api.PlanPending}
		t.ids = append(t.ids, in.ID)
	}
	if in.Subject != "" {
		e.Content = in.Subject
	}
	if in.ActiveForm != "" {
		e.ActiveForm = in.ActiveForm
	}
	if in.Status != "" {
		e.Status = in.Status
	}
	t.tasks[in.ID] = e
	return true
}

// result reads a tool result: a TaskCreate's names the task it made, or
// drops it when the call failed. True when it changed the list.
func (t *claudeTasks) result(b claudeBlock) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.pending[b.ToolUseID]
	if !ok {
		return false
	}
	delete(t.pending, b.ToolUseID)
	t.created = slices.DeleteFunc(t.created, func(id string) bool { return id == b.ToolUseID })
	if b.IsError {
		return true
	}
	m := claudeTaskCreated.FindStringSubmatch(blockText(b.Content))
	if m == nil {
		return true
	}
	if _, dup := t.tasks[m[1]]; !dup {
		t.ids = append(t.ids, m[1])
		t.tasks[m[1]] = e
	}
	return true
}

// list is the tasks in order, then the ones still being created.
func (t *claudeTasks) list() []api.ChatPlanEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]api.ChatPlanEntry, 0, len(t.ids)+len(t.created))
	for _, id := range t.ids {
		out = append(out, t.tasks[id])
	}
	for _, id := range t.created {
		out = append(out, t.pending[id])
	}
	return out
}
