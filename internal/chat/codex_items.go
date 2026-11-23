package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// codexItem is a ThreadItem, every field lich reads off any of its types.
type codexItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	StartedAt int64  `json:"startedAtMs"`
	// agentMessage, plan
	Text string `json:"text"`
	// userMessage
	Content json.RawMessage `json:"content"`
	// reasoning
	Summary []string `json:"summary"`
	// commandExecution
	Command  string  `json:"command"`
	Output   *string `json:"aggregatedOutput"`
	ExitCode *int    `json:"exitCode"`
	// fileChange
	Changes []codexChange `json:"changes"`
	// mcpToolCall, dynamicToolCall
	Server string          `json:"server"`
	Tool   string          `json:"tool"`
	Args   json.RawMessage `json:"arguments"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
	// webSearch
	Query string `json:"query"`
	// imageView
	Path   string `json:"path"`
	Status string `json:"status"`
}

type codexChange struct {
	Path string `json:"path"`
	Kind struct {
		Type string `json:"type"`
	} `json:"kind"`
	Diff string `json:"diff"`
}

// The names a Codex tool item is drawn under: the ones Codex's own hooks
// report for a shell command and a patch, and its tools' own for the rest.
const (
	codexShellTool  = "Bash"
	codexPatchTool  = "apply_patch"
	codexSearchTool = "web_search"
	codexImageTool  = "view_image"
)

// codexStates maps a tool item's status onto the chat's: a declined command
// or patch did not run.
var codexStates = map[string]string{
	"inProgress": api.ToolRunning,
	"completed":  api.ToolDone,
	"failed":     api.ToolFailed,
	"declined":   api.ToolFailed,
}

// chatItem is the chat's reading of it, false for a kind the chat does not
// draw (a plan, a compaction, a review marker) and for the person's message.
func (it codexItem) chatItem(done bool) (api.ChatItem, bool) {
	ci := api.ChatItem{ID: it.ID, At: it.StartedAt}
	switch it.Type {
	case "agentMessage":
		ci.Kind, ci.Text = api.ItemAgent, it.Text
	case "reasoning":
		ci.Kind, ci.Text = api.ItemThought, strings.Join(it.Summary, "\n\n")
	default:
		tool, ok := it.tool()
		if !ok {
			return ci, false
		}
		tool.State = api.ToolRunning
		if done {
			tool.State = api.ToolDone
		}
		if state, ok := codexStates[it.Status]; ok {
			tool.State = state
		}
		ci.Kind, ci.Tool = api.ItemTool, &tool
	}
	return ci, ci.ID != ""
}

// tool reads a tool item.
func (it codexItem) tool() (api.ChatToolCall, bool) {
	switch it.Type {
	case "commandExecution":
		t := api.ChatToolCall{Name: codexShellTool, Title: it.Command}
		if it.Output != nil {
			t.Output = shorten(*it.Output)
		}
		return t, true
	case "fileChange":
		return codexPatch(it.Changes), true
	case "mcpToolCall":
		t := api.ChatToolCall{Name: "mcp__" + it.Server + "__" + it.Tool, Title: argsTitle(it.Args),
			Output: shorten(mcpResultText(it.Result))}
		if it.Error != nil {
			t.Output = shorten(it.Error.Message)
		}
		return t, true
	case "dynamicToolCall":
		return api.ChatToolCall{Name: it.Tool, Title: argsTitle(it.Args)}, true
	case "webSearch":
		return api.ChatToolCall{Name: codexSearchTool, Title: it.Query}, true
	case "imageView":
		return api.ChatToolCall{Name: codexImageTool, Title: it.Path}, true
	}
	return api.ChatToolCall{}, false
}

// codexPatch reads a patch's changes: an update's diff is its hunks with no
// file header, an added or deleted file's the whole content (measured).
func codexPatch(changes []codexChange) api.ChatToolCall {
	t := api.ChatToolCall{Name: codexPatchTool}
	var diffs, paths []string
	for _, ch := range changes {
		paths = append(paths, ch.Path)
		switch ch.Kind.Type {
		case "add":
			diffs = append(diffs, unifiedDiff(ch.Path, "", ch.Diff))
		case "delete":
			diffs = append(diffs, deletedDiff(ch.Path, ch.Diff))
		default:
			diffs = append(diffs, fmt.Sprintf("--- a/%s\n+++ b/%s\n%s", ch.Path, ch.Path, ch.Diff))
		}
	}
	t.Title, t.Diff = strings.Join(paths, ", "), strings.Join(diffs, "")
	return t
}

// under is it with a patch's paths made relative to cwd: Codex sends them
// absolute (measured), and a diff header spells them a/<path>.
func (it codexItem) under(cwd string) codexItem {
	changes := slices.Clone(it.Changes)
	for i, ch := range changes {
		if rel, err := filepath.Rel(cwd, ch.Path); err == nil && filepath.IsLocal(rel) {
			changes[i].Path = filepath.ToSlash(rel)
		}
	}
	it.Changes = changes
	return it
}

// deletedDiff is content removed with its file.
func deletedDiff(path, content string) string {
	lines := diffLines(content)
	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ /dev/null\n@@ -%s +0,0 @@\n", path, hunkRange(len(lines)))
	for _, line := range lines {
		b.WriteString("-" + line + "\n")
	}
	return b.String()
}

// argsTitle is the first string argument of a call, what it acts on.
func argsTitle(raw json.RawMessage) string {
	var args map[string]any
	_ = json.Unmarshal(raw, &args)
	for _, key := range []string{"path", "file_path", "command", "query", "url", "name"} {
		if v, ok := args[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// mcpResultText is the text blocks of an MCP tool's result.
func mcpResultText(raw json.RawMessage) string {
	var r struct {
		Content []acpContent `json:"content"`
	}
	_ = json.Unmarshal(raw, &r)
	var parts []string
	for _, c := range r.Content {
		if c.Type == "text" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// userText is a userMessage's text inputs.
func (it codexItem) userText() string {
	var inputs []acpContent
	_ = json.Unmarshal(it.Content, &inputs)
	var parts []string
	for _, in := range inputs {
		if in.Type == "text" {
			parts = append(parts, in.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// codexTurn is one turn of thread/turns/list.
type codexTurn struct {
	ID    string      `json:"id"`
	Items []codexItem `json:"items"`
}

// codexTurnsPage bounds one thread/turns/list page.
const codexTurnsPage = 50

// readHistory reads thread t's turns into the chat, oldest first, each
// answer named by the model and effort its turn ran on, which the rollout
// records per turn and the turns do not. thread/resume's own full history is
// deprecated for the threads Codex writes now (0.170.0 says so).
func (d *codexDriver) readHistory(ctx context.Context, t codexThread) error {
	producers := codexTurnProducers(t.Thread.Path)
	cursor := ""
	for {
		params := map[string]any{"threadId": t.Thread.ID, "sortDirection": "asc", "itemsView": "full", "limit": codexTurnsPage}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := d.call(ctx, "thread/turns/list", params)
		if err != nil {
			return err
		}
		var page struct {
			Data []codexTurn `json:"data"`
			Next *string     `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return err
		}
		for _, turn := range page.Data {
			d.replayTurn(turn, producers[turn.ID])
		}
		if page.Next == nil || *page.Next == "" {
			return nil
		}
		cursor = *page.Next
	}
}

