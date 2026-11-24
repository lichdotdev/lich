//go:build !windows

package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The provider a chat test talks to is this test binary run again with
// fakeEnv set (TestMain): it speaks Claude Code's stream-json when spawned with
// -p and ACP when spawned with acp, and answers each message by its text with
// one scripted turn, the shapes measured against the real CLIs; spawned with
// app-server it speaks Codex's (fake_codex_test.go).
const fakeEnv = "LICH_CHAT_FAKE"

// fakeLogEnv names a file the fake appends every line it reads to, for a test
// that checks what lich sent.
const fakeLogEnv = "LICH_CHAT_FAKE_LOG"

type fake struct {
	in   *bufio.Reader
	log  *os.File
	args []string
	// asks counts the fake's opencode questions, which name their tool calls,
	// read by its server too, and answers is where that server puts the
	// person's answer.
	asks    atomic.Int64
	answers chan string
	// out keeps a line the server writes whole beside the reader's.
	out sync.Mutex
}

func runFake(args []string) {
	f := &fake{in: bufio.NewReader(os.Stdin), args: args}
	if path := os.Getenv(fakeLogEnv); path != "" {
		f.log, _ = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		fmt.Fprintf(f.log, "argv %s\n", strings.Join(args, " "))
	}
	if len(args) > 1 && args[0] == "plugin" {
		f.claudePluginCLI(args)
		return
	}
	if slices.Contains(args, "edit-permission-rules") {
		f.editPermissionRules()
		return
	}
	if slices.Contains(args, "edit-memory-settings") {
		f.editMemorySettings()
		return
	}
	if slices.Contains(args, "edit-skill-overrides") {
		f.editSkillOverrides()
		return
	}
	if slices.Contains(args, "acp") {
		f.acp(args)
		return
	}
	if slices.Contains(args, "app-server") {
		f.codex()
		return
	}
	if slices.Contains(args, "--no-session-persistence") || slices.Contains(args, "--no-title") {
		f.claudeTitle()
		return
	}
	if len(args) > 1 && args[0] == "session" && args[1] == "delete" {
		return
	}
	if len(args) > 1 && args[0] == "run" && slices.Contains(args, "--format") {
		f.openCodeTitle()
		return
	}
	f.claude(args)
}

// openCodeTitle answers `opencode run --format json`: the events of a call that
// made a session and wrote its reply, with the stray lines a log adds.
func (f *fake) openCodeTitle() {
	if os.Getenv("LICH_SESSION_ID") != "" {
		fmt.Println(`{"type":"text","sessionID":"ses_leak","part":{"text":"leak"}}`)
		return
	}
	fmt.Println(`not json`)
	fmt.Println(`{"type":"step_start","sessionID":"ses_fake","part":{}}`)
	fmt.Println(`{"type":"text","sessionID":"ses_fake","part":{"text":"\"Fake title.\""}}`)
}

// claudeTitle answers the print-mode call that names a chat, a quoted title with a
// stop as a model writes one, or "leak" if the hooks' variables reached it.
func (f *fake) claudeTitle() {
	if os.Getenv("LICH_SESSION_ID") != "" {
		fmt.Println("leak")
		return
	}
	fmt.Println(`"Fake title."`)
}

func (f *fake) send(v any) {
	line, _ := json.Marshal(v)
	f.out.Lock()
	defer f.out.Unlock()
	fmt.Println(string(line))
}

// next reads one message, nil at EOF.
func (f *fake) next() map[string]any {
	for {
		line, err := f.in.ReadString('\n')
		if strings.TrimSpace(line) != "" {
			if f.log != nil {
				fmt.Fprint(f.log, line)
			}
			var m map[string]any
			if json.Unmarshal([]byte(line), &m) == nil {
				return m
			}
		}
		if err != nil {
			return nil
		}
	}
}

func str(m map[string]any, path ...string) string {
	var v any = m
	for _, k := range path {
		mm, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = mm[k]
	}
	s, _ := v.(string)
	return s
}

