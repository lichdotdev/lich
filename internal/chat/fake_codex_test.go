//go:build !windows

package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The fake's Codex app-server, spawned with app-server: the shapes measured on
// 0.170.0, its own lines without "jsonrpc" as Codex writes them.

// fakeCodexModels is what the fake's model/list answers: gpt-a takes low and
// medium, gpt-b only high, and gpt-hidden is hidden.
var fakeCodexModels = []any{
	map[string]any{"id": "gpt-a", "displayName": "GPT A", "description": "fast", "hidden": false, "defaultReasoningEffort": "medium",
		"supportedReasoningEfforts": []any{map[string]any{"reasoningEffort": "low", "description": "quick"}, map[string]any{"reasoningEffort": "medium", "description": "balanced"}}},
	map[string]any{"id": "gpt-b", "displayName": "GPT B", "description": "deep", "hidden": false, "defaultReasoningEffort": "high",
		"supportedReasoningEfforts": []any{map[string]any{"reasoningEffort": "high", "description": "deep"}}},
	map[string]any{"id": "gpt-hidden", "displayName": "Hidden", "hidden": true, "defaultReasoningEffort": "low", "supportedReasoningEfforts": []any{}},
}

// codexRolloutPath is where the fake says thread id's rollout is, which the
// test writes: the date is the one fakeRollout files it under.
func codexRolloutPath(id string) string {
	return filepath.Join(os.Getenv("HOME"), ".codex", "sessions", "2026", "01", "02", "rollout-2026-01-02T03-04-05-"+id+".jsonl")
}

type fakeCodex struct {
	*fake
	thread  string
	model   string
	effort  string
	sandbox string
	turns   int
	// off are the skills skills/config/write set, by path: true when off.
	off map[string]bool
}

func (f *fake) codex() {
	c := &fakeCodex{fake: f, model: "gpt-a", effort: "medium", sandbox: "workspaceWrite"}
	for m := f.next(); m != nil; m = f.next() {
		c.serve(m)
	}
}

func (c *fakeCodex) reply(id, result any) { c.send(map[string]any{"id": id, "result": result}) }

func (c *fakeCodex) notify(method string, params map[string]any) {
	c.send(map[string]any{"method": method, "params": params})
}

// ask sends a server request and reads lich's answer, keeping a turn/steer
// that came first for after it.
func (c *fakeCodex) ask(id int, method string, params map[string]any) (map[string]any, []map[string]any) {
	c.send(map[string]any{"id": id, "method": method, "params": params})
	var before []map[string]any
	for m := c.next(); m != nil; m = c.next() {
		if str(m, "method") == "" {
			return m, before
		}
		before = append(before, m)
	}
	return nil, before
}

func (c *fakeCodex) threadAnswer(params map[string]any) map[string]any {
	if str(params, "approvalPolicy") == codexApprovalNever {
		c.sandbox = "dangerFullAccess"
	}
	if model := str(params, "model"); model != "" {
		c.model = model
	}
	return map[string]any{
		"thread": map[string]any{"id": c.thread, "path": codexRolloutPath(c.thread)},
		"model":  c.model, "reasoningEffort": c.effort, "approvalPolicy": "on-request", "sandbox": map[string]any{"type": c.sandbox},
		"instructionSources": fakeCodexSources(),
	}
}

func (c *fakeCodex) serve(m map[string]any) {
	id := m["id"]
	params, _ := m["params"].(map[string]any)
	switch str(m, "method") {
	case "initialize":
		c.reply(id, map[string]any{"userAgent": "fake", "codexHome": "/x", "platformFamily": "unix", "platformOs": "linux"})
	case "model/list":
		c.reply(id, map[string]any{"data": fakeCodexModels, "nextCursor": nil})
	case "thread/start":
		c.thread = "codex-new"
		c.reply(id, c.threadAnswer(params))
	case "thread/resume":
		if str(params, "threadId") == "gone" {
			c.send(map[string]any{"id": id, "error": map[string]any{"code": -32600, "message": "no rollout found for thread id gone"}})
			return
		}
		c.thread = str(params, "threadId")
		c.reply(id, c.threadAnswer(params))
	case "thread/turns/list":
		c.turnsPage(id, str(params, "cursor"))
	case "turn/start":
		c.start(id, params)
	case "turn/steer", "turn/interrupt":
		c.reply(id, map[string]any{})
	case "thread/shellCommand":
		c.shell(id, str(params, "command"))
	case "thread/compact/start":
		c.compact(id)
	default:
		c.serveTools(id, str(m, "method"), params)
	}
}

