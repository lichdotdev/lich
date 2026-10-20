package main

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/lichdotdev/lich/native/i18n"
)

// costReadoutKey is internal/store's costReadoutKey: the global flag that
// turns the per-session cost readout on, and with it transcript pricing.
const costReadoutKey = "usage.cost"

// costMissReasonKeys are the catalog keys of why a session has no cost,
// keyed by the reason the backend names.
var costMissReasonKeys = map[string]string{
	"mixed-models":   "session.cost.mixedModels",
	"unpriced-model": "session.cost.unpricedModel",
}

// costGate is the window's copy of the cost-readout flag.
// Off until loaded and on a failed load: nobody should see a money figure
// appear and vanish.
type costGate struct {
	get func(ctx context.Context, key, scope string) (string, error)
	set func(ctx context.Context, key, scope, value string) error
	// onChange runs, outside the lock, whenever enabled flips.
	onChange func()

	mu       sync.Mutex
	enabled  bool
	ready    bool
	revision int
}

// load reads the stored flag. A toggle made while it was in flight wins over
// what it read. ready turns true either way, so a layout edit waiting on the
// old choice can go ahead.
func (g *costGate) load(ctx context.Context) error {
	g.mu.Lock()
	startedAt := g.revision
	g.mu.Unlock()
	value, err := g.get(ctx, costReadoutKey, "")
	g.mu.Lock()
	g.ready = true
	changed := err == nil && startedAt == g.revision && (value == "true") != g.enabled
	if changed {
		g.enabled = value == "true"
	}
	g.mu.Unlock()
	if changed {
		g.onChange()
	}
	return err
}

func (g *costGate) on() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.enabled
}

func (g *costGate) isReady() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ready
}

// setOn flips the flag at once and writes it through; a failed write puts the
// previous value back unless another toggle has come since.
func (g *costGate) setOn(ctx context.Context, on bool) error {
	g.mu.Lock()
	previous := g.enabled
	g.revision++
	mine := g.revision
	g.enabled = on
	g.mu.Unlock()
	if on != previous {
		g.onChange()
	}
	err := g.set(ctx, costReadoutKey, "", fmt.Sprint(on))
	if err == nil {
		return nil
	}
	g.mu.Lock()
	rollback := g.revision == mine && g.enabled != previous
	if rollback {
		g.enabled = previous
	}
	g.mu.Unlock()
	if rollback {
		g.onChange()
	}
	return err
}

// formatCost is a session's spend at footer size. The precision follows the
// number: a one-question session costs fractions of a cent, so three decimals
// below a dollar, and a floor of "<$0.001" rather than zeros that read as free.
func formatCost(usd float64) string {
	switch {
	case math.IsNaN(usd) || math.IsInf(usd, 0) || usd < 0:
		return ""
	case usd == 0:
		return "$0"
	case usd < 0.001:
		return "<$0.001"
	case usd >= 1:
		return fmt.Sprintf("$%.2f", usd)
	}
	return fmt.Sprintf("$%.3f", usd)
}

// budgetShare is how much of a spend ceiling is gone, 0–100, and 0 with no
// ceiling so a user who set none sees the readout uncoloured.
func budgetShare(usd, budget float64) float64 {
	finite := !math.IsNaN(usd) && !math.IsInf(usd, 0) && !math.IsNaN(budget) && !math.IsInf(budget, 0)
	if !finite || usd <= 0 || budget <= 0 {
		return 0
	}
	return math.Min(usd/budget*100, 100)
}

// costReadout is the readout of a session's cost. False when there is neither a
// figure nor a named reason for its absence.
func costReadout(u sessionUsage, budget float64) (footerReadout, bool) {
	if u.CostUSD == nil && u.CostMiss == "" {
		return footerReadout{}, false
	}
	if u.CostUSD == nil {
		return footerReadout{
			Label: i18n.T("shell.footerSession.costUnavailable"), Text: "$—",
			Detail: []string{i18n.T(costMissReasonKeys[u.CostMiss])},
		}, true
	}
	detail := []string{i18n.T("shell.footerSession.costNote")}
	if budget > 0 {
		line := i18n.T("shell.footerSession.costOfBudget", i18n.Args{"cost": formatCost(*u.CostUSD), "budget": formatCost(budget)})
		detail = append([]string{line}, detail...)
	}
	return footerReadout{
		Label: i18n.T("shell.footerSession.sessionCost"), Text: formatCost(*u.CostUSD),
		Detail: detail, Tone: toneOf(budgetShare(*u.CostUSD, budget)),
	}, true
}
