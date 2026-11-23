package chat

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/lichdotdev/lich/internal/quota"
	"github.com/lichdotdev/lich/internal/terminal"
)

// What Claude Code's stream-json says about its own spend (2.1.330): a result
// carries the conversation's whole cost so far, carried over a resume, and the
// context window of each model the turn ran; a rate_limit_event, once per
// process, the plan's windows as the fraction used and a reset in Unix seconds.
type claudeResultUsage struct {
	SessionID string                      `json:"session_id"`
	Cost      *float64                    `json:"total_cost_usd"`
	Models    map[string]claudeModelUsage `json:"modelUsage"`
}

type claudeModelUsage struct {
	ContextWindow int `json:"contextWindow"`
}

type claudeRateLimits struct {
	Info struct {
		Windows map[string]struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    int64   `json:"resetsAt"`
		} `json:"unifiedWindows"`
	} `json:"rate_limit_info"`
}

// reportUsage hands a result's cost and window to the footer's readout.
func (d *claudeDriver) reportUsage(raw []byte) {
	var r claudeResultUsage
	if json.Unmarshal(raw, &r) != nil || r.SessionID == "" {
		return
	}
	d.mu.Lock()
	model := d.run.model
	d.mu.Unlock()
	d.c.svc.term.ReportChatUsage(d.c.id, terminal.ChatUsage{
		Conversation: r.SessionID, Window: contextWindow(r.Models, model), CostUSD: r.Cost,
	})
}

// contextWindow is the window of the model the session runs: modelUsage also
// lists the models its subagents and its own housekeeping ran, under their
// full names, where a 1M variant may be written with its [1m] suffix or
// without. 0 when none is the session's.
func contextWindow(models map[string]claudeModelUsage, model string) int {
	base, _, _ := strings.Cut(model, "[")
	for _, name := range []string{model, base} {
		if m, ok := models[name]; ok {
			return m.ContextWindow
		}
	}
	if len(models) == 1 {
		for _, m := range models {
			return m.ContextWindow
		}
	}
	return 0
}

// reportRateLimits hands the plan's windows to the gauge.
func (d *claudeDriver) reportRateLimits(raw []byte) {
	var r claudeRateLimits
	if json.Unmarshal(raw, &r) != nil {
		return
	}
	var limits []quota.ReportedLimit
	for _, kind := range slices.Sorted(maps.Keys(r.Info.Windows)) {
		w := r.Info.Windows[kind]
		limit := quota.ReportedLimit{Kind: kind, PercentUsed: w.Utilization * 100}
		if w.ResetsAt > 0 {
			limit.ResetsAt = time.Unix(w.ResetsAt, 0).UTC().Format(time.RFC3339)
		}
		limits = append(limits, limit)
	}
	if len(limits) > 0 {
		d.c.svc.term.ReportChatUsage(d.c.id, terminal.ChatUsage{RateLimits: limits})
	}
}
