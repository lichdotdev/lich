package main

import (
	"encoding/json"
	"math"
	"slices"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// costMisses are the named absences of a cost figure this build has a
// sentence for (internal/terminal/usage_cost.go). One from a newer backend
// reads as no reason.
var costMisses = []string{"mixed-models", "unpriced-model"}

// sessionUsage is a session's context-window occupancy and what it has cost.
// CostUSD is nil whenever the backend sent no number; CostMiss names the one
// absence the user has to know about.
type sessionUsage struct {
	Percent  int
	Tokens   int
	Window   int
	Model    string
	Effort   string
	CostUSD  *float64
	CostMiss string
	Cache    *api.SessionCache
}

// usageEvent is the session-usage payload (internal/terminal usageEvent).
// Pointers tell a missing context field apart from a zero one; the cost and
// cache fields stay raw so a malformed one reads as absent, not as a bad payload.
type usageEvent struct {
	ID       string          `json:"id"`
	Percent  *int            `json:"percent"`
	Tokens   *int            `json:"tokens"`
	Window   *int            `json:"window"`
	Model    *string         `json:"model"`
	Effort   *string         `json:"effort"`
	CostUSD  json.RawMessage `json:"costUsd"`
	CostMiss json.RawMessage `json:"costMiss"`
	Cache    json.RawMessage `json:"cache"`
}

// footerUsage keeps the last usage report of every session: retained while the
// card is off screen, cleared when the provider changes or leaves.
type footerUsage struct {
	mu    sync.Mutex
	usage map[string]sessionUsage
	// heard is the sessions /events spoke for, a report or a clear: a snapshot
	// older than that must not put back what the stream took away.
	heard map[string]bool
}

func newFooterUsage() *footerUsage {
	return &footerUsage{usage: map[string]sessionUsage{}, heard: map[string]bool{}}
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
func (f *footerUsage) apply(ev backend.Event) (bool, error) {
	var id string
	switch ev.Name {
	case api.SessionUsageEvent.Name:
		return true, f.applyUsage(ev.Data)
	case api.SessionAgentEvent.Name:
		// Every spawn announces its agent, "" included, so any agent report
		// means the prior provider's numbers no longer describe this session.
		e, err := backend.Decode(ev, api.SessionAgentEvent)
		if err != nil {
			return true, err
		}
		id = e.ID
	case api.SessionStatusEvent.Name:
		e, err := backend.Decode(ev, api.SessionStatusEvent)
		if err != nil || e.State != "idle" {
			return true, err
		}
		id = e.ID
	default:
		return false, nil
	}
	f.mu.Lock()
	delete(f.usage, id)
	f.heard[id] = true
	f.mu.Unlock()
	return true, nil
}

func (f *footerUsage) applyUsage(data json.RawMessage) error {
	id, u, ok, err := decodeUsage(data)
	if err != nil || !ok {
		return err
	}
	f.mu.Lock()
	f.usage[id] = u
	f.heard[id] = true
	f.mu.Unlock()
	return nil
}

// seed takes the readout a snapshot of the backend carries for a session the
// stream has not spoken for; one it has is newer than the snapshot.
func (f *footerUsage) seed(data json.RawMessage) error {
	id, u, ok, err := decodeUsage(data)
	if err != nil || !ok {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.heard[id] {
		f.usage[id] = u
	}
	return nil
}

// decodeUsage reads a session-usage payload. ok is false for one missing a
// context field, which is not a report.
func decodeUsage(data json.RawMessage) (id string, u sessionUsage, ok bool, err error) {
	var e usageEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return "", sessionUsage{}, false, err
	}
	if e.Percent == nil || e.Tokens == nil || e.Window == nil || e.Model == nil || e.Effort == nil {
		return "", sessionUsage{}, false, nil
	}
	u = sessionUsage{
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
	var cache *api.SessionCache
	if json.Unmarshal(e.Cache, &cache) == nil && cache != nil && cache.Read >= 0 && cache.Write >= 0 && cache.Uncached >= 0 {
		u.Cache = cache
	}
	return e.ID, u, true, nil
}