func (f *fake) claude(args []string) {
	session := "claude-new"
	if i := slices.Index(args, "--resume"); i >= 0 && i+1 < len(args) {
		session = args[i+1]
		if session == "drifts" {
			session = "claude-other"
		}
	}
	run := newFakeClaude(args)
	for m := f.next(); m != nil; m = f.next() {
		switch str(m, "type") {
		case "control_request":
			f.claudeControl(run, m)
		case "bash_command":
			f.claudeShell(str(m, "command"))
		case "user":
			f.send(map[string]any{"type": "system", "subtype": "init", "session_id": session, "model": run.model, "permissionMode": run.mode,
				"skills": []any{"deploy"}, "terminal_slash_commands": []any{"doctor"}})
			f.claudeTurn(run, claudeText(m))
		}
	}
}

// claudeText is a user message's text: its content, or its first text block
// when the content is blocks.
func claudeText(m map[string]any) string {
	msg, _ := m["message"].(map[string]any)
	blocks, ok := msg["content"].([]any)
	if !ok {
		return str(m, "message", "content")
	}
	for _, b := range blocks {
		if b, _ := b.(map[string]any); str(b, "type") == "text" {
			return str(b, "text")
		}
	}
	return ""
}

// claudeShell answers a bash_command with the two replay rows Claude Code
// writes, XML-escaped (measured on 2.1.330): the command, then its output, or
// why it did not run. "fail" exits 4 and "sleep" waits for the interrupt.
func (f *fake) claudeShell(command string) {
	row := func(content string) {
		f.send(map[string]any{"type": "user", "isReplay": true, "parent_tool_use_id": nil, "message": map[string]any{"role": "user", "content": content}})
	}
	if strings.TrimSpace(command) == "" {
		row("<bash-stderr>Not run: no command was given.</bash-stderr>")
		return
	}
	row("<bash-input>" + claudeEscape.Replace(command) + "</bash-input>")
	switch command {
	case "fail":
		row("<bash-stdout></bash-stdout><bash-stderr>x &gt; y</bash-stderr><bash-exit-code>4</bash-exit-code>")
	case "long":
		row("<bash-stdout>" + strings.Repeat("x", claudeShellToModel+100) + "</bash-stdout><bash-stderr></bash-stderr><bash-exit-code>0</bash-exit-code>")
	case "sleep":
		if m := f.next(); str(m, "type") == "control_request" {
			f.send(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": str(m, "request_id"), "response": map[string]any{"still_queued": []any{}}}})
		}
		row("<bash-stdout></bash-stdout><bash-stderr>[interrupted]</bash-stderr><bash-exit-code>130</bash-exit-code>")
	default:
		row("<bash-stdout>ran " + claudeEscape.Replace(command) + "</bash-stdout><bash-stderr></bash-stderr><bash-exit-code>0</bash-exit-code>")
	}
}

func (f *fake) claudeResult(isError bool, terminal, text string) {
	f.send(map[string]any{"type": "result", "subtype": "success", "is_error": isError, "terminal_reason": terminal, "result": text})
}

func (f *fake) claudeTurn(run *fakeClaude, text string) {
	msg := func(id string, block map[string]any) {
		f.send(map[string]any{"type": "assistant", "parent_tool_use_id": nil, "message": map[string]any{"id": id, "model": run.model, "content": []any{block}}})
	}
	// the TUI's two lines of a shell command, which the agent answers; "slow"
	// answers as the slow prompt does
	if text == "<bash-input>slow</bash-input>" {
		text = "slow"
	} else if strings.HasPrefix(text, "<bash-input>") {
		msg("m8", map[string]any{"type": "text", "text": "The command ran."})
		f.claudeResult(false, "completed", "The command ran.")
		return
	}
	if f.claudeCompactTurn(msg, text) {
		return
	}
	// /effort is a command Claude Code runs itself: it writes the command and
	// what it printed as user lines, and get_settings answers the new level.
	if level, ok := strings.CutPrefix(text, "/effort "); ok {
		run.effort = level
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": "<command-name>/effort</command-name>\n<command-message>effort</command-message>\n<command-args>" + level + "</command-args>"}})
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": "<local-command-stdout>Set effort level to " + level + "</local-command-stdout>"}})
		f.claudeResult(false, "completed", "")
		return
	}
	// an envelope plays as "suggest" does: lich, not the fake, drops its
	// suggestion
	if strings.HasPrefix(text, "<lich-notification ") {
		text = "suggest"
	}
	switch text {
	case "usage":
		// 2.1.330's own figures: the plan's windows once per process, then a
		// result with the conversation's cost and each model's window.
		f.send(map[string]any{"type": "rate_limit_event", "session_id": "claude-new", "rate_limit_info": map[string]any{
			"status": "allowed", "unifiedWindows": map[string]any{
				"seven_day": map[string]any{"utilization": 0.55, "resetsAt": 1795446000, "status": "allowed"},
				"five_hour": map[string]any{"utilization": 0.42, "resetsAt": 1795437000, "status": "allowed"},
			}}})
		msg("m11", map[string]any{"type": "text", "text": "counted"})
		f.send(map[string]any{"type": "result", "subtype": "success", "is_error": false, "terminal_reason": "completed", "result": "counted",
			"session_id": "claude-new", "total_cost_usd": 0.25, "modelUsage": map[string]any{
				"claude-haiku-5-5": map[string]any{"costUSD": 0.01, "contextWindow": 1000000},
				run.model:          map[string]any{"costUSD": 0.24, "contextWindow": 200000},
			}})
	case "suggest":
		msg("m10", map[string]any{"type": "text", "text": "Fixed."})
		f.claudeResult(false, "completed", "Fixed.")
		f.send(map[string]any{"type": "prompt_suggestion", "suggestion": " commit this ", "uuid": "u1", "session_id": "claude-new"})
	case "hi":
		f.send(map[string]any{"type": "stream_event", "api_message_id": "m0", "event": map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "thinking_delta", "thinking": ""}}})
		msg("m0", map[string]any{"type": "thinking", "thinking": ""})
		for _, part := range []string{"O", "i", "!"} {
			f.send(map[string]any{"type": "stream_event", "api_message_id": "m1", "event": map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": part}}})
		}
		msg("m1", map[string]any{"type": "text", "text": "Oi!"})
		f.claudeResult(false, "completed", "Oi!")
	case "tasks":
		f.claudeTasks(msg)
	case "shells":
		f.claudeShellTurn(msg)
	case "wait":
		f.claudeMovedTurn(msg)
	case "wake":
		f.claudeWakeTurn(msg)
	case "workflow":
		for line := range bytes.Lines(claudeWorkflowLines) {
			f.send(json.RawMessage(bytes.TrimSpace(line)))
		}
		f.claudeResult(false, "completed", "")
	case "todos":
		msg("m2", map[string]any{"type": "tool_use", "id": "toolu_t", "name": "TodoWrite", "input": map[string]any{"todos": []any{
			map[string]any{"content": "Read the parser", "status": "completed", "activeForm": "Reading the parser"},
			map[string]any{"content": "Fix it", "status": "in_progress", "activeForm": "Fixing it"},
		}}})
		f.claudeResult(false, "completed", "")
	case "tool":
		msg("m2", map[string]any{"type": "thinking", "thinking": "reading"})
		msg("m2", map[string]any{"type": "tool_use", "id": "toolu_1", "name": "Edit", "input": map[string]any{"file_path": "a.txt", "old_string": "x", "new_string": "y"}})
		msg("m3", map[string]any{"type": "tool_use", "id": "toolu_sub", "name": "Read", "input": map[string]any{}})
		f.send(map[string]any{"type": "assistant", "parent_tool_use_id": "toolu_sub", "message": map[string]any{"id": "m9", "content": []any{map[string]any{"type": "text", "text": "subagent"}}}})
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": []any{map[string]any{"type": "text", "text": "edited"}}}}}})
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_sub", "is_error": true, "content": "nope"}}}})
		f.claudeResult(false, "completed", "")
	case "local":
		// A local command, and the synthetic answer Claude Code writes after
		// it, streamed a few characters at a time.
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": "<command-name>/model</command-name>\n<command-message>model</command-message>\n<command-args>haiku</command-args>"}})
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": "<local-command-stdout>Set model to haiku (claude-haiku-5-5)</local-command-stdout>"}})
		for _, part := range []string{"No resp", "onse requ", "ested."} {
			f.send(map[string]any{"type": "stream_event", "api_message_id": "m6", "event": map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": part}}})
		}
		msg("m6", map[string]any{"type": "text", "text": "No response requested."})
		for _, part := range []string{"No ", "way"} {
			f.send(map[string]any{"type": "stream_event", "api_message_id": "m7", "event": map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": part}}})
		}
		msg("m7", map[string]any{"type": "text", "text": "No way"})
		f.claudeResult(false, "completed", "")
	case "interrupted":
		// What Claude Code writes into the user's turn when it is interrupted:
		// as text, as a block, and for a tool call.
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": "[Request interrupted by user]"}})
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "[Request interrupted by user for tool use]"}}}})
		msg("m5", map[string]any{"type": "text", "text": "stopped"})
		f.claudeResult(false, "completed", "")
	case "perm":
		f.send(map[string]any{"type": "control_request", "request_id": "r1", "request": map[string]any{"subtype": "can_use_tool", "tool_name": "Write", "input": map[string]any{"file_path": "n.txt", "content": "ok"}, "permission_suggestions": []any{map[string]any{"type": "setMode"}}, "tool_use_id": "toolu_w"}})
		reply := f.next()
		msg("m4", map[string]any{"type": "text", "text": str(reply, "response", "response", "behavior")})
		f.claudeResult(false, "completed", "")
	case "question", "question-then-write":
		f.claudeQuestion(msg, text == "question-then-write")
	case "exit-plan":
		f.claudeExitPlan(msg)
	case "slow":
		msg("m5", map[string]any{"type": "text", "text": "counting"})
		if m := f.next(); str(m, "request", "subtype") == "interrupt" {
			f.send(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": str(m, "request_id")}})
		}
		f.claudeResult(true, "aborted_streaming", "")
	case "fail":
		f.claudeResult(true, "", "API Error: 401 key expired")
	case "ask":
		f.send(map[string]any{"type": "control_request", "request_id": "r2", "request": map[string]any{"subtype": "hook_callback"}})
		f.next()
		f.claudeResult(false, "completed", "")
	case "plan":
		f.send(map[string]any{"type": "system", "subtype": "status", "permissionMode": "plan"})
		f.claudeResult(false, "completed", "")
	case "sed":
		// A shell command that edits a.txt and writes c.txt, then a Read
		// that changes nothing, each reported done before the next starts.
		msg("m2", map[string]any{"type": "tool_use", "id": "toolu_sed", "name": "Bash", "input": map[string]any{"command": "sed -i s/x/z/ a.txt"}})
		_ = os.WriteFile("a.txt", []byte("z\n"), 0o644)
		_ = os.WriteFile("c.txt", []byte("new\n"), 0o644)
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_sed", "content": "ok"}}}})
		msg("m3", map[string]any{"type": "tool_use", "id": "toolu_read", "name": "Read", "input": map[string]any{"file_path": "a.txt"}})
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_read", "content": "z"}}}})
		f.claudeResult(false, "completed", "")
	case "write":
		f.claudeEdit(false)
	case "commit":
		f.claudeEdit(true)
	case "die":
		fmt.Fprintln(os.Stderr, "boom")
		os.Exit(3)
	}
}

func (f *fake) acp(args []string) {
	opts := newFakeACP(args)
	f.openCodeServe()
	for m := f.next(); m != nil; m = f.next() {
		id, method := m["id"], str(m, "method")
		params, _ := m["params"].(map[string]any)
		switch method {
		case "initialize":
			f.result(id, map[string]any{"agentCapabilities": map[string]any{"loadSession": os.Getenv("LICH_CHAT_FAKE_NOLOAD") == ""}})
		case "session/new":
			f.result(id, opts.session(map[string]any{"sessionId": "ses-new"}))
			f.acpCommands()
		case "session/set_config_option", "session/set_model", "session/set_mode":
			f.acpSet(opts, id, method, params)
		case "session/load":
			f.acpLoad(id, str(params, "sessionId"), opts)
		case "session/prompt":
			blocks, _ := params["prompt"].([]any)
			first, _ := blocks[0].(map[string]any)
			f.acpTurn(id, str(first, "text"))
		}
	}
}

func (f *fake) result(id any, v any) {
	f.send(map[string]any{"jsonrpc": "2.0", "id": id, "result": v})
}

func (f *fake) update(u map[string]any) {
	f.send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "s", "update": u}})
}

