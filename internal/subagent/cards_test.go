package subagent

import (
	"context"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/store"
)

// With a provider's cards off its subagent runs natively and opens nothing,
// whatever the provider.
func TestCardsOffKeepsTheSubagentNative(t *testing.T) {
	for _, p := range providers.Registry {
		t.Run(p.ID, func(t *testing.T) {
			svc, w, vetoes := newFake(Asker{Kind: p.ID, CardsOff: true})
			got, commit := svc.Decide(context.Background(), spawnReq("toolu_1", api.SubagentTask{Title: "Fix it", Prompt: "Fix #412."}))
			commit(true)
			if got.Decision != api.SubagentAllow || len(w.opened) != 0 || len(*vetoes) != 0 {
				t.Errorf("decision %q, opened %d, vetoes %d; want a native subagent and nothing opened", got.Decision, len(w.opened), len(*vetoes))
			}
		})
	}
}

func TestWorkspaceReadsCardsOff(t *testing.T) {
	w := Workspace{
		Load: func(context.Context, api.None) ([]store.Project, error) {
			return []store.Project{{Sessions: []store.Session{{ID: "s1", Kind: providers.OpenCode}}}}, nil
		},
		Depth:    func(string) int { return 0 },
		CardsOff: func(kind string) bool { return kind == providers.OpenCode },
	}
	if got, _ := w.Asker("s1"); !got.CardsOff {
		t.Errorf("Asker = %+v, want opencode's cards off", got)
	}
}

type fakeCardStore struct {
	off    map[string]bool
	brings map[string][]string
}

func (f *fakeCardStore) SubagentCards(id string) bool { return !f.off[id] }
func (f *fakeCardStore) SetSubagentCards(id string, on bool) error {
	f.off[id] = !on
	return nil
}
func (f *fakeCardStore) SubagentBrings(id string) []string { return f.brings[id] }
func (f *fakeCardStore) SessionKind(id string) string {
	if id == "s1" {
		return providers.Claude
	}
	return ""
}
func (f *fakeCardStore) SetSubagentBring(id, item string, on bool) error {
	items := slices.DeleteFunc(f.brings[id], func(it string) bool { return it == item })
	if on {
		items = append(items, item)
	}
	f.brings[id] = items
	return nil
}

// fakeItems lists the same setup for every provider: two servers, one of
// them the library's, and a plugin.
type fakeItems struct{}

func (fakeItems) CardItems(string) []api.SubagentCardItem {
	return []api.SubagentCardItem{
		{ID: "mcp:ai-memory", Kind: api.SubagentBringMCP, Name: "ai-memory", Origin: ".claude.json"},
		{ID: "mcp:github", Kind: api.SubagentBringMCP, Name: "github", Origin: api.SubagentOriginLibrary},
		{ID: "plugin:ponytail@ponytail", Kind: api.SubagentBringPlugin, Name: "ponytail", Origin: "ponytail"},
	}
}

func newCards() (*Cards, *fakeCardStore, *int) {
	store := &fakeCardStore{off: map[string]bool{}, brings: map[string][]string{}}
	emitted := 0
	return NewCards(store, fakeItems{}, func() { emitted++ }), store, &emitted
}

// With nothing picked a card leaves out every item its provider can, brings
// nothing, and a provider with no leaner start leaves out nothing.
func TestACardBringsNothingByDefault(t *testing.T) {
	wantLeft := map[string][]string{
		providers.Claude:   {"mcp:ai-memory", "mcp:github", "plugin:ponytail@ponytail"},
		providers.OpenCode: {"mcp:ai-memory", "mcp:github"},
		providers.OMP:      nil,
	}
	cards, _, _ := newCards()
	for _, p := range providers.Registry {
		t.Run(p.ID, func(t *testing.T) {
			card := cards.Card(p.ID)
			want, lean := wantLeft[p.ID]
			if card.Lean != lean {
				t.Fatalf("Lean = %v, want %v", card.Lean, lean)
			}
			var left []string
			for _, it := range card.Left {
				left = append(left, it.ID)
			}
			if !slices.Equal(left, want) || len(card.Brought) != 0 {
				t.Errorf("left %v brought %v, want left %v and nothing brought", left, card.Brought, want)
			}
		})
	}
}

