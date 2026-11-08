package relay

import (
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/internal/prompt"
)

// Thresholds of the ping-pong breaker, taken from clowder-ai's F167: a pair of
// sessions trading messages with no work between them is warned at the second
// idle hop and refused at the fourth.
const (
	pingPongWarnAt  = 2
	pingPongBreakAt = 4
)

// relayTools are lich's own messaging tools: calling them is talking about the
// work, not doing it. Matched as a suffix because every provider spells the
// MCP prefix its own way (docs/hooks/session-state.md › The tool a turn is
// running), and Antigravity puts `lich/<tool>` in the detail instead.
var relayTools = []string{ToolSend, ToolReply, ToolCollect, ToolList}

// turnTools is what one session's open turn has reported running.
type turnTools struct {
	// any is whether a tool was reported at all. A provider that reports none
	// (Crush) gives no evidence of an idle turn, so its hops never count.
	any bool
	// work is whether one of them was something other than a relay tool.
	work bool
}

// sessionPair is two sessions in either order, so A→B and B→A share a streak.
type sessionPair [2]string

func pairOf(a, b string) sessionPair {
	if a > b {
		a, b = b, a
	}
	return sessionPair{a, b}
}

// ObserveTool takes the tool a session's PreToolUse reported, for the breaker
// to tell a turn that worked from one that only messaged.
func (s *Service) ObserveTool(sessionID, tool, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tt := s.turnTools[sessionID]
	tt.any = true
	tt.work = tt.work || !isRelayTool(tool, detail)
	s.turnTools[sessionID] = tt
}

func isRelayTool(tool, detail string) bool {
	for _, name := range relayTools {
		if strings.HasSuffix(tool, name) || strings.HasSuffix(detail, MCPServerName+"/"+name) {
			return true
		}
	}
	return false
}

// UserTyped ends every streak a session is part of: a person sending a line
// there has stepped into the exchange.
func (s *Service) UserTyped(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropStreaksLocked(sessionID, sessionPair{})
}

// dropStreaksLocked forgets every streak sessionID is part of except keep.
// Called under s.mu.
func (s *Service) dropStreaksLocked(sessionID string, keep sessionPair) {
	for pair := range s.streaks {
		if pair != keep && (pair[0] == sessionID || pair[1] == sessionID) {
			delete(s.streaks, pair)
		}
	}
}

// noteHop counts one message from fromID to toID and answers the streak it
// makes: the idle hops in a row between the two. A hop from a turn that worked
// starts the count over, and so does either of them messaging a third session.
// A hop that would reach pingPongBreakAt is not counted, since it will not be
// delivered.
func (s *Service) noteHop(fromID, toID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	pair := pairOf(fromID, toID)
	s.dropStreaksLocked(fromID, pair)
	s.dropStreaksLocked(toID, pair)
	tt := s.turnTools[fromID]
	if !tt.any || tt.work {
		delete(s.streaks, pair)
		return 0
	}
	streak := s.streaks[pair] + 1
	if streak < pingPongBreakAt {
		s.streaks[pair] = streak
	}
	return streak
}

// checkPingPong is the breaker in front of a send: the warning to append to a
// message that continues a streak, or the error refusing one that ends it.
func (s *Service) checkPingPong(fromID, toID, target string) (string, error) {
	if fromID == "" || fromID == toID {
		return "", nil
	}
	streak := s.noteHop(fromID, toID)
	if streak >= pingPongBreakAt {
		return "", fmt.Errorf("not delivered: this is message %d in a row between this session and %q "+
			"with no work done between them, which looks like a loop. Do the work, or end your turn "+
			"and leave the exchange to the user", streak, target)
	}
	if streak >= pingPongWarnAt {
		return "\n\n" + prompt.For(s.lang()).PingPongWarning, nil
	}
	return "", nil
}