func chunk(kind, text string) map[string]any {
	return map[string]any{"sessionUpdate": kind, "content": map[string]any{"type": "text", "text": text}}
}

func (f *fake) acpLoad(id any, session string, opts *fakeACP) {
	switch session {
	case "gone":
		f.send(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32002, "message": "session not found"}})
		return
	case "omp-empty":
	case "omp-shelled":
		// opencode files a shell command under a user message for the agent
		// alone (measured on 1.19.4).
		f.update(map[string]any{"sessionUpdate": "user_message_chunk", "content": map[string]any{"type": "text", "text": "The following tool was executed by the user", "annotations": map[string]any{"audience": []any{"assistant"}}}})
		f.update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t9", "title": "ls", "kind": "execute", "status": "completed"})
		f.update(chunk("user_message_chunk", "and now?"))
	default:
		f.update(chunk("user_message_chunk", "remember caju"))
		f.update(chunk("agent_message_chunk", "ok"))
		f.update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t0", "title": "Reading a", "kind": "read", "status": "completed"})
	}
	f.result(id, opts.session(map[string]any{}))
}

func (f *fake) acpTurn(id any, text string) {
	if f.acpCompactTurn(id, text) {
		return
	}
	// an envelope plays as "suggest" does: lich drops its suggestion
	if strings.HasPrefix(text, "<lich-notification ") {
		text = "suggest"
	}
	switch text {
	case "suggest":
		f.update(chunk("agent_message_chunk", "Fixed."))
		f.update(map[string]any{"sessionUpdate": "prompt_suggestions_update", "suggestions": []any{
			map[string]any{"prompt": []any{map[string]any{"type": "text", "text": " run the tests "}}},
			map[string]any{"prompt": []any{map[string]any{"type": "text", "text": "commit it"}}},
		}})
	case "hi":
		f.update(chunk("user_message_chunk", "hi"))
		f.update(chunk("agent_thought_chunk", "hmm"))
		for _, part := range []string{"O", "i", "!"} {
			f.update(chunk("agent_message_chunk", part))
		}
		f.update(map[string]any{"sessionUpdate": "usage_update", "used": 1})
	case "plan":
		f.update(map[string]any{"sessionUpdate": "plan", "entries": []any{
			map[string]any{"content": "Read the parser", "priority": "high", "status": "completed"},
			map[string]any{"content": "Fix it", "priority": "medium", "status": "in_progress"},
		}})
	case "plan-cleared":
		f.update(map[string]any{"sessionUpdate": "plan", "entries": []any{}})
	case "tool":
		f.update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t1", "title": "Editing a.txt", "kind": "edit", "status": "pending", "_meta": map[string]any{"kiro": map[string]any{"toolName": "write"}}})
		f.update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "t1", "status": "in_progress"})
		f.update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "t1", "status": "completed", "content": []any{
			map[string]any{"type": "content", "content": map[string]any{"type": "text", "text": "wrote"}},
			map[string]any{"type": "diff", "path": "a.txt", "oldText": "x", "newText": "y"},
		}})
		f.update(chunk("agent_message_chunk", "done"))
	case "perm":
		f.send(map[string]any{"jsonrpc": "2.0", "id": "p1", "method": "session/request_permission", "params": map[string]any{
			"toolCall": map[string]any{"toolCallId": "t2", "title": "/outside", "kind": "other", "status": "pending"},
			"options": []any{
				map[string]any{"optionId": "once", "name": "Allow once", "kind": "allow_once"},
				map[string]any{"optionId": "reject", "name": "Reject", "kind": "reject_once"},
			},
		}})
		reply := f.next()
		result, _ := reply["result"].(map[string]any)
		outcome, _ := result["outcome"].(map[string]any)
		f.update(chunk("agent_message_chunk", str(outcome, "outcome")+":"+str(outcome, "optionId")))
	case "slow":
		f.update(chunk("agent_message_chunk", "counting"))
		for m := f.next(); m != nil && str(m, "method") != "session/cancel"; m = f.next() {
			if str(m, "method") == "" {
				f.update(chunk("agent_message_chunk", "answered:"+fmt.Sprint(m["result"])))
			}
		}
		f.result(id, map[string]any{"stopReason": "cancelled"})
		return
	case "permslow":
		f.send(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "session/request_permission", "params": map[string]any{
			"toolCall": map[string]any{"toolCallId": "t3", "title": "x"},
			"options":  []any{map[string]any{"optionId": "once", "kind": "allow_once"}},
		}})
		var outcome string
		for cancelled := false; outcome == "" || !cancelled; {
			m := f.next()
			if m == nil {
				return
			}
			if str(m, "method") == "session/cancel" {
				cancelled = true
			} else if result, ok := m["result"].(map[string]any); ok {
				o, _ := result["outcome"].(map[string]any)
				outcome = str(o, "outcome")
			}
		}
		f.update(chunk("agent_message_chunk", "after:"+outcome))
		f.result(id, map[string]any{"stopReason": "cancelled"})
		return
	case "err":
		f.send(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32603, "message": "Model access is disabled"}})
		return
	case "octool":
		f.openCodeQuestion()
	case "question", "question-then-write", "question-url":
		if f.acpElicit(id, text) {
			return
		}
	case "fs":
		f.send(map[string]any{"jsonrpc": "2.0", "id": 99, "method": "fs/write_text_file", "params": map[string]any{"path": "/x", "content": "y"}})
		reply := f.next()
		errObj, _ := reply["error"].(map[string]any)
		f.update(chunk("agent_message_chunk", fmt.Sprint(errObj["code"])))
	case "mode":
		f.update(map[string]any{"sessionUpdate": "current_mode_update", "currentModeId": "plan"})
	case "die":
		fmt.Fprintln(os.Stderr, "acp boom")
		os.Exit(3)
	case "sleep":
		time.Sleep(time.Hour)
	}
	f.result(id, map[string]any{"stopReason": "end_turn"})
}

