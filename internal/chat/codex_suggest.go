package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/lichdotdev/lich/internal/prompt"
)

// codexFork is the suggestion being made: a fork of the thread that copies
// none of its turns into a rollout, asked the suggestion prompt once. Its
// notifications never reach the chat (codexDriver.notification).
type codexFork struct {
	// busy holds one fork per session; after is the turn whose end it answers.
	busy   bool
	after  int
	thread string
	turn   string
	// dropped is set by a turn the person began meanwhile.
	dropped bool
	reply   strings.Builder
	tool    bool
}

var errNoFork = errors.New("Codex forked no thread")

// codexForkParams are thread/fork's: ephemeral (no rollout, not in
// thread/list), and with no tool that runs anything, which read-only alone does
// not stop (measured on 0.170.0: the model ran ls).
func codexForkParams(thread, model string) map[string]any {
	params := map[string]any{
		"threadId": thread, "ephemeral": true, "excludeTurns": true,
		"sandbox": "read-only", "approvalPolicy": codexApprovalNever,
		"config": map[string]any{"features.shell_tool": false, "features.unified_exec": false},
	}
	if model != "" {
		params["model"] = model
	}
	return params
}

// suggestAfter makes the suggestion for turn's end, unless one is already
// being made. It calls Codex, so it runs off the reader.
func (d *codexDriver) suggestAfter(turn int) {
	d.mu.Lock()
	// a turn/completed lich began no turn for leaves the last offer standing
	if d.fork.busy || d.forked == turn {
		d.mu.Unlock()
		return
	}
	d.fork, d.forked = codexFork{busy: true, after: turn}, turn
	// the model that answered last keeps the prompt cache warm
	model := d.run.on.Model
	if model == "" {
		model = d.run.model
	}
	thread := d.thread
	d.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	raw, err := d.call(ctx, "thread/fork", codexForkParams(thread, model))
	var t codexThread
	if err == nil && (json.Unmarshal(raw, &t) != nil || t.Thread.ID == "") {
		err = errNoFork
	}
	if err != nil {
		slog.Debug("chat: fork Codex's thread for a suggestion", "session", d.c.id, "err", err)
		d.endFork("")
		return
	}
	d.mu.Lock()
	d.fork.thread = t.Thread.ID
	dropped := d.fork.dropped
	d.mu.Unlock()
	if dropped {
		d.endFork(t.Thread.ID)
		return
	}
	params := map[string]any{"threadId": t.Thread.ID, "input": textInput(prompt.For(d.c.svc.lang()).ChatSuggestPrompt), "effort": "low"}
	if model != "" {
		params["model"] = model
	}
	raw, err = d.call(ctx, "turn/start", params)
	if err != nil {
		slog.Debug("chat: ask Codex for a suggestion", "session", d.c.id, "err", err)
		d.endFork(t.Thread.ID)
		return
	}
	var started codexTurnEvent
	_ = json.Unmarshal(raw, &started)
	d.mu.Lock()
	d.fork.turn = started.Turn.ID
	dropped = d.fork.dropped
	d.mu.Unlock()
	if dropped {
		d.interruptFork()
	}
}

// dropFork stops the suggestion being made: the person began a turn.
func (d *codexDriver) dropFork() {
	d.mu.Lock()
	busy := d.fork.busy && !d.fork.dropped
	d.fork.dropped = true
	d.mu.Unlock()
	if busy {
		go d.interruptFork()
	}
}

// interruptFork interrupts the fork's turn once it has an id; one without it
// yet is interrupted by suggestAfter when turn/start answers.
func (d *codexDriver) interruptFork() {
	d.mu.Lock()
	thread, turn := d.fork.thread, d.fork.turn
	d.mu.Unlock()
	if thread == "" || turn == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	if _, err := d.call(ctx, "turn/interrupt", map[string]any{"threadId": thread, "turnId": turn}); err != nil {
		slog.Debug("chat: interrupt Codex's suggestion", "session", d.c.id, "err", err)
	}
}

// otherThread is the thread params belong to when it is not the session's:
// the fork's notifications go to forkNotification, and no other thread's
// reaches the chat. A notification naming no thread is the session's.
func (d *codexDriver) otherThread(params json.RawMessage) (string, bool) {
	var p struct {
		ThreadID string `json:"threadId"`
		Thread   struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(params, &p)
	id := p.ThreadID
	if id == "" {
		id = p.Thread.ID
	}
	main := d.threadID()
	return id, id != "" && main != "" && id != main
}

// forkNotification reads the fork's answer: its messages, and whether it
// tried a tool, which makes no suggestion. Another thread's is dropped.
func (d *codexDriver) forkNotification(thread, method string, params json.RawMessage) {
	d.mu.Lock()
	ours := thread == d.fork.thread
	d.mu.Unlock()
	if !ours {
		return
	}
	switch method {
	case "item/completed":
		var e codexItemEvent
		if json.Unmarshal(params, &e) != nil {
			return
		}
		d.mu.Lock()
		switch e.Item.Type {
		case "agentMessage":
			d.fork.reply.WriteString(e.Item.Text)
		case "userMessage", "reasoning":
		default:
			d.fork.tool = true
		}
		d.mu.Unlock()
	case "turn/completed":
		var e codexTurnEvent
		_ = json.Unmarshal(params, &e)
		d.mu.Lock()
		f := &d.fork
		text, after, thread := "", f.after, f.thread
		if e.Turn.Status == "completed" && !f.tool && !f.dropped {
			text = cleanSuggestion(f.reply.String())
		}
		d.mu.Unlock()
		d.c.suggest(after, text)
		go d.endFork(thread)
	}
}

// endFork lets the next suggestion be made, and lets Codex unload the fork:
// 0.170.0 has no thread/close, refuses archive and delete for an ephemeral
// thread, and unloads one about a minute after its last subscriber left, its
// MCP servers exiting with it (measured: 62s).
func (d *codexDriver) endFork(thread string) {
	d.mu.Lock()
	d.fork = codexFork{}
	d.mu.Unlock()
	if thread == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	if _, err := d.call(ctx, "thread/unsubscribe", map[string]any{"threadId": thread}); err != nil {
		slog.Debug("chat: unload Codex's suggestion fork", "session", d.c.id, "err", err)
	}
}
