package relay

import "fmt"

// maxChainDepth is how many relayed messages deep a chain may run before a
// send is refused, taken from agent-teams-ai's cascade guard
// (agent-teams-controller/src/internal/cascadeGuard.js, MAX_CHAIN_DEPTH). It
// catches the loop through three or more sessions the ping-pong breaker, which
// watches one pair, never sees.
const maxChainDepth = 5

// chainDepthLocked is the depth of a message fromID sends now: one past the
// deepest relayed task it still owes an answer to, 0 when it owes none, which
// is a turn its user started. A turn that worked starts over at 0 too, on the
// ping-pong breaker's rule: a review loop passes every message on from a task,
// and counting those refused it by its fifth round. Called under s.mu.
func (s *Service) chainDepthLocked(fromID string) int {
	if tt := s.turnTools[fromID]; !tt.any || tt.work {
		return 0
	}
	depth := 0
	for _, t := range s.tickets {
		if t.targetID == fromID && t.submitted {
			depth = max(depth, t.depth+1)
		}
	}
	return depth
}

func chainTooDeep(depth int, target string) error {
	return fmt.Errorf("not delivered to %q: this message would be %d relayed messages deep, "+
		"each sent while working on the one before, which looks like a loop through several "+
		"sessions. Answer the task you were given instead of passing it on", target, depth)
}