// claudeEdit changes the checkout the way a shell command would, behind the
// chat's back: a line appended to a.txt and a three-line b.txt, committed
// when asked.
func (f *fake) claudeEdit(commit bool) {
	a, _ := os.OpenFile("a.txt", os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = a.WriteString("y\n")
	_ = a.Close()
	_ = os.WriteFile("b.txt", []byte("1\n2\n3\n"), 0o644)
	if commit {
		_ = exec.Command("git", "add", "-A").Run()
		_ = exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "turn").Run()
	}
	f.claudeResult(false, "completed", "")
}

// fakeQuestions is what the fake asks with AskUserQuestion: a single choice
// with a preview, a multiple choice and a free-text question.
var fakeQuestions = map[string]any{"questions": []any{
	map[string]any{"question": "Which fruit?", "header": "Fruit", "multiSelect": false, "options": []any{
		map[string]any{"label": "Apple", "description": "red"}, map[string]any{"label": "Pear", "preview": "pear()"},
	}},
	map[string]any{"question": "Which colors?", "header": "Colors", "multiSelect": true, "options": []any{
		map[string]any{"label": "Red"}, map[string]any{"label": "Green"}, map[string]any{"label": "Blue"},
	}},
	map[string]any{"question": "Anything else?", "header": "Notes", "options": []any{}},
}}

