package terminal

import (
	"strconv"

	"github.com/lichdotdev/lich/api"
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
// (relay.SubagentRoute): its own subagent tool, as cards, wherever the
// provider's hooks ask /subagent; the same tool kept native in a card that does
// not open cards; and lich sessions where no hook runs: Cursor CLI's chat, and
// a Claude Code plugin older than the cards.
func subagentRoute(kind, view, profileDir string, depth int, cardsOn bool) relay.SubagentRoute {
	if kind == providers.Cursor && view == api.ViewChat {
		return relay.RouteSessions
	}
	if kind == providers.Claude && !agentplugin.ClaudeRunsSubagentCards(profileDir) {
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
// a session nobody opened as a subagent, or a card above subagent.MaxDepth.
// Under a Claude Code plugin that does not tell a worker by its depth, a
// worker's own subagents stay native, since that mod would stop answering for
// a worker that opens cards.
func subagentCardsOn(kind, profileDir string, depth int) bool {
	return depth == 0 || (depth < subagent.MaxDepth && (kind != providers.Claude || agentplugin.ClaudeNestsSubagentCards(profileDir)))
}
