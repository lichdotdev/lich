package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
)

// A Claude Code chat's background shells, as measured on 2.1.330: a command
// its Bash tool runs is a task of type local_bash, reported by
// system/task_started with is_backgrounded (false for a foreground command
// slow enough to become a task, which moving to the background patches), then
// task_updated with its patch.status (completed, failed, killed) and
// task_notification with its end (completed, failed, stopped). The tool's
// result names the file the output is written to. stop_task stops one, and
// read_file reads the file's end through Claude Code itself, which sees it
// where a sandbox's private /tmp hides it from lich.

const (
	claudeShellTask = "local_bash"
	// claudeOutputAt opens the sentence of every background variant of the
	// Bash tool's result that names the output file.
	claudeOutputAt = "Output is being written to: "
	// claudeShellTail is how much of the output's end a read takes.
	claudeShellTail = 16 << 10
)

type claudeShell struct {
	api.ChatShell
	toolUse, path string
	background    bool
}

// claudeShells is the chat's local_bash tasks in the order they started,
// background or not yet.
type claudeShells struct {
	mu   sync.Mutex
	list []*claudeShell
}

// claudeTaskLine is the fields of a system/task_* line.
type claudeTaskLine struct {
	TaskID       string `json:"task_id"`
	TaskType     string `json:"task_type"`
	ToolUseID    string `json:"tool_use_id"`
	Description  string `json:"description"`
	Backgrounded *bool  `json:"is_backgrounded"`
	Status       string `json:"status"`
	OutputFile   string `json:"output_file"`
	Patch        struct {
		Status       string `json:"status"`
		EndTime      int64  `json:"end_time"`
		Backgrounded *bool  `json:"is_backgrounded"`
	} `json:"patch"`
}

// task reads a system/task_* line, and reports whether the shells shown
// changed. command is the call's command, "" where lich holds no item for it.
func (s *claudeShells) task(subtype string, raw []byte, command func(toolUse string) string, now time.Time) bool {
	var t claudeTaskLine
	if json.Unmarshal(raw, &t) != nil || t.TaskID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if subtype == "task_started" {
		if t.TaskType != claudeShellTask {
			return false
		}
		cmd := command(t.ToolUseID)
		if cmd == "" {
			cmd = t.Description
		}
		bg := t.Backgrounded != nil && *t.Backgrounded
		s.list = append(s.list, &claudeShell{
			ID: t.TaskID, Command: cmd, Description: t.Description, State: api.ShellRunning, Started: now.UnixMilli(),
			toolUse: t.ToolUseID, background: bg,
		})
		return bg
	}
	sh := s.find(t.TaskID)
	if sh == nil {
		return false
	}
	shown := sh.background
	if b := t.Patch.Backgrounded; b != nil {
		sh.background = *b
	}
	if t.OutputFile != "" {
		sh.path = t.OutputFile
	}
	status := t.Status
	if subtype == "task_updated" {
		status = t.Patch.Status
	}
	if state := claudeShellState(status); state != "" && sh.State == api.ShellRunning {
		sh.State, sh.Ended = state, t.Patch.EndTime
		if sh.Ended == 0 {
			sh.Ended = now.UnixMilli()
		}
	}
	if !sh.background && sh.State != api.ShellRunning {
		s.drop(sh.ID)
	}
	return shown || sh.background
}

// claudeShellState is the shell's state a task status names, "" for one
// that is no end.
func claudeShellState(status string) string {
	switch status {
	case "completed":
		return api.ShellCompleted
	case "failed":
		return api.ShellFailed
	case "killed", "stopped":
		return api.ShellStopped
	}
	return ""
}

// result reads the output file off the Bash call's result.
func (s *claudeShells) result(toolUse, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sh := range s.list {
		if sh.toolUse == toolUse {
			if p := claudeOutputPath(text, sh.ID); p != "" {
				sh.path = p
			}
		}
	}
}

// claudeOutputPath is the output file a background Bash result names, which
// ends in the task's id: the path itself may hold any character.
func claudeOutputPath(text, id string) string {
	_, rest, ok := strings.Cut(text, claudeOutputAt)
	if !ok {
		return ""
	}
	end := strings.Index(rest, id+".output")
	if end < 0 {
		return ""
	}
	return rest[:end+len(id)+len(".output")]
}

func (s *claudeShells) find(id string) *claudeShell {
	for _, sh := range s.list {
		if sh.ID == id {
			return sh
		}
	}
	return nil
}

func (s *claudeShells) drop(id string) {
	for i, sh := range s.list {
		if sh.ID == id {
			s.list = append(s.list[:i], s.list[i+1:]...)
			return
		}
	}
}

// shown is the background shells.
func (s *claudeShells) shown() []api.ChatShell {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []api.ChatShell{}
	for _, sh := range s.list {
		if sh.background {
			out = append(out, sh.ChatShell)
		}
	}
	return out
}

// lookup is background shell id, false when the chat has none by that id.
func (s *claudeShells) lookup(id string) (claudeShell, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sh := s.find(id)
	if sh == nil || !sh.background {
		return claudeShell{}, false
	}
	return *sh, true
}

// shellResults reads the output files a live user line's tool results name,
// a subagent's among them.
func (d *claudeDriver) shellResults(line claudeLine) {
	var blocks []claudeBlock
	if json.Unmarshal(line.Message.Content, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		if b.Type == "tool_result" {
			d.shells.result(b.ToolUseID, blockText(b.Content))
		}
	}
}

func (d *claudeDriver) shellTask(subtype string, raw []byte) {
	command := func(toolUse string) string {
		if it, ok := d.c.conv.get(toolUse); ok && it.Tool != nil {
			return it.Tool.Title
		}
		return ""
	}
	if d.shells.task(subtype, raw, command, time.Now()) {
		d.c.setShells(d.shells.shown())
	}
}

func (d *claudeDriver) stopShell(ctx context.Context, id string) error {
	sh, ok := d.shells.lookup(id)
	if !ok {
		return fmt.Errorf("no background shell %q", id)
	}
	if sh.State != api.ShellRunning {
		return fmt.Errorf("the shell %q has already ended", id)
	}
	_, err := d.control(ctx, map[string]any{"subtype": "stop_task", "task_id": id})
	return err
}

func (d *claudeDriver) shellOutput(ctx context.Context, id string) (string, error) {
	sh, ok := d.shells.lookup(id)
	if !ok {
		return "", fmt.Errorf("no background shell %q", id)
	}
	if sh.path == "" {
		return "", nil
	}
	r, err := d.control(ctx, map[string]any{"subtype": "read_file", "path": sh.path, "max_bytes": claudeShellTail, "offset": -claudeShellTail})
	if err != nil {
		return "", err
	}
	var read struct {
		Contents string `json:"contents"`
		Offset   int64  `json:"offset"`
	}
	if err := json.Unmarshal(r.Body, &read); err != nil {
		return "", err
	}
	return outputTail(read.Contents, read.Offset > 0), nil
}

// outputTail is the end of a shell's output as plain text: the first line
// left out when the read began inside it, the terminal's escapes and
// carriage-return redraws gone.
func outputTail(text string, cut bool) string {
	if cut {
		if _, rest, ok := strings.Cut(text, "\n"); ok {
			text = rest
		}
	}
	text = plainText(text)
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		// a progress bar redraws its line after \r: the last drawing stands
		if j := strings.LastIndex(strings.TrimRight(l, "\r"), "\r"); j >= 0 {
			l = l[j+1:]
		}
		lines[i] = strings.TrimRight(l, "\r")
	}
	return strings.Join(lines, "\n")
}
