package terminal

import (
	"sync"

	"github.com/lichdotdev/lich/internal/quota"
)

// ChatUsage is what a Claude Code chat's stream-json says about its own
// conversation: the context window its model runs with, the conversation's
// whole cost so far, and the account's rate limits. Window and CostUSD belong
// to Conversation and are applied to no other; a report without one carries
// rate limits alone. Zero and nil mean not said.
type ChatUsage struct {
	Conversation string
	Window       int
	CostUSD      *float64
	RateLimits   []quota.ReportedLimit
}

// ReportChatUsage keeps a chat's figures for the footer's readout, passes its
// rate limits on to the plan gauge and pushes the readout again.
func (s *Service) ReportChatUsage(id string, u ChatUsage) {
	if u.Conversation != "" {
		s.usageReports.set(id, u)
	}
	s.rateLimitsMu.Lock()
	fn := s.rateLimitReports
	s.rateLimitsMu.Unlock()
	if fn != nil && len(u.RateLimits) > 0 {
		fn(id, u.RateLimits)
	}
	go s.emitUsage(id)
}

// usageReports is the latest report of each session. A session keeps one: a
// report carries the conversation's whole figures, never a delta.
type usageReports struct {
	mu   sync.Mutex
	byID map[string]ChatUsage
}

func (u *usageReports) set(id string, r ChatUsage) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.byID == nil {
		u.byID = map[string]ChatUsage{}
	}
	u.byID[id] = r
}

// of is session id's latest report, if it is about conversation.
func (u *usageReports) of(id, conversation string) (ChatUsage, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	r, ok := u.byID[id]
	return r, ok && r.Conversation == conversation
}

func (u *usageReports) forget(id string) {
	u.mu.Lock()
	delete(u.byID, id)
	u.mu.Unlock()
}

// SetRateLimitReports wires where the rate limits a session reports go: the
// plan gauge's reader. Startup wiring, called once before any session reports.
func (s *Service) SetRateLimitReports(fn func(sessionID string, limits []quota.ReportedLimit)) {
	s.rateLimitsMu.Lock()
	s.rateLimitReports = fn
	s.rateLimitsMu.Unlock()
}

// reportedContextUsage reads what the transcript says about a conversation's
// context against the window Claude Code reported, which no transcript
// records. The tokens stay the transcript's, which moves mid-turn.
func reportedContextUsage(u contextUsage, window int) contextUsage {
	if window <= 0 {
		return u
	}
	u.window = window
	u.percent = min(u.tokens*100/u.window, 100)
	return u
}
