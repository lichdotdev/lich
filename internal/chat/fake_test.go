//go:build !windows

package chat

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// The provider a chat test talks to is this test binary run again with
// fakeEnv set (TestMain): it speaks Claude Code's stream-json when spawned with
// -p and ACP when spawned with acp, and answers each message by its text with
// one scripted turn, the shapes measured against the real CLIs.
const fakeEnv = "LICH_CHAT_FAKE"

// fakeLogEnv names a file the fake appends every line it reads to, for a test
// that checks what lich sent.
const fakeLogEnv = "LICH_CHAT_FAKE_LOG"

type fake struct {
	in  *bufio.Reader
	log *os.File
}

func runFake(args []string) {
	f := &fake{in: bufio.NewReader(os.Stdin)}
	if path := os.Getenv(fakeLogEnv); path != "" {
		f.log, _ = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		fmt.Fprintf(f.log, "argv %s\n", strings.Join(args, " "))
	}
	if slices.Contains(args, "acp") {
		f.acp(args)
		return
	}
	f.claude(args)
}

func (f *fake) send(v any) {
	line, _ := json.Marshal(v)
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
		case "user":
			f.send(map[string]any{"type": "system", "subtype": "init", "session_id": session, "model": run.model, "permissionMode": run.mode})
			f.claudeTurn(run, str(m, "message", "content"))
		}
	}
}

func (f *fake) claudeResult(isError bool, terminal, text string) {
	f.send(map[string]any{"type": "result", "subtype": "success", "is_error": isError, "terminal_reason": terminal, "result": text})
}

func (f *fake) claudeTurn(run *fakeClaude, text string) {
	msg := func(id string, block map[string]any) {
		f.send(map[string]any{"type": "assistant", "parent_tool_use_id": nil, "message": map[string]any{"id": id, "model": run.model, "content": []any{block}}})
	}
	switch text {
	case "hi":
		f.send(map[string]any{"type": "stream_event", "api_message_id": "m0", "event": map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "thinking_delta", "thinking": ""}}})
		msg("m0", map[string]any{"type": "thinking", "thinking": ""})
		for _, part := range []string{"O", "i", "!"} {
			f.send(map[string]any{"type": "stream_event", "api_message_id": "m1", "event": map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": part}}})
		}
		msg("m1", map[string]any{"type": "text", "text": "Oi!"})
		f.claudeResult(false, "completed", "Oi!")
	case "tool":
		msg("m2", map[string]any{"type": "thinking", "thinking": "reading"})
		msg("m2", map[string]any{"type": "tool_use", "id": "toolu_1", "name": "Edit", "input": map[string]any{"file_path": "a.txt", "old_string": "x", "new_string": "y"}})
		msg("m3", map[string]any{"type": "tool_use", "id": "toolu_sub", "name": "Read", "input": map[string]any{}})
		f.send(map[string]any{"type": "assistant", "parent_tool_use_id": "toolu_sub", "message": map[string]any{"id": "m9", "content": []any{map[string]any{"type": "text", "text": "subagent"}}}})
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": []any{map[string]any{"type": "text", "text": "edited"}}}}}})
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_sub", "is_error": true, "content": "nope"}}}})
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
	for m := f.next(); m != nil; m = f.next() {
		id, method := m["id"], str(m, "method")
		params, _ := m["params"].(map[string]any)
		switch method {
		case "initialize":
			f.result(id, map[string]any{"agentCapabilities": map[string]any{"loadSession": os.Getenv("LICH_CHAT_FAKE_NOLOAD") == ""}})
		case "session/new":
			f.result(id, opts.session(map[string]any{"sessionId": "ses-new"}))
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
	default:
		f.update(chunk("user_message_chunk", "remember caju"))
		f.update(chunk("agent_message_chunk", "ok"))
		f.update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t0", "title": "Reading a", "kind": "read", "status": "completed"})
	}
	f.result(id, opts.session(map[string]any{}))
}

func (f *fake) acpTurn(id any, text string) {
	switch text {
	case "hi":
		f.update(chunk("user_message_chunk", "hi"))
		f.update(chunk("agent_thought_chunk", "hmm"))
		for _, part := range []string{"O", "i", "!"} {
			f.update(chunk("agent_message_chunk", part))
		}
		f.update(map[string]any{"sessionUpdate": "usage_update", "used": 1})
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
