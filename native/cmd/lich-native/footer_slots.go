package main

import (
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

// ponytail: English until the native window has i18n.

// footerInput is everything FooterBar reads about what is on screen: the
// active session, or with none the project's own checkout.
type footerInput struct {
	sessionID string // "" with no active session
	provider  string // the agent live in the PTY, else the session's kind
	path      string // the session's live cwd, else its checkout
	checkout  string // where the session was spawned, which a cd does not move
	sandboxed bool
	host      string // where the shell went that the backend cannot see into
	git       lichclient.DiffStats
	isGit     bool // path is a git checkout git answered for
	pr        *lichclient.PullRequest
	usage     sessionUsage
	hasUsage  bool
	plans     []lichclient.QuotaPlan
	handsOn   int64
	// handsOnErr is why the hands-on figure could not be read.
	handsOnErr error
	cost       bool    // the cost readout is turned on
	budget     float64 // the spend ceiling, 0 for none
	now        time.Time
}

// footerSlot is one item the strip draws, with what it shows; an item whose
// data is missing gets no slot, as FooterBar renders it null.
type footerSlot struct {
	ID   string
	Text string // checkout, path, model, clock
	// Hint is the tooltip of checkout, path and model.
	Hint    string
	Readout footerReadout // context, plan, cost, handsOn
	Plan    lichclient.QuotaPlan
}

// footerSlots is the side's items that have something to show, in order.
func footerSlots(ids []string, in footerInput) []footerSlot {
	var out []footerSlot
	for _, id := range ids {
		if s, ok := footerSlotOf(id, in); ok {
			out = append(out, s)
		}
	}
	return out
}

func footerSlotOf(id string, in footerInput) (footerSlot, bool) {
	s := footerSlot{ID: id}
	switch id {
	case "attach":
		return s, true
	case "files":
		return s, in.path != ""
	case "changes":
		return s, in.isGit
	case "pr":
		return s, in.pr != nil
	case "checkout":
		s.Text = branchText(in.path, in.git.Branch)
		s.Hint = s.Text
		return s, in.path != ""
	case "path":
		s.Text, s.Hint = pathText(in.path, in.host)
		return s, in.path != ""
	case "clock":
		s.Text = clockText(in.now)
		return s, true
	}
	return sessionSlot(s, in)
}

// sessionSlot is FooterSession: the readings of the active session's
// provider, each absent until the backend has reported it.
func sessionSlot(s footerSlot, in footerInput) (footerSlot, bool) {
	var ok bool
	switch s.ID {
	case "model":
		if !in.hasUsage || in.provider == "" {
			return s, false
		}
		s.Text, s.Hint, ok = sessionModelText(in.usage)
	case "context":
		if !in.hasUsage {
			return s, false
		}
		s.Readout, ok = contextReadout(in.usage)
	case "plan":
		s.Plan, ok = planFor(in.plans, in.provider)
		if ok {
			s.Readout, ok = planReadout(s.Plan)
		}
	case "cost":
		if !in.cost || !in.hasUsage {
			return s, false
		}
		s.Readout, ok = costReadout(in.usage, in.budget)
	case "handsOn":
		if in.sessionID == "" {
			return s, false
		}
		s.Readout, ok = handsOnReadout(in.handsOn, in.handsOnErr, in.provider)
	}
	return s, ok
}

// branchText is FooterCheckout's branch display: the branch, else the
// folder's name, else the whole path.
func branchText(path, branch string) string {
	if branch != "" {
		return branch
	}
	if name := baseName(path); name != "" {
		return name
	}
	return displayPath(path)
}

// pathText is FooterCheckout's path display and its hint: the full path
// behind a home-shortened one, or the host the shell went into.
func pathText(path, host string) (text, hint string) {
	if host != "" {
		return unknownCwd(host), unknownCwd(host)
	}
	return displayPath(path), path
}
