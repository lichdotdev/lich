package subagent

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// Settings › Subagents: whether a provider's subagents open as cards, and
// what a card brings of the user's own setup beyond lich and the project
// (docs/hooks/subagent-cards.md, What a card brings).

// CardStore keeps the settings.
type CardStore interface {
	SubagentCards(providerID string) bool
	SetSubagentCards(providerID string, on bool) error
	SubagentBrings(providerID string) []string
	SetSubagentBring(providerID, item string, on bool) error
	SessionKind(sessionID string) string
}

// CardItems reads the MCP servers and plugins of the user's own setup a
// session of provider loads (library.CardItems).
type CardItems interface {
	CardItems(provider string) []api.SubagentCardItem
}

// leanKnobs is what a card of a provider can leave out, by the routes
// internal/terminal spawns it with (cardArgs, cardEnv): item by item for the
// kinds named, and each also item as a whole. A provider missing here has no
// way to start with less, and a card of it brings everything
// (docs/ceilings.md).
type leanKnobs struct {
	kinds []string
	also  []string
}

var lean = map[string]leanKnobs{
	providers.Claude:   {kinds: []string{api.SubagentBringMCP, api.SubagentBringPlugin}, also: []string{api.SubagentAlsoMemory}},
	providers.OpenCode: {kinds: []string{api.SubagentBringMCP}, also: []string{api.SubagentAlsoClaudeSetup}},
	providers.OMP:      {also: []string{api.SubagentAlsoExtensions, api.SubagentAlsoSkills, api.SubagentAlsoRules}},
}

// Cards is the settings over RPC and what a card spawn reads of them.
type Cards struct {
	store CardStore
	items CardItems
	emit  func()
}

// NewCards is the settings in store, listing items, telling emit of each
// change once it is stored.
func NewCards(store CardStore, items CardItems, emit func()) *Cards {
	return &Cards{store: store, items: items, emit: emit}
}

// List answers api.SubagentCards.
func (c *Cards) List(context.Context, api.None) ([]api.SubagentCardProvider, error) {
	out := make([]api.SubagentCardProvider, 0, len(providers.Registry))
	for _, p := range providers.Registry {
		knobs, ok := lean[p.ID]
		out = append(out, api.SubagentCardProvider{Provider: p.ID, Cards: c.store.SubagentCards(p.ID), Lean: ok, Items: c.listed(p.ID, knobs)})
	}
	return out, nil
}

// listed is what a card of provider can bring, each with whether it does.
func (c *Cards) listed(provider string, knobs leanKnobs) []api.SubagentCardItem {
	brings := c.store.SubagentBrings(provider)
	items := []api.SubagentCardItem{}
	if len(knobs.kinds) > 0 {
		for _, it := range c.items.CardItems(provider) {
			if slices.Contains(knobs.kinds, it.Kind) {
				items = append(items, it)
			}
		}
	}
	for _, id := range knobs.also {
		items = append(items, api.SubagentCardItem{ID: id, Kind: api.SubagentBringAlso, Name: id})
	}
	for i := range items {
		items[i].On = slices.Contains(brings, items[i].ID)
	}
	return items
}

// SetCards answers api.SubagentSetCards.
func (c *Cards) SetCards(_ context.Context, opts api.SubagentSetCardsOptions) (api.None, error) {
	if !providers.Known(opts.Provider) {
		return api.None{}, fmt.Errorf("no provider %q", opts.Provider)
	}
	if err := c.store.SetSubagentCards(opts.Provider, opts.On); err != nil {
		return api.None{}, err
	}
	c.changed()
	return api.None{}, nil
}

// SetBring answers api.SubagentSetBring.
func (c *Cards) SetBring(_ context.Context, opts api.SubagentSetBringOptions) (api.None, error) {
	if _, ok := lean[opts.Provider]; !ok {
		return api.None{}, fmt.Errorf("a card of %q brings everything and has nothing to choose", opts.Provider)
	}
	if strings.TrimSpace(opts.Item) == "" {
		return api.None{}, fmt.Errorf("no item named")
	}
	if err := c.store.SetSubagentBring(opts.Provider, opts.Item, opts.On); err != nil {
		return api.None{}, err
	}
	c.changed()
	return api.None{}, nil
}

func (c *Cards) changed() {
	if c.emit != nil {
		c.emit()
	}
}

// CardsOn answers api.SubagentCardsOn.
func (c *Cards) CardsOn(_ context.Context, opts api.SessionIDOptions) (api.SubagentCardsState, error) {
	kind := c.store.SessionKind(opts.ID)
	if kind == "" {
		return api.SubagentCardsState{}, fmt.Errorf("lich has no session with id %q", opts.ID)
	}
	return api.SubagentCardsState{Cards: c.store.SubagentCards(kind)}, nil
}

// On reports whether provider's subagents open as cards.
func (c *Cards) On(provider string) bool { return c.store.SubagentCards(provider) }

// Card is what one card spawn of a provider brings and leaves out.
type Card struct {
	// Lean is whether the provider can start with less; a card that cannot
	// brings everything, and the rest is empty.
	Lean bool
	// Left are the MCP servers and plugins it leaves out.
	Left []api.SubagentCardItem
	// Brought are the IDs of what it brings, listed items and also ones.
	Brought []string
}

// Card is what a card of provider spawned now brings.
func (c *Cards) Card(provider string) Card {
	knobs, ok := lean[provider]
	if !ok {
		return Card{}
	}
	card := Card{Lean: true, Brought: []string{}}
	for _, it := range c.listed(provider, knobs) {
		switch {
		case it.On:
			card.Brought = append(card.Brought, it.ID)
		case it.Kind != api.SubagentBringAlso:
			card.Left = append(card.Left, it)
		}
	}
	return card
}

// Brings reports whether the card brings item id.
func (c Card) Brings(id string) bool { return slices.Contains(c.Brought, id) }

// LeftNames are the names of the items of kind it leaves out: a server's
// name, a plugin's name@marketplace.
func (c Card) LeftNames(kind string) []string {
	var out []string
	for _, it := range c.Left {
		if it.Kind == kind {
			out = append(out, strings.TrimPrefix(it.ID, kind+":"))
		}
	}
	return out
}