// What the user picks is brought and no longer left out, an also item
// included.
func TestPickedItemsPassThrough(t *testing.T) {
	cards, store, _ := newCards()
	store.brings[providers.Claude] = []string{"mcp:ai-memory", api.SubagentAlsoMemory, "plugin:ponytail@ponytail"}
	card := cards.Card(providers.Claude)
	if got := card.LeftNames(api.SubagentBringMCP); !slices.Equal(got, []string{"github"}) {
		t.Errorf("left servers %v, want github alone", got)
	}
	if got := card.LeftNames(api.SubagentBringPlugin); len(got) != 0 {
		t.Errorf("left plugins %v, want none", got)
	}
	for _, id := range []string{"mcp:ai-memory", api.SubagentAlsoMemory, "plugin:ponytail@ponytail"} {
		if !card.Brings(id) {
			t.Errorf("does not bring %s", id)
		}
	}
}

func TestListShowsEachProvidersChoices(t *testing.T) {
	cards, store, _ := newCards()
	store.off[providers.Codex] = true
	store.brings[providers.OMP] = []string{api.SubagentAlsoRules}
	list, err := cards.List(context.Background(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(providers.Registry) {
		t.Fatalf("%d providers, want %d", len(list), len(providers.Registry))
	}
	by := map[string]api.SubagentCardProvider{}
	for i, p := range list {
		if p.Provider != providers.Registry[i].ID {
			t.Errorf("provider %d is %s, want registry order", i, p.Provider)
		}
		by[p.Provider] = p
	}
	if c := by[providers.Codex]; c.Cards || c.Lean || len(c.Items) != 0 {
		t.Errorf("codex = %+v, want cards off, nothing to choose", c)
	}
	ids := func(p api.SubagentCardProvider) (out []string) {
		for _, it := range p.Items {
			out = append(out, it.ID)
		}
		return out
	}
	if got := ids(by[providers.OpenCode]); !slices.Equal(got, []string{"mcp:ai-memory", "mcp:github", api.SubagentAlsoClaudeSetup}) {
		t.Errorf("opencode items %v", got)
	}
	omp := by[providers.OMP]
	if got := ids(omp); !slices.Equal(got, []string{api.SubagentAlsoExtensions, api.SubagentAlsoSkills, api.SubagentAlsoRules}) {
		t.Errorf("omp items %v", got)
	}
	for _, it := range omp.Items {
		if it.On != (it.ID == api.SubagentAlsoRules) {
			t.Errorf("omp %s on = %v", it.ID, it.On)
		}
	}
}

func TestSettingsChangeAndTellTheWindow(t *testing.T) {
	cards, _, emitted := newCards()
	ctx := context.Background()
	if _, err := cards.SetCards(ctx, api.SubagentSetCardsOptions{Provider: providers.Claude, On: false}); err != nil {
		t.Fatal(err)
	}
	if cards.On(providers.Claude) {
		t.Error("cards still on")
	}
	if _, err := cards.SetBring(ctx, api.SubagentSetBringOptions{Provider: providers.Claude, Item: "mcp:github", On: true}); err != nil {
		t.Fatal(err)
	}
	if !cards.Card(providers.Claude).Brings("mcp:github") {
		t.Error("the pick did not reach the next card")
	}
	if *emitted != 2 {
		t.Errorf("emitted %d changes, want 2", *emitted)
	}
	for _, opts := range []api.SubagentSetBringOptions{
		{Provider: providers.Codex, Item: "mcp:github", On: true},
		{Provider: providers.Claude, Item: " ", On: true},
	} {
		if _, err := cards.SetBring(ctx, opts); err == nil {
			t.Errorf("%+v was taken", opts)
		}
	}
	if _, err := cards.SetCards(ctx, api.SubagentSetCardsOptions{Provider: "nobody"}); err == nil {
		t.Error("an unknown provider was taken")
	}
	if *emitted != 2 {
		t.Errorf("a refused change emitted: %d", *emitted)
	}
}

// A session asks whether its own provider opens cards; a session lich does
// not know is refused.
func TestCardsOnReadsTheSessionsProvider(t *testing.T) {
	cards, store, _ := newCards()
	store.off[providers.Claude] = true
	got, err := cards.CardsOn(context.Background(), api.SessionIDOptions{ID: "s1"})
	if err != nil || got.Cards {
		t.Errorf("CardsOn = %+v, %v; want Claude Code's cards off", got, err)
	}
	if _, err := cards.CardsOn(context.Background(), api.SessionIDOptions{ID: "nobody"}); err == nil {
		t.Error("an unknown session was answered")
	}
}