func (d *codexDriver) replayTurn(turn codexTurn, p codexProducer) {
	model := d.modelName(p.Model)
	for _, it := range turn.Items {
		switch it.Type {
		case "userMessage":
			d.c.conv.userMessage(it.userText(), nil)
			continue
		case codexCompactItem:
			d.replayCompaction(turn)
			continue
		}
		ci, ok := it.under(d.c.cwd).chatItem(true)
		if !ok || (ci.Kind != api.ItemTool && ci.Text == "") {
			continue
		}
		ci.Model, ci.Effort = model, p.Effort
		d.c.conv.put(ci, true)
	}
}

// codexProducer is what a turn ran on, as its rollout's turn_context says.
type codexProducer struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

// codexTurnProducers reads every turn_context of the rollout at path, by turn
// id; none where the file cannot be read.
func codexTurnProducers(path string) map[string]codexProducer {
	out := map[string]codexProducer{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if strings.Contains(string(line), `"turn_context"`) {
			var entry struct {
				Type    string `json:"type"`
				Payload struct {
					Turn string `json:"turn_id"`
					codexProducer
				} `json:"payload"`
			}
			if json.Unmarshal(line, &entry) == nil && entry.Type == "turn_context" && entry.Payload.Turn != "" {
				out[entry.Payload.Turn] = entry.Payload.codexProducer
			}
		}
		if err != nil {
			return out
		}
	}
}

// codexPlanStates maps a turn/plan/updated step's status onto a plan entry's
// (codex app-server generate-json-schema, 0.170.0).
var codexPlanStates = map[string]string{
	"pending":    api.PlanPending,
	"inProgress": api.PlanInProgress,
	"completed":  api.PlanCompleted,
}

// codexPlan reads turn/plan/updated: Codex names a step once, with no form for
// while it runs.
func codexPlan(params json.RawMessage) []api.ChatPlanEntry {
	var e struct {
		Plan []struct {
			Step   string `json:"step"`
			Status string `json:"status"`
		} `json:"plan"`
	}
	_ = json.Unmarshal(params, &e)
	out := make([]api.ChatPlanEntry, 0, len(e.Plan))
	for _, s := range e.Plan {
		status, ok := codexPlanStates[s.Status]
		if !ok {
			status = api.PlanPending
		}
		out = append(out, api.ChatPlanEntry{Content: s.Step, Status: status})
	}
	return out
}
