package chat

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// Codex compacts as a turn of its own, with a contextCompaction item started
// and completed in it; a failed one never completes the item, and its turn
// fails with the reason (measured on 0.170.0). Its TUI's /compact is the client's
// thread/compact/start, which app-server lists nowhere: lich routes the typed
// command itself.
const (
	codexCompactItem    = "contextCompaction"
	codexCompactCommand = "/compact"
)

// codexCompactTyped reports whether m is the person's /compact, which takes no
// arguments in Codex's TUI.
func codexCompactTyped(m message) bool {
	return strings.TrimSpace(m.text) == codexCompactCommand && len(m.images) == 0
}

// compact asks Codex to compact the thread, the turn it starts ending through
// turn/completed like any other.
func (d *codexDriver) compact() error {
	d.c.conv.compactStart(false, d.c.counted())
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	_, err := d.call(ctx, "thread/compact/start", map[string]any{"threadId": d.threadID()})
	return err
}

// compactItem reads a contextCompaction item, live: one that starts where the
// person typed no /compact is Codex's own.
func (d *codexDriver) compactItem(done bool) {
	if !done {
		d.c.conv.compactStart(true, d.c.counted())
		return
	}
	d.c.conv.compaction(func(cc *api.ChatCompaction) { cc.State = api.CompactionDone })
}

// tokenUsage reads thread/tokenUsage/updated: the last request's total is
// what the conversation holds, as the rollout's token_count says
// (terminal.scanCodexContextUsage).
func (d *codexDriver) tokenUsage(params json.RawMessage) {
	var p struct {
		Usage struct {
			Last struct {
				Total int `json:"totalTokens"`
			} `json:"last"`
		} `json:"tokenUsage"`
	}
	if json.Unmarshal(params, &p) == nil && p.Usage.Last.Total > 0 {
		d.c.reading(p.Usage.Last.Total)
	}
}

// replayCompaction draws a compaction read back: in a turn the person opened,
// Codex compacted on its own; in one with no message of theirs, it was their
// /compact, which the turn does not hold.
func (d *codexDriver) replayCompaction(turn codexTurn) {
	asked := slices.ContainsFunc(turn.Items, func(it codexItem) bool { return it.Type == "userMessage" })
	if !asked {
		d.c.conv.userMessage(codexCompactCommand, nil)
	}
	d.c.conv.compactOpen(func(cc *api.ChatCompaction) { cc.State, cc.Auto = api.CompactionDone, asked })
	d.c.conv.compactClose(api.CompactionDone)
}