// claudeQuestion asks AskUserQuestion the way Claude Code does (measured on
// 2.1.328): the tool_use first, then its can_use_tool. It writes back what the
// answer allowed or denied and, with write, the message that followed a deny.
func (f *fake) claudeQuestion(msg func(string, map[string]any), write bool) {
	msg("mq", map[string]any{"type": "tool_use", "id": "toolu_q", "name": "AskUserQuestion", "input": fakeQuestions})
	f.send(map[string]any{"type": "control_request", "request_id": "r3", "request": map[string]any{
		"subtype": "can_use_tool", "tool_name": "AskUserQuestion", "input": fakeQuestions, "tool_use_id": "toolu_q", "requires_user_interaction": true,
	}})
	reply := f.next()
	response, _ := reply["response"].(map[string]any)
	behavior, _ := response["response"].(map[string]any)
	if str(behavior, "behavior") == "allow" {
		updated, _ := behavior["updatedInput"].(map[string]any)
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_q", "content": "answered"}}}, "tool_use_result": updated})
		answers, _ := json.Marshal(updated["answers"])
		msg("mq2", map[string]any{"type": "text", "text": "answers:" + string(answers)})
	} else {
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_q", "is_error": true, "content": str(behavior, "message")}}}})
		msg("mq2", map[string]any{"type": "text", "text": "declined"})
	}
	if write {
		msg("mq3", map[string]any{"type": "text", "text": "then:" + str(f.next(), "message", "content")})
	}
	f.claudeResult(false, "completed", "")
}

