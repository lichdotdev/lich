package api

// Settings › Subagents: per provider, whether its subagents open as cards and
// what a card brings of the user's own setup beyond lich and the project's
// instructions (docs/hooks/subagent-cards.md, What a card brings).

var (
	// SubagentCards is every provider's card settings, in providers.Registry
	// order.
	SubagentCards = define[None, []SubagentCardProvider]("subagent.Cards")
	// SubagentSetCards turns a provider's cards on or off.
	SubagentSetCards = define[SubagentSetCardsOptions, None]("subagent.SetCards")
	// SubagentSetBring turns one item a card of a provider brings on or off.
	SubagentSetBring = define[SubagentSetBringOptions, None]("subagent.SetBring")
)

// The kinds of item a card can bring, as SubagentCardItem.Kind names them.
const (
	SubagentBringMCP    = "mcp"
	SubagentBringPlugin = "plugin"
	// SubagentBringAlso is one all-or-nothing part of the provider's setup,
	// named by its ID: SubagentAlsoMemory and the rest.
	SubagentBringAlso = "also"
)

// The all-or-nothing items, as a SubagentBringAlso item's ID.
const (
	// SubagentAlsoMemory is Claude Code's auto memory.
	SubagentAlsoMemory = "memory"
	// SubagentAlsoClaudeSetup is what opencode reads of Claude Code's own
	// setup: ~/.claude's CLAUDE.md and skills.
	SubagentAlsoClaudeSetup = "claude-setup"
	// SubagentAlsoExtensions, SubagentAlsoSkills and SubagentAlsoRules are what
	// oh-my-pi discovers on its own.
	SubagentAlsoExtensions = "extensions"
	SubagentAlsoSkills     = "skills"
	SubagentAlsoRules      = "rules"
)

// SubagentOriginLibrary is the Origin of an MCP server lich's library hands
// the provider, rather than one its own config holds.
const SubagentOriginLibrary = "library"

// SubagentCardProvider is one provider's card settings. Cards is whether its
// subagents open as cards. Lean is whether a card of it can start with less
// than the user's whole setup; Items is what it can bring, empty when it
// cannot.
type SubagentCardProvider struct {
	Provider string             `json:"provider"`
	Cards    bool               `json:"cards"`
	Lean     bool               `json:"lean"`
	Items    []SubagentCardItem `json:"items"`
}

// SubagentCardItem is one thing a card can bring. ID is what SubagentSetBring
// and SessionBrought name it by, Name what the screen shows (the window names
// an "also" item from its ID), Origin where it is configured: a file's name,
// a plugin's marketplace or SubagentOriginLibrary.
type SubagentCardItem struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Origin string `json:"origin,omitempty"`
	On     bool   `json:"on"`
}

// SubagentSetCardsOptions turns Provider's cards on or off.
type SubagentSetCardsOptions struct {
	Provider string `json:"provider"`
	On       bool   `json:"on"`
}

// SubagentSetBringOptions turns item Item of Provider's cards on or off.
type SubagentSetBringOptions struct {
	Provider string `json:"provider"`
	Item     string `json:"item"`
	On       bool   `json:"on"`
}

// SessionBrought is the payload of SessionBroughtEvent: a card and the items
// it brought beyond lich and the project, by SubagentCardItem.ID.
type SessionBrought struct {
	ID      string   `json:"id"`
	Brought []string `json:"brought"`
}
