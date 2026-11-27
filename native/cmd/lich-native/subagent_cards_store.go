package main

import (
	"context"
	"log"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// subagentCardsStore is Settings › Subagents as the backend last read it, one
// per window: loaded at start and again on api.SubagentCardsChangedEvent and
// api.LibraryChangedEvent, since what a card can bring is the library's
// reading of the user's setup. Read by the Subagents screens.
type subagentCardsStore struct {
	mu     sync.Mutex
	list   []api.SubagentCardProvider
	loaded bool
}

// reloadSubagentCards reads the settings again after event name said they
// moved.
func (m *model) reloadSubagentCards(ctx context.Context, name string) {
	if err := m.subagentCards.load(ctx, m.client); err != nil {
		log.Printf("%s: %v", name, err)
	}
	m.invalidate()
}

func (s *subagentCardsStore) load(ctx context.Context, client *backend.Client) error {
	list, err := client.SubagentCards(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.list, s.loaded = list, true
	s.mu.Unlock()
	return nil
}

// provider is provider's settings, false before the first read or for a
// provider the backend does not list.
func (s *subagentCardsStore) provider(id string) (api.SubagentCardProvider, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.list {
		if p.Provider == id {
			p.Items = append([]api.SubagentCardItem(nil), p.Items...)
			return p, true
		}
	}
	return api.SubagentCardProvider{}, false
}

// setCards files a cards switch the person just pressed, until the backend's
// answer replaces it.
func (s *subagentCardsStore) setCards(provider string, on bool) {
	s.edit(provider, func(p *api.SubagentCardProvider) { p.Cards = on })
}

// setBring files an item switch the same way.
func (s *subagentCardsStore) setBring(provider, item string, on bool) {
	s.edit(provider, func(p *api.SubagentCardProvider) {
		for i := range p.Items {
			if p.Items[i].ID == item {
				p.Items[i].On = on
			}
		}
	})
}

func (s *subagentCardsStore) edit(provider string, edit func(*api.SubagentCardProvider)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].Provider == provider {
			edit(&s.list[i])
		}
	}
}