// fakeElicitSchema is what the fake asks over elicitation/create, in the
// shapes oh-my-pi and Antigravity were measured sending: a single choice with
// its other property, a multiple choice with Antigravity's other suffix, and a
// question with no choices.
var fakeElicitSchema = map[string]any{"type": "object", "properties": map[string]any{
	"q0":        map[string]any{"type": "string", "title": "Which fruit?", "description": "Fruit", "oneOf": []any{map[string]any{"const": "1", "title": "Apple"}, map[string]any{"const": "2", "title": "Pear"}}},
	"q0__other": map[string]any{"type": "string", "title": "Other (type your own)"},
	"q1":        map[string]any{"type": "array", "title": "Which colors?", "items": map[string]any{"anyOf": []any{map[string]any{"const": "r", "title": "Red"}, map[string]any{"const": "b", "title": "Blue"}}}},
	"q1_other":  map[string]any{"type": "string", "title": "Other"},
	"q2":        map[string]any{"type": "string", "title": "Anything else?"},
}}

// acpElicit asks the fake's questions and writes back what came; with write it
// ends the turn on the decline, as oh-my-pi does, before the cancel a message
// sent past them brings (true).
func (f *fake) acpElicit(id any, text string) bool {
	mode := "form"
	if text == "question-url" {
		mode = "url"
	}
	f.send(map[string]any{"jsonrpc": "2.0", "id": 8, "method": "elicitation/create", "params": map[string]any{
		"sessionId": "s", "mode": mode, "message": "Answer 3 questions", "requestedSchema": fakeElicitSchema,
	}})
	reply := f.next()
	out, _ := json.Marshal(reply["result"])
	f.update(chunk("agent_message_chunk", "elicit:"+string(out)))
	if text != "question-then-write" {
		return false
	}
	f.result(id, map[string]any{"stopReason": "cancelled"})
	return true
}

