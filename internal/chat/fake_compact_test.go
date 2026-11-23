//go:build !windows

package chat

import (
	"fmt"
	"maps"
	"os"
)

// The fake's compactions, in the lines each provider writes for one.

const fakeSummary = "This session is being continued from a previous conversation that ran out of context. Summary: the parser was fixed."

// claudeCompactTurn plays a Claude Code compaction (2.1.330's lines), and
// reports whether text asked for one.
func (f *fake) claudeCompactTurn(msg func(string, map[string]any), text string) bool {
	status := func(extra map[string]any) {
		m := map[string]any{"type": "system", "subtype": "status", "status": nil}
		maps.Copy(m, extra)
		f.send(m)
	}
	boundary := func(trigger string, pre, post int) {
		f.send(map[string]any{"type": "system", "subtype": "compact_boundary", "session_id": "claude-new",
			"compact_metadata": map[string]any{"trigger": trigger, "pre_tokens": pre, "post_tokens": post}})
	}
	user := func(content string) {
		f.send(map[string]any{"type": "user", "message": map[string]any{"content": content}})
	}
	switch text {
	case "/compact":
		status(map[string]any{"status": "compacting"})
		status(map[string]any{"status": "compacting"})
		status(map[string]any{"compact_result": "success"})
		boundary("manual", 182000, 9000)
		f.send(map[string]any{"type": "user", "is_compact_summary": true, "message": map[string]any{"content": fakeSummary}})
		user("<local-command-stdout>Compacted PreCompact [hook-a] completed successfully: {}\nPreCompact [hook-b] completed successfully</local-command-stdout>")
	case "/compact fail":
		status(map[string]any{"status": "compacting"})
		status(map[string]any{"compact_result": "failed", "compact_error": "Not enough messages to compact."})
	case "/compact quiet":
		status(map[string]any{"status": "compacting"})
		status(map[string]any{"compact_result": "failed"})
		user("<local-command-stderr>Error: Not enough messages to compact.</local-command-stderr>")
	case "/compact die":
		status(map[string]any{"status": "compacting"})
		fmt.Fprintln(os.Stderr, "compact boom")
		os.Exit(3)
	case "/compact silent":
		// neither a result nor a boundary: the turn's end finds it running
	case "auto":
		msg("m1", map[string]any{"type": "text", "text": "working"})
		status(map[string]any{"status": "compacting"})
		boundary("auto", 196000, 11000)
		status(map[string]any{"compact_result": "success"})
		msg("m2", map[string]any{"type": "text", "text": "carried on"})
	default:
		return false
	}
	f.claudeResult(false, "completed", "")
	return true
}

// codexCompact answers thread/compact/start as app-server does (measured on
// 0.170.0): a turn of its own around a contextCompaction item; signed out,
// the item never completes and the turn fails with the reason.
func (c *fakeCodex) compact(id any) {
	if c.turns == 0 {
		c.send(map[string]any{"id": id, "error": map[string]any{"code": -32600, "message": "the thread has nothing to compact"}})
		return
	}
	c.reply(id, map[string]any{})
	c.turns++
	turn := "turn-compact"
	c.notify("turn/started", map[string]any{"threadId": c.thread, "turn": map[string]any{"id": turn}})
	c.item("item/started", map[string]any{"type": "contextCompaction", "id": "cc1"})
	if c.signedOut {
		c.notify("error", map[string]any{"error": map[string]any{"message": "Reconnecting... 1/2"}})
		c.notify("turn/completed", map[string]any{"threadId": c.thread, "turn": map[string]any{"id": turn, "status": "failed",
			"error": map[string]any{"message": "Error running remote compact task: unexpected status 401 Unauthorized"}}})
		return
	}
	c.item("item/completed", map[string]any{"type": "contextCompaction", "id": "cc1"})
	c.usage(turn, 9000)
	c.notify("thread/compacted", map[string]any{"threadId": c.thread, "turnId": turn})
	c.notify("turn/completed", map[string]any{"threadId": c.thread, "turn": map[string]any{"id": turn, "status": "completed"}})
}

func (c *fakeCodex) usage(turn string, total int) {
	c.notify("thread/tokenUsage/updated", map[string]any{"threadId": c.thread, "turnId": turn, "tokenUsage": map[string]any{
		"last": map[string]any{"totalTokens": total}, "total": map[string]any{"totalTokens": total}}})
}

// codexCompactTurn plays a turn of text that compacts, false for any other.
func (c *fakeCodex) codexCompactTurn(turn, text string) bool {
	switch text {
	case "big":
		c.usage(turn, 182000)
		c.say("big-"+turn, "lots")
	case "sign out":
		c.signedOut = true
	case "autocompact":
		c.say("pre-"+turn, "working")
		c.item("item/started", map[string]any{"type": "contextCompaction", "id": "cc2"})
		c.item("item/completed", map[string]any{"type": "contextCompaction", "id": "cc2"})
		c.say("post-"+turn, "carried on")
	default:
		return false
	}
	return true
}

// acpCompactTurn plays an ACP agent's compaction, and reports whether text
// asked for one.
func (f *fake) acpCompactTurn(id any, text string) bool {
	switch text {
	case "big":
		f.update(map[string]any{"sessionUpdate": "usage_update", "used": 182000, "size": 200000})
		f.update(chunk("agent_message_chunk", "lots"))
	case "/compact":
		// opencode: nothing says it compacts, the usage after it does
		f.update(chunk("agent_message_chunk", "Compaction complete."))
		f.update(map[string]any{"sessionUpdate": "usage_update", "used": 9000, "size": 200000})
	case "spec":
		f.update(map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "working"}})
		f.update(map[string]any{"sessionUpdate": "compaction_update", "compactionId": "c1", "status": "in_progress"})
		f.update(map[string]any{"sessionUpdate": "compaction_summary_chunk", "compactionId": "c1", "content": map[string]any{"type": "text", "text": "Summary "}})
		f.update(map[string]any{"sessionUpdate": "compaction_summary_chunk", "compactionId": "c1", "content": map[string]any{"type": "text", "text": "so far"}})
		f.update(map[string]any{"sessionUpdate": "compaction_update", "compactionId": "c1", "status": "completed"})
		f.update(map[string]any{"sessionUpdate": "compaction_update", "compactionId": "c2", "status": "failed", "error": "the model refused", "summary": nil})
		f.update(map[string]any{"sessionUpdate": "compaction_update", "compactionId": "c3", "status": "_vendor_paused"})
	case "kiro":
		kiro := func(params map[string]any) {
			f.send(map[string]any{"jsonrpc": "2.0", "method": kiroCompaction, "params": params})
		}
		kiro(map[string]any{"status": map[string]any{"type": "started"}})
		kiro(map[string]any{"status": map[string]any{"type": "completed"}, "summary": "kiro summary"})
		kiro(map[string]any{"status": map[string]any{"type": "started"}})
		kiro(map[string]any{"status": map[string]any{"type": "failed", "error": "context overflow"}})
		kiro(map[string]any{"status": map[string]any{"type": "paused"}})
	default:
		return false
	}
	f.result(id, map[string]any{"stopReason": "end_turn"})
	return true
}
