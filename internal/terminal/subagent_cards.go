package terminal

import (
	"strconv"

	"github.com/lichdotdev/lich/internal/agentplugin"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/subagent"
)

// subagentEnv tells a Claude Code session's lich-plugin mod how deep it runs as
// a subagent (LICH_SUBAGENT_DEPTH, 0 for a session nobody opened as one), which
// the mod's worker-answer.js reads to tell a worker. A mod older than the
// subagent hook still converts subagents itself, so a session past the depth
// cards nest to is also told to leave them native (LICH_SUBAGENT_CARDS=off,
// read by that mod's agent-cards.js); a newer one has lich decide at
// /subagent, and is told nothing.
func subagentEnv(env []string, kind string, depth int, cardsOn, asksLich bool) []string {
	if kind != providers.Claude {
		return env
	}
	env = append(env, "LICH_SUBAGENT_DEPTH="+strconv.Itoa(depth))
	if cardsOn || asksLich {
		return env
	}
	return append(env, "LICH_SUBAGENT_CARDS=off")
}

// subagentRoute is where this spawn's briefing sends the agent to fan work out
// (relay.SubagentRoute): its own Agent tool, as cards, where a plugin release
// that does that is installed; the same tool kept native in a card that does
// not open cards; and lich sessions everywhere else.
func subagentRoute(kind, profileDir string, depth int, cardsOn bool) relay.SubagentRoute {
	if kind != providers.Claude || !agentplugin.ClaudeRunsSubagentCards(profileDir) {
		return relay.RouteSessions
	}
	if cardsOn {
		return relay.RouteCards
	}
	if depth > 0 {
		return relay.RouteNative
	}
	return relay.RouteSessions
}

// subagentCardsOn is whether a session at depth turns its subagents into cards:
// a session nobody opened as a subagent, or a card above subagent.MaxDepth
// whose plugin tells a worker by its depth. Under an older plugin a worker's
// own subagents stay native, since that mod would stop answering for a worker
// that opens cards.
func subagentCardsOn(profileDir string, depth int) bool {
	return depth == 0 || (depth < subagent.MaxDepth && agentplugin.ClaudeNestsSubagentCards(profileDir))
}