// claudeTasks is Claude Code keeping its list with TaskCreate and TaskUpdate,
// as 2.1.330 does where the model has the tools: the task's id comes in the
// create's result, and a subagent's own calls are not the session's list.
func (f *fake) claudeTasks(msg func(string, map[string]any)) {
	result := func(id, text string, isError bool) {
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": isError, "content": text}}}})
	}
	msg("m2", map[string]any{"type": "tool_use", "id": "toolu_c1", "name": "TaskCreate", "input": map[string]any{"subject": "Read the parser", "activeForm": "Reading the parser"}})
	result("toolu_c1", "Task #1 created successfully: Read the parser", false)
	msg("m3", map[string]any{"type": "tool_use", "id": "toolu_c2", "name": "TaskCreate", "input": map[string]any{"subject": "Fix it", "activeForm": "Fixing it"}})
	result("toolu_c2", "Task #2 created successfully: Fix it", false)
	msg("m4", map[string]any{"type": "tool_use", "id": "toolu_c3", "name": "TaskCreate", "input": map[string]any{"subject": "Refused"}})
	result("toolu_c3", "nope", true)
	f.send(map[string]any{"type": "assistant", "parent_tool_use_id": "toolu_sub", "message": map[string]any{"id": "m9", "content": []any{
		map[string]any{"type": "tool_use", "id": "toolu_s", "name": "TaskCreate", "input": map[string]any{"subject": "A subagent's own"}}}}})
	msg("m5", map[string]any{"type": "tool_use", "id": "toolu_u1", "name": "TaskUpdate", "input": map[string]any{"taskId": "1", "status": "completed"}})
	msg("m6", map[string]any{"type": "tool_use", "id": "toolu_u2", "name": "TaskUpdate", "input": map[string]any{"taskId": "2", "status": "in_progress"}})
	f.claudeResult(false, "completed", "")
}
