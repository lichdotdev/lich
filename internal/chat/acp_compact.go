package chat

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// ACP's compaction updates (schema v1.34.1, stable), which an agent sends only
// to a client that advertises session.compaction: compaction_update upserts
// one by its id, compaction_summary_chunk appends to its summary. No agent
// lich runs sends them yet (opencode 1.19.4, oh-my-pi 18.8.9, Antigravity
// 1.5.14, Kiro 2.21.0, Cursor).
const (
	acpCompactInProgress = "in_progress"
	acpCompactCompleted  = "completed"
	acpCompactFailed     = "failed"
	acpCompactCancelled  = "cancelled"
)

// kiroCompaction is Kiro CLI's own compaction notice, {status: {type, error},
// summary}; its TUI reads type as started, completed or failed (2.21.0).
const kiroCompaction = "_kiro.dev/compaction/status"

// acpCompactStates maps a compaction's status onto the chat's.
var acpCompactStates = map[string]string{
	acpCompactCompleted: api.CompactionDone,
	"started":           api.CompactionRunning,
	acpCompactFailed:    api.CompactionFailed,
	acpCompactCancelled: api.CompactionCancelled,
}

// compactFor opens the compaction the agent names id, which a new id starts,
// unless one already runs: the person's /compact opened it.
func (d *acpDriver) compactFor(id string) {
	d.mu.Lock()
	known := id == d.compactID
	d.compactID = id
	d.mu.Unlock()
	if !known {
		d.c.conv.compactStart(true, d.c.counted())
	}
}

// compactionUpdate reads a compaction_update. summary and error are patches:
// absent keeps what was said, null clears it.
func (d *acpDriver) compactionUpdate(raw json.RawMessage) {
	var u struct {
		ID     string                     `json:"compactionId"`
		Status string                     `json:"status"`
		Fields map[string]json.RawMessage `json:"-"`
	}
	if json.Unmarshal(raw, &u) != nil || u.ID == "" {
		return
	}
	_ = json.Unmarshal(raw, &u.Fields)
	d.compactFor(u.ID)
	state, known := acpCompactStates[u.Status]
	if !known && u.Status != acpCompactInProgress {
		slog.Info("chat: an unknown compaction status", "session", d.c.id, "status", u.Status)
	}
	d.c.conv.compaction(func(cc *api.ChatCompaction) {
		if known {
			cc.State = state
		}
		if s, ok := u.Fields["summary"]; ok {
			cc.Summary = contentText(s)
		}
		if e, ok := u.Fields["error"]; ok {
			cc.Error = reasonText(e)
		}
	})
}

// compactionChunk reads a compaction_summary_chunk.
func (d *acpDriver) compactionChunk(raw json.RawMessage) {
	var u struct {
		ID      string     `json:"compactionId"`
		Content acpContent `json:"content"`
	}
	if json.Unmarshal(raw, &u) != nil || u.ID == "" || u.Content.Type != "text" {
		return
	}
	d.compactFor(u.ID)
	d.c.conv.compaction(func(cc *api.ChatCompaction) { cc.Summary += u.Content.Text })
}

// usageUpdate reads a usage_update: used is the tokens the context holds.
func (d *acpDriver) usageUpdate(raw json.RawMessage) {
	var u struct {
		Used int `json:"used"`
	}
	if json.Unmarshal(raw, &u) == nil && u.Used > 0 {
		d.c.reading(u.Used)
	}
}

// kiroCompactionStatus reads Kiro's notice, which names no compaction: each
// start is a new one.
func (d *acpDriver) kiroCompactionStatus(params json.RawMessage) {
	var p struct {
		Status struct {
			Type  string          `json:"type"`
			Error json.RawMessage `json:"error"`
		} `json:"status"`
		Summary json.RawMessage `json:"summary"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	state, known := acpCompactStates[p.Status.Type]
	switch {
	case !known:
		slog.Info("chat: an unknown Kiro compaction status", "session", d.c.id, "status", p.Status.Type)
		return
	case state == api.CompactionRunning:
		d.c.conv.compactStart(true, d.c.counted())
		return
	}
	d.c.conv.compaction(func(cc *api.ChatCompaction) {
		cc.State = state
		if s := contentText(p.Summary); s != "" {
			cc.Summary = s
		}
		if e := reasonText(p.Status.Error); e != "" {
			cc.Error = e
		}
	})
}

// contentText reads a summary: a string, a content block or a list of them;
// "" for null and for an empty list.
func contentText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var block acpContent
	if json.Unmarshal(raw, &block) == nil && block.Type == "text" {
		return strings.TrimSpace(block.Text)
	}
	var blocks []acpContent
	_ = json.Unmarshal(raw, &blocks)
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// reasonText reads an error: a string, or an object with its message.
func reasonText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &e)
	return strings.TrimSpace(e.Message)
}
