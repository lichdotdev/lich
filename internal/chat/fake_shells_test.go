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

// claudeWakeTurn is a turn that ends with a command still in the background,
// and the turn Claude Code starts on its own once it ends, as 2.1.330 writes
// them (measured): background_tasks_changed listing the shell, the turn's
// result; then, here once the test stops the shell, its notification, the
// list emptied, and system/init opening a turn no message started.
func (f *fake) claudeWakeTurn(msg func(string, map[string]any)) {
	msg("m2", map[string]any{"type": "tool_use", "id": "toolu_gate", "name": "Bash", "input": map[string]any{"command": "task gate", "run_in_background": true}})
	f.send(map[string]any{"type": "system", "subtype": "background_tasks_changed", "tasks": []any{map[string]any{"task_id": "bgate", "task_type": "local_bash", "description": "Run the gate"}}})
	f.send(map[string]any{"type": "system", "subtype": "task_started", "task_id": "bgate", "tool_use_id": "toolu_gate", "description": "Run the gate", "is_backgrounded": true, "task_type": "local_bash"})
	msg("m3", map[string]any{"type": "text", "text": "Gate running."})
	f.claudeResult(false, "completed", "Gate running.")
	m := f.next()
	if str(m, "request", "subtype") != "stop_task" {
		return
	}
	f.send(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": str(m, "request_id"), "response": map[string]any{}}})
	f.send(map[string]any{"type": "system", "subtype": "task_notification", "task_id": "bgate", "status": "completed"})
	f.send(map[string]any{"type": "system", "subtype": "background_tasks_changed", "tasks": []any{}})
	f.send(map[string]any{"type": "system", "subtype": "init", "session_id": "claude-new"})
	msg("m4", map[string]any{"type": "text", "text": "Gate green."})
	f.claudeResult(false, "completed", "Gate green.")
}

// claudeMovedTurn is a foreground command the person moves to the
// background, as 2.1.330 wrote it (measured): its task_started once it has
// run 2s, and on background_tasks the patch, the answer, the call's result
// and the turn carrying on to its end.
func (f *fake) claudeMovedTurn(msg func(string, map[string]any)) {
	msg("m2", map[string]any{"type": "tool_use", "id": "toolu_01MJPrhDuMGj39vkbze1sMz7", "name": "Bash", "input": map[string]any{"command": "timeout 20 tail -f /dev/null; echo finished-waiting", "description": "Wait 20 seconds in the foreground"}})
	f.send(map[string]any{"type": "system", "subtype": "task_started", "task_id": "bc4ibdw5i", "run_id": "0mwrtlro7-5717fd6e", "tool_use_id": "toolu_01MJPrhDuMGj39vkbze1sMz7", "description": "Wait 20 seconds in the foreground", "is_backgrounded": false, "task_type": "local_bash"})
	m := f.next()
	if str(m, "request", "subtype") != "background_tasks" {
		f.claudeResult(true, "error", "not moved")
		return
	}
	f.send(map[string]any{"type": "system", "subtype": "background_tasks_changed", "tasks": []any{map[string]any{"task_id": "bc4ibdw5i", "run_id": "0mwrtlro7-5717fd6e", "task_type": "local_bash", "description": "Wait 20 seconds in the foreground"}}})
	f.send(map[string]any{"type": "system", "subtype": "task_updated", "task_id": "bc4ibdw5i", "run_id": "0mwrtlro7-5717fd6e", "patch": map[string]any{"is_backgrounded": true}})
	f.send(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": str(m, "request_id"), "response": map[string]any{}}})
	f.send(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{map[string]any{"tool_use_id": "toolu_01MJPrhDuMGj39vkbze1sMz7", "type": "tool_result", "is_error": false,
		"content": "Command was manually backgrounded by user with ID: bc4ibdw5i. Output is being written to: " + claudeMovedPath + ". If it is still running after 30m in the background, it will be stopped and you will be notified."}}}})
	msg("m3", map[string]any{"type": "text", "text": "Not yet."})
	f.claudeResult(false, "completed", "Not yet.")
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
