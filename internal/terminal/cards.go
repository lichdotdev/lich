package terminal

import (
	"encoding/json"
	"log/slog"
	"maps"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/library"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/subagent"
)

// A subagent card starts with lich and the project's own instructions, and of
// the user's own setup only what Settings › Subagents has it bring
// (docs/hooks/subagent-cards.md, What a card brings). Every route below was
// measured on the version named, with lich's plugin and MCP server still
// loaded beside it; the cost of each is in that doc.

// SetCards wires what a card spawn brings (subagent.Cards.Card). Startup
// wiring, before any spawn; without it a card brings everything.
func (s *Service) SetCards(fn func(provider string) subagent.Card) { s.cards = fn }

// cardFor is what this spawn brings: a card's settings, nothing to leave out
// for a session nobody opened as a subagent, which the user's own sessions
// are.
func (s *Service) cardFor(kind string, depth int) subagent.Card {
	if depth == 0 || s.cards == nil {
		return subagent.Card{}
	}
	return s.cards(kind)
}

// Claude Code 2.1.332: --settings is a flag-level settings document, above the
// user's, project's and local settings (the schema's enabledPlugins
// description). false in enabledPlugins drops a plugin's hooks, skills and MCP
// servers; deniedMcpServers blocks a server by name in every scope, claude.ai
// connectors and --mcp-config included. Auto memory has its own variable.
const (
	claudeDisableMemoryVar = "CLAUDE_CODE_DISABLE_AUTO_MEMORY"
	// opencode 1.19.4 stops reading ~/.claude's CLAUDE.md and skills under
	// this variable (opencode's docs, Environment variables).
	opencodeDisableClaudeVar = "OPENCODE_DISABLE_CLAUDE_CODE"
)

// The oh-my-pi 19.1.1 flags (--help) that stop it discovering extensions,
// skills and rules; an explicit -e, which is lich's, still loads.
var ompAlsoFlags = map[string]string{
	api.SubagentAlsoExtensions: "--no-extensions",
	api.SubagentAlsoSkills:     "--no-skills",
	api.SubagentAlsoRules:      "--no-rules",
}

// claudeSettingsArgs is the one --settings document a Claude Code spawn takes:
// ultracode when on, and what a card leaves out. Claude Code reads one such
// flag, so both ride the same document. Unlike the model and effort it is
// passed on a resume too: Claude Code forgets ultracode there.
func claudeSettingsArgs(kind string, ultracode bool, card subagent.Card) []string {
	if kind != providers.Claude {
		return nil
	}
	doc := map[string]any{}
	if ultracode {
		doc["ultracode"] = true
	}
	if plugins := card.LeftNames(api.SubagentBringPlugin); len(plugins) > 0 {
		off := map[string]bool{}
		for _, id := range plugins {
			off[id] = false
		}
		doc["enabledPlugins"] = off
	}
	if servers := card.LeftNames(api.SubagentBringMCP); len(servers) > 0 {
		denied := make([]map[string]string, len(servers))
		for i, name := range servers {
			denied[i] = map[string]string{"serverName": name}
		}
		doc["deniedMcpServers"] = denied
	}
	if len(doc) == 0 {
		return nil
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		slog.Warn("claude settings skipped", "err", err)
		return nil
	}
	return []string{claudeSettingsFlag, string(raw)}
}

// cardArgs are the flags a card of oh-my-pi leaves its own discovery out with.
func cardArgs(kind string, card subagent.Card) []string {
	if kind != providers.OMP || !card.Lean {
		return nil
	}
	var out []string
	for _, id := range []string{api.SubagentAlsoExtensions, api.SubagentAlsoSkills, api.SubagentAlsoRules} {
		if !card.Brings(id) {
			out = append(out, ompAlsoFlags[id])
		}
	}
	return out
}

// cardEnv is what a card's environment leaves out: Claude Code's auto memory,
// opencode's reading of Claude Code's setup and the MCP servers of its own
// config, each turned off in the inline config it merges over them (measured
// on 1.19.4).
func cardEnv(env []string, kind string, card subagent.Card) []string {
	if !card.Lean {
		return env
	}
	switch kind {
	case providers.Claude:
		if !card.Brings(api.SubagentAlsoMemory) {
			env = append(env, claudeDisableMemoryVar+"=1")
		}
	case providers.OpenCode:
		if !card.Brings(api.SubagentAlsoClaudeSetup) {
			env = append(env, opencodeDisableClaudeVar+"=1")
		}
		return opencodeServersOff(env, card)
	}
	return env
}

// opencodeServersOff turns off the servers of the user's own opencode config
// the card leaves out. The library's are left out of the spawn instead
// (cardLibrary): an entry with nothing but enabled names no server opencode
// can start.
func opencodeServersOff(env []string, card subagent.Card) []string {
	var off []string
	for _, it := range card.Left {
		if it.Kind == api.SubagentBringMCP && it.Origin != api.SubagentOriginLibrary {
			off = append(off, it.Name)
		}
	}
	if len(off) == 0 {
		return env
	}
	extended, err := extendInlineConfig(env, opencodeConfigContentVar, func(config map[string]any) error {
		servers, err := objectAt(config, opencodeMCPKey)
		if err != nil {
			return err
		}
		for _, name := range off {
			servers[name] = map[string]any{"enabled": false}
		}
		return nil
	})
	if err != nil {
		slog.Warn("card servers left on", "provider", providers.OpenCode, "err", err)
		return env
	}
	return extended
}

// cardLibrary is the library's delivery without the servers the card leaves
// out.
func cardLibrary(lib library.Delivery, card subagent.Card) library.Delivery {
	left := card.LeftNames(api.SubagentBringMCP)
	if len(left) == 0 || len(lib.MCP) == 0 {
		return lib
	}
	lib.MCP = maps.Clone(lib.MCP)
	for _, name := range left {
		delete(lib.MCP, name)
	}
	return lib
}

// reportBrought records and announces what a card brought, for its tooltip,
// as reportSandbox does the sandbox.
func (s *Service) reportBrought(id string, isCard bool, card subagent.Card) {
	if !isCard {
		return
	}
	if err := s.store.SetSessionBrought(id, card.Brought); err != nil {
		slog.Warn("record what a card brought", "session", id, "error", err)
	}
	events.Send(s.hub.Emit, api.SessionBroughtEvent, api.SessionBrought{ID: id, Brought: card.Brought})
}
