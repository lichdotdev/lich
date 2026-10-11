package main

import (
	"encoding/json"
	"math"
	"slices"
	"sync"

	"github.com/omartelo/lich/native/lichclient"
)

// costMisses are the named absences of a cost figure this build has a
// sentence for (internal/terminal/usage_cost.go). One from a newer backend
// reads as no reason.
var costMisses = []string{"mixed-models", "unpriced-model"}

// sessionUsage is a session's context-window occupancy and what it has cost
// (session-events.ts SessionUsage). CostUSD is nil whenever the backend sent
// no number; CostMiss names the one absence the user has to know about.
type sessionUsage struct {
	Percent  int
	Tokens   int
	Window   int
	Model    string
	Effort   string
	CostUSD  *float64
	CostMiss string
}

// usageEvent is the session-usage payload (internal/terminal usageEvent).
// Pointers tell a missing context field apart from a zero one; the cost
// fields stay raw so a malformed one reads as absent, not as a bad payload.
type usageEvent struct {
	ID       string          `json:"id"`
	Percent  *int            `json:"percent"`
	Tokens   *int            `json:"tokens"`
	Window   *int            `json:"window"`
	Model    *string         `json:"model"`
	Effort   *string         `json:"effort"`
	CostUSD  json.RawMessage `json:"costUsd"`
	CostMiss json.RawMessage `json:"costMiss"`
}

// footerUsage keeps the last usage report of every session, ported from
// session-usage-store.ts: retained while the card is off screen, cleared when
// the provider changes or leaves.
type footerUsage struct {
	mu    sync.Mutex
	usage map[string]sessionUsage
}

func newFooterUsage() *footerUsage {
	return &footerUsage{usage: map[string]sessionUsage{}}
}

// get is session id's last report; false until the backend sends one.
func (f *footerUsage) get(id string) (sessionUsage, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.usage[id]
	return u, ok
}

// apply takes session-usage reports, and the session-agent and idle
// session-status events that void the prior provider's report. It reports
// whether ev was one of the three; session-agent and session-status are not
// consumed, so the caller still hands them to the card stores.
func (f *footerUsage) apply(ev lichclient.Event) (bool, error) {
	var e struct {
		ID    string  `json:"id"`
		Agent *string `json:"agent"`
		State string  `json:"state"`
	}
	if ev.Name == "session-usage" {
		return true, f.applyUsage(ev.Data)
	}
	if ev.Name != "session-agent" && ev.Name != statusEventName {
		return false, nil
	}
	if err := json.Unmarshal(ev.Data, &e); err != nil {
		return true, err
	}
	// Every spawn announces its agent, "" included, so any agent report means
	// the prior provider's numbers no longer describe this session.
	agentChanged := ev.Name == "session-agent" && e.Agent != nil
	providerLeft := ev.Name == statusEventName && e.State == "idle"
	if agentChanged || providerLeft {
		f.mu.Lock()
		delete(f.usage, e.ID)
		f.mu.Unlock()
	}
	return true, nil
}

func (f *footerUsage) applyUsage(data json.RawMessage) error {
	var e usageEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if e.Percent == nil || e.Tokens == nil || e.Window == nil || e.Model == nil || e.Effort == nil {
		return nil
	}
	u := sessionUsage{
		Percent: *e.Percent, Tokens: *e.Tokens, Window: *e.Window,
		Model: *e.Model, Effort: *e.Effort,
	}
	var cost *float64
	if json.Unmarshal(e.CostUSD, &cost) == nil && cost != nil && !math.IsInf(*cost, 0) && !math.IsNaN(*cost) {
		u.CostUSD = cost
	}
	var miss string
	if json.Unmarshal(e.CostMiss, &miss) == nil && slices.Contains(costMisses, miss) {
		u.CostMiss = miss
	}
	f.mu.Lock()
	f.usage[e.ID] = u
	f.mu.Unlock()
	return nil
}
