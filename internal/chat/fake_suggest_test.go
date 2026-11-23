//go:build !windows

package chat

import "strings"

// fork answers thread/fork for the turns named suggest…, the only ones whose
// end the fake makes a suggestion for; any other is refused, so a test that
// does not ask for one sees no fork turn. The fork announces itself and its
// settings the way Codex does, on its own thread.
func (c *fakeCodex) fork(id any, params map[string]any) {
	if !strings.HasPrefix(c.last, "suggest") {
		c.send(map[string]any{"id": id, "error": map[string]any{"code": -32600, "message": "the fake forks no " + c.last}})
		return
	}
	c.forked = "codex-fork-" + c.last
	c.reply(id, map[string]any{"thread": map[string]any{"id": c.forked}, "model": str(params, "model"), "reasoningEffort": "low"})
	c.notify("thread/started", map[string]any{"thread": map[string]any{"id": c.forked}})
}

// forkStart plays the fork's turn as c.last asks: a wrapped, labelled reply,
// a tool call, or a turn that waits for its interrupt and then answers late.
func (c *fakeCodex) forkStart(id any) {
	turn := "fork-turn"
	c.reply(id, map[string]any{"turn": map[string]any{"id": turn, "status": "inProgress"}})
	on := func(method string, params map[string]any) {
		params["threadId"] = c.forked
		c.notify(method, params)
	}
	on("turn/started", map[string]any{"turn": map[string]any{"id": turn}})
	// a thread that is neither the session's nor the fork's says its own
	// answer first, which lich must not take for the fork's
	c.notify("item/completed", map[string]any{"threadId": "codex-stray", "item": map[string]any{"type": "agentMessage", "id": "sx1", "text": "stray"}})
	c.notify("turn/completed", map[string]any{"threadId": "codex-stray", "turn": map[string]any{"id": "stray-turn", "status": "completed"}})
	on("thread/settings/updated", map[string]any{"threadSettings": map[string]any{"model": "gpt-b", "effort": "low", "sandboxPolicy": map[string]any{"type": "readOnly"}}})
	switch c.last {
	case "suggest-slow":
		c.forkTurn = turn
		return
	case "suggest-tool":
		on("item/completed", map[string]any{"item": map[string]any{"type": "commandExecution", "id": "fx1", "command": "ls", "status": "completed"}})
	default:
		on("item/completed", map[string]any{"item": map[string]any{"type": "agentMessage", "id": "fm1", "text": "<suggestion>Suggestion: run the tests</suggestion>"}})
	}
	on("turn/completed", map[string]any{"turn": map[string]any{"id": turn, "status": "completed"}})
}

// forkInterrupt answers the fork's interrupt with a reply that raced it,
// which lich must not show.
func (c *fakeCodex) forkInterrupt(id any) {
	c.reply(id, map[string]any{})
	if c.forkTurn == "" {
		return
	}
	turn := c.forkTurn
	c.forkTurn = ""
	c.notify("item/completed", map[string]any{"threadId": c.forked, "item": map[string]any{"type": "agentMessage", "id": "fm2", "text": "too late"}})
	c.notify("turn/completed", map[string]any{"threadId": c.forked, "turn": map[string]any{"id": turn, "status": "completed"}})
}
