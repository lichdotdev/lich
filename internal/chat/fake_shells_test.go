//go:build !windows

package chat

// fakeShellPath is the file the fake's background command writes to, as
// Claude Code names it.
const fakeShellPath = "/tmp/claude-1000/-proj/s/tasks/bsh1.output"

// claudeShellTurn is Claude Code leaving one command running in the
// background, as 2.1.330 writes it: the call, its task_started, the result
// naming the output file; a slow foreground command, which is a task too; and
// a subagent's task, which is not a shell.
func (f *fake) claudeShellTurn(msg func(string, map[string]any)) {
	started := func(id, toolUse, desc string, bg bool, kind string) {
		f.send(map[string]any{"type": "system", "subtype": "task_started", "task_id": id, "tool_use_id": toolUse, "description": desc, "is_backgrounded": bg, "task_type": kind})
	}
	result := func(toolUse, text string) {
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": toolUse, "content": text}}}})
	}
	msg("m2", map[string]any{"type": "tool_use", "id": "toolu_bg", "name": "Bash", "input": map[string]any{"command": "npm run dev", "description": "Start the dev server", "run_in_background": true}})
	started("bsh1", "toolu_bg", "Start the dev server", true, "local_bash")
	result("toolu_bg", "Command running in background with ID: bsh1. Output is being written to: "+fakeShellPath+". You will be notified when it completes.")
	msg("m3", map[string]any{"type": "tool_use", "id": "toolu_fg", "name": "Bash", "input": map[string]any{"command": "sleep 7"}})
	started("bfg1", "toolu_fg", "Wait", false, "local_bash")
	f.send(map[string]any{"type": "system", "subtype": "task_notification", "task_id": "bfg1", "tool_use_id": "toolu_fg", "status": "completed"})
	result("toolu_fg", "done")
	started("a1", "toolu_agent", "Explore", true, "local_agent")
	f.claudeResult(false, "completed", "")
}

// claudeShellControl serves stop_task and read_file for the fake's shell.
func (f *fake) claudeShellControl(subtype string, m map[string]any, ok func(any), fail func(string)) bool {
	switch subtype {
	case "stop_task":
		id := str(m, "request", "task_id")
		f.send(map[string]any{"type": "system", "subtype": "task_updated", "task_id": id, "patch": map[string]any{"status": "killed", "end_time": 1795318375995}})
		f.send(map[string]any{"type": "system", "subtype": "task_notification", "task_id": id, "status": "stopped", "output_file": fakeShellPath})
		ok(map[string]any{})
	case "read_file":
		if str(m, "request", "path") != fakeShellPath {
			fail("ENOENT: no such file")
			return true
		}
		ok(map[string]any{"contents": "ial line\n\x1b[32mready\x1b[0m on :5173\r\nbuild 10%\rbuild 100%\n", "offset": 4096, "read_bytes": 50, "total_bytes": 4146})
	default:
		return false
	}
	return true
}