// turnsPage answers the history in two pages, the second turn on its own.
func (c *fakeCodex) turnsPage(id any, cursor string) {
	if cursor == "" {
		next := "p2"
		c.reply(id, map[string]any{"data": []any{map[string]any{"id": "t1", "items": []any{
			map[string]any{"type": "userMessage", "id": "u1", "content": []any{map[string]any{"type": "text", "text": "remember caju"}}},
			map[string]any{"type": "reasoning", "id": "r1", "summary": []any{"thinking"}},
			map[string]any{"type": "commandExecution", "id": "e1", "command": "ls", "status": "completed", "aggregatedOutput": "a.txt\n"},
			map[string]any{"type": "agentMessage", "id": "a1", "text": "ok"},
			map[string]any{"type": "contextCompaction", "id": "k1"},
		}}}, "nextCursor": next})
		return
	}
	c.reply(id, map[string]any{"data": []any{map[string]any{"id": "t2", "items": []any{
		map[string]any{"type": "userMessage", "id": "u2", "content": []any{map[string]any{"type": "text", "text": "and then?"}}},
		map[string]any{"type": "agentMessage", "id": "a2", "text": "second"},
	}}}, "nextCursor": nil})
}

func (c *fakeCodex) start(id any, params map[string]any) {
	c.turns++
	turn := fmt.Sprintf("turn-%d", c.turns)
	c.reply(id, map[string]any{"turn": map[string]any{"id": turn, "status": "inProgress"}})
	c.notify("turn/started", map[string]any{"threadId": c.thread, "turn": map[string]any{"id": turn}})
	if model := str(params, "model"); model != "" {
		c.model = model
	}
	if effort := str(params, "effort"); effort != "" {
		c.effort = effort
	}
	if policy, ok := params["sandboxPolicy"].(map[string]any); ok {
		c.sandbox = str(policy, "type")
	}
	c.notify("thread/settings/updated", map[string]any{"threadId": c.thread, "threadSettings": map[string]any{
		"model": c.model, "effort": c.effort, "sandboxPolicy": map[string]any{"type": c.sandbox},
	}})
	input, _ := params["input"].([]any)
	text := ""
	if len(input) > 0 {
		first, _ := input[0].(map[string]any)
		text = str(first, "text")
	}
	c.notify("item/started", map[string]any{"item": map[string]any{"type": "userMessage", "id": "echo-" + turn, "content": input}})
	status, message := c.turn(turn, text)
	done := map[string]any{"id": turn, "status": status}
	if message != "" {
		done["error"] = map[string]any{"message": message}
	}
	c.notify("turn/completed", map[string]any{"threadId": c.thread, "turn": done})
}

// shell runs the person's command as Codex does (measured on 0.170.0): the
// answer, then a turn of its own holding one commandExecution of the user's
// shell, run by the thread's shell.
func (c *fakeCodex) shell(id any, command string) {
	c.reply(id, map[string]any{})
	c.turns++
	turn := fmt.Sprintf("turn-%d", c.turns)
	c.notify("turn/started", map[string]any{"threadId": c.thread, "turn": map[string]any{"id": turn}})
	run := map[string]any{"type": "commandExecution", "id": "sh-" + turn, "command": "/bin/zsh -lc '" + command + "'", "source": "userShell", "status": "inProgress"}
	c.item("item/started", run)
	run["status"], run["aggregatedOutput"], run["exitCode"] = "completed", "ran "+command+"\n", 0
	c.item("item/completed", run)
	c.notify("turn/completed", map[string]any{"threadId": c.thread, "turn": map[string]any{"id": turn, "status": "completed"}})
}

func (c *fakeCodex) item(event string, item map[string]any) {
	c.notify(event, map[string]any{"threadId": c.thread, "item": item})
}

func (c *fakeCodex) say(id, text string) {
	c.item("item/completed", map[string]any{"type": "agentMessage", "id": id, "text": text})
}

func asJSON(v any) string {
	out, _ := json.Marshal(v)
	return string(out)
}

