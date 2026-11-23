package main

import (
	"context"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// vetoStore is the provider subagent calls lich opened as cards, by the chat
// and tool call they came from: a chat draws such a call as opened in a card
// rather than as the failed call the provider reports (subagent-veto).
type vetoStore struct {
	mu     sync.Mutex
	byCall map[string][]api.SubagentOpened
}

func vetoKey(session, toolUseID string) string { return session + "\x00" + toolUseID }

func (s *vetoStore) add(v api.SubagentVeto) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byCall == nil {
		s.byCall = map[string][]api.SubagentOpened{}
	}
	s.byCall[vetoKey(v.SessionID, v.ToolUseID)] = v.Sessions
}

// load reads the vetoes made before the window attached.
func (s *vetoStore) load(ctx context.Context, client *backend.Client) error {
	list, err := client.SubagentVetoed(ctx)
	if err != nil {
		return err
	}
	for _, v := range list {
		s.add(v)
	}
	return nil
}

// opened is the sessions tool call toolUseID of session's chat was opened as,
// false for a call lich did not veto.
func (s *vetoStore) opened(session, toolUseID string) ([]api.SubagentOpened, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.byCall[vetoKey(session, toolUseID)]
	return o, ok && len(o) > 0
}
