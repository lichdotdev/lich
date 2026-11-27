// Package subagent decides what becomes of a provider's own subagent call:
// lich's /subagent endpoint (docs/hooks/subagent-cards.md). A deny opens each
// task as a lich session, a card the user can watch, and hands it its task;
// an allow lets the native subagent run.
package subagent

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/relay"
)

// MaxDepth is how many levels of subagent cards lich nests: a session opens
// cards, and so does a card it opened; a card one level further keeps its own
// subagents native. Every level multiplies the cards on screen and the
// sessions a report climbs back through, and two let a worker split its own
// task once more, which is the case nesting was asked for.
const MaxDepth = 2

// budget is how long a decision may take: the hook waits 10 s, and an answer
// written after it gave up is one nobody reads (contract › Timing).
const budget = 5 * time.Second

// Asker is what a decision reads of the session asking.
type Asker struct {
	Kind  string
	Label string
	Depth int
	// CardsOff is whether the user keeps this provider's subagents inside
	// the session (Settings › Subagents).
	CardsOff bool
}

// Sessions finds the session asking.
type Sessions interface {
	Asker(id string) (Asker, bool)
}

// Spawner opens a task as a session, answering once the session exists
// rather than once its agent is up (contract › Timing), and closes one again.
type Spawner interface {
	OpenSubagentNow(ctx context.Context, opts api.OpenSubagentOptions) (api.OpenedSession, error)
	Close(ctx context.Context, opts api.CloseSessionOptions) (api.ClosedSession, error)
}

// Relay hands a session its task as a subagent errand, and says whether one
// still waits on its worker.
type Relay interface {
	HandSubagent(ctx context.Context, opts relay.SendOptions) (relay.Result, error)
	ErrandOpen(ticket string) bool
}

// Service is the decisions and what they remember: the answers given by tool
// call, the workers running by prompt, and each conversation's spawns for the
// waits that follow them.
type Service struct {
	sessions Sessions
	spawner  Spawner
	relay    Relay
	lang     func() prompt.Lang
	emit     func(api.SubagentVeto)
	budget   time.Duration

	mu      sync.Mutex
	answers map[string]api.SubagentAnswer
	running map[string][]api.SubagentOpened
	convs   map[string]*conversation
	vetoes  []api.SubagentVeto
}

// conversation is what a conversation's spawns were answered: a wait is
// denied only while every one of them was denied in time.
type conversation struct {
	denied  int
	allowed bool
}

// New is a decider over sessions, spawner and relay, writing in the prompt
// language lang gives and telling emit of every veto once it is written.
func New(sessions Sessions, spawner Spawner, rl Relay, lang func() prompt.Lang, emit func(api.SubagentVeto)) *Service {
	return &Service{
		sessions: sessions, spawner: spawner, relay: rl, lang: lang, emit: emit, budget: budget,
		answers: map[string]api.SubagentAnswer{}, running: map[string][]api.SubagentOpened{}, convs: map[string]*conversation{},
	}
}

// Vetoed is every veto written since lich started, for a window that attaches
// later.
func (s *Service) Vetoed(context.Context, api.None) ([]api.SubagentVeto, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]api.SubagentVeto{}, s.vetoes...), nil
}

// Decide answers req. commit is to be called once the answer was written, or
// failed to be: a deny that never reached the hook closes what it opened, so
// a card and a native subagent never do the same task.
func (s *Service) Decide(ctx context.Context, req api.SubagentRequest) (api.SubagentAnswer, func(written bool)) {
	asker, ok := s.sessions.Asker(req.SessionID)
	switch {
	case !ok:
		return s.allowed(req, "unknown session")
	case req.Nested:
		return s.allowed(req, "nested conversation")
	case asker.Depth >= MaxDepth:
		return s.allowed(req, "deepest card")
	}
	if req.Call == api.SubagentWait {
		return s.wait(req), func(bool) {}
	}
	if asker.CardsOff {
		return s.allowed(req, "cards off for this provider")
	}
	if req.Sealed {
		return s.denied(req, api.SubagentAnswer{Decision: api.SubagentDeny, Reason: prompt.For(s.lang()).SubagentSealed}, nil, asker)
	}
	if keepsNative(asker.Kind, req.Tasks) {
		return s.allowed(req, "type kept native")
	}
	if overLimit(req.Tasks) {
		return s.allowed(req, "prompt over the limit")
	}
	if answer, ok := s.repeat(req); ok {
		return answer, func(written bool) {
			if written {
				s.veto(req, answer.Sessions)
			}
		}
	}
	return s.open(ctx, req, asker)
}

// allowed lets the native subagent run, logging why: the hook stays silent on
// an allow, so this line is the only trace of one.
func (s *Service) allowed(req api.SubagentRequest, why string) (api.SubagentAnswer, func(bool)) {
	logAllow(req, why)
	answer := api.SubagentAnswer{Decision: api.SubagentAllow}
	return answer, func(written bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.conv(req).allowed = true
	}
}

// wait is a deny while every spawn this conversation made was denied in
// time, so no native subagent can be running; an allow otherwise, a
// conversation lich saw no spawn from included.
func (s *Service) wait(req api.SubagentRequest) api.SubagentAnswer {
	s.mu.Lock()
	c := s.conv(req)
	denied := c.denied > 0 && !c.allowed
	s.mu.Unlock()
	if !denied {
		logAllow(req, "wait on a native subagent")
		return api.SubagentAnswer{Decision: api.SubagentAllow}
	}
	return api.SubagentAnswer{Decision: api.SubagentDeny, Reason: prompt.For(s.lang()).SubagentWait}
}

// conv is req's conversation's record. Called under s.mu.
func (s *Service) conv(req api.SubagentRequest) *conversation {
	key := req.SessionID + "\x00" + req.ProviderSessionID
	c, ok := s.convs[key]
	if !ok {
		c = &conversation{}
		s.convs[key] = c
	}
	return c
}

// repeat is the answer a call already had, by its tool_use_id, or the session
// whose errand for the same prompt from the same session still waits on its
// answer: a model retrying after the veto opens nothing new.
func (s *Service) repeat(req api.SubagentRequest) (api.SubagentAnswer, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.ToolUseID != "" {
		if answer, ok := s.answers[req.SessionID+"\x00"+req.ToolUseID]; ok {
			return answer, true
		}
	}
	if len(req.Tasks) != 1 {
		return api.SubagentAnswer{}, false
	}
	opened, ok := s.running[promptKey(req.SessionID, req.Tasks[0].Prompt)]
	if !ok || len(opened) == 0 || !s.relay.ErrandOpen(opened[0].Ticket) {
		return api.SubagentAnswer{}, false
	}
	return api.SubagentAnswer{Decision: api.SubagentDeny, Reason: s.openedReason(opened), Sessions: opened}, true
}

func logAllow(req api.SubagentRequest, why string) {
	slog.Info("subagent: allow", "session", req.SessionID, "tool", req.Tool, "tool_use_id", req.ToolUseID, "why", why)
}

func promptKey(sessionID, text string) string { return sessionID + "\x00" + strings.TrimSpace(text) }