// turn plays the turn text asks for and returns how it ended.
func (c *fakeCodex) turn(turn, text string) (status, message string) {
	if c.codexCompactTurn(turn, text) {
		return "completed", ""
	}
	switch text {
	case "hi":
		c.item("item/started", map[string]any{"type": "reasoning", "id": "rs1", "summary": []any{}})
		for _, part := range []string{"hm", "m"} {
			c.notify("item/reasoning/summaryTextDelta", map[string]any{"itemId": "rs1", "delta": part})
		}
		c.item("item/completed", map[string]any{"type": "reasoning", "id": "rs1", "summary": []any{"hmm"}})
		c.item("item/started", map[string]any{"type": "agentMessage", "id": "msg1", "text": ""})
		for _, part := range []string{"O", "i", "!"} {
			c.notify("item/agentMessage/delta", map[string]any{"itemId": "msg1", "delta": part})
		}
		c.say("msg1", "Oi!")
	case "plan":
		c.notify("turn/plan/updated", map[string]any{"threadId": "codex-new", "turnId": turn, "explanation": nil, "plan": []any{
			map[string]any{"step": "Read the parser", "status": "completed"},
			map[string]any{"step": "Fix it", "status": "inProgress"},
			map[string]any{"step": "Test it", "status": "pending"},
		}})
	case "tool":
		c.tools()
	case "question":
		c.question()
	case "perm":
		reply, _ := c.ask(3, "item/permissions/requestApproval", map[string]any{"itemId": "p1", "reason": "reach the network",
			"permissions": map[string]any{"network": map[string]any{"enabled": true}, "fileSystem": nil}})
		c.say("perm-said", "perm:"+asJSON(reply["result"]))
	case "resolved":
		c.item("item/started", map[string]any{"type": "commandExecution", "id": "ex9", "command": "rm -rf x", "status": "inProgress"})
		c.send(map[string]any{"id": 4, "method": "item/commandExecution/requestApproval", "params": map[string]any{"itemId": "ex9", "command": "rm -rf x"}})
		c.notify("serverRequest/resolved", map[string]any{"threadId": c.thread, "requestId": 4})
		return "interrupted", ""
	case "elicit":
		reply, _ := c.ask(5, "mcpServer/elicitation/request", map[string]any{"serverName": "srv", "mode": "form", "message": "Answer", "requestedSchema": fakeElicitSchema})
		c.say("elicit-said", "elicit:"+asJSON(reply["result"]))
	case "dynamic":
		reply, _ := c.ask(6, "item/tool/call", map[string]any{"tool": "x"})
		errObj, _ := reply["error"].(map[string]any)
		c.say("dyn-said", fmt.Sprint(errObj["code"]))
	case "slow":
		c.notify("item/agentMessage/delta", map[string]any{"itemId": "slow1", "delta": "counting"})
		for m := c.next(); m != nil; m = c.next() {
			switch str(m, "method") {
			case "turn/steer":
				c.reply(m["id"], map[string]any{})
			case "turn/interrupt":
				c.reply(m["id"], map[string]any{})
				return "interrupted", ""
			}
		}
	case "fail":
		return "failed", "The model is not supported"
	case "die":
		fmt.Fprintln(os.Stderr, "codex boom")
		os.Exit(3)
	}
	return "completed", ""
}

// tools runs a command and a patch, each behind an approval, and says what
// both were answered.
func (c *fakeCodex) tools() {
	c.item("item/started", map[string]any{"type": "commandExecution", "id": "ex1", "command": "ls", "status": "inProgress"})
	first, _ := c.ask(0, "item/commandExecution/requestApproval", map[string]any{"itemId": "ex1", "command": "ls"})
	d1 := str(first, "result", "decision")
	status := map[string]string{"accept": "completed", "acceptForSession": "completed"}[d1]
	if status == "" {
		status = "declined"
	}
	c.item("item/completed", map[string]any{"type": "commandExecution", "id": "ex1", "command": "ls", "status": status, "aggregatedOutput": "a.txt\n"})
	patch := map[string]any{"type": "fileChange", "id": "fc1", "status": "inProgress", "changes": []any{
		map[string]any{"path": "a.txt", "kind": map[string]any{"type": "update", "move_path": nil}, "diff": "@@ -1 +1 @@\n-x\n+y\n"},
	}}
	c.item("item/started", patch)
	second, _ := c.ask(1, "item/fileChange/requestApproval", map[string]any{"itemId": "fc1"})
	d2 := str(second, "result", "decision")
	patch["status"] = "declined"
	c.item("item/completed", patch)
	c.say("tools-said", "decisions:"+d1+","+d2)
}

// question asks request_user_input's two questions and says the answers, or,
// on a decline, the message steered into the turn after it.
func (c *fakeCodex) question() {
	reply, before := c.ask(2, "item/tool/requestUserInput", map[string]any{"itemId": "q1", "questions": []any{
		map[string]any{"id": "fruit", "header": "Fruit", "question": "Which fruit?", "isOther": true, "isSecret": false, "options": []any{
			map[string]any{"label": "Apple", "description": "red"}, map[string]any{"label": "Pear", "description": "green"},
		}},
		map[string]any{"id": "notes", "header": "Notes", "question": "Anything else?", "isOther": false, "isSecret": false, "options": nil},
	}})
	result, _ := reply["result"].(map[string]any)
	answers, _ := result["answers"].(map[string]any)
	if len(answers) > 0 {
		c.say("q-said", "answers:"+asJSON(answers))
		return
	}
	steer := c.next()
	if len(before) > 0 {
		steer = before[0]
	}
	if steer != nil {
		c.reply(steer["id"], map[string]any{"turnId": "x"})
		params, _ := steer["params"].(map[string]any)
		input, _ := params["input"].([]any)
		first, _ := input[0].(map[string]any)
		c.say("q-said", "steered:"+str(first, "text"))
	}
}

// writeCodexRollout files thread id's rollout, with a turn_context for turn
// t1 alone, where the fake's thread/resume says it is.
func (r *rig) writeCodexRollout(id string) {
	r.t.Helper()
	path := filepath.Join(r.home, ".codex", "sessions", "2026", "01", "02", "rollout-2026-01-02T03-04-05-"+id+".jsonl")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	lines := `{"type":"session_meta","payload":{"id":"` + id + `"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"t1","model":"gpt-b","effort":"high"}}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		r.t.Fatal(err)
	}
}
