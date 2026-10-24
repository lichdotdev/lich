package main

import (
	"context"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
)

// threadFixStore is the review threads waiting for their pull request's next
// push, as the backend last announced them (api.ThreadFixesEvent): the Pulls
// screen reads it, and only the event writes it.
type threadFixStore struct {
	mu      sync.Mutex
	waiting map[string]bool
}

func (s *threadFixStore) set(list []backend.ThreadFix) {
	waiting := make(map[string]bool, len(list))
	for _, f := range list {
		waiting[f.ThreadID] = true
	}
	s.mu.Lock()
	s.waiting = waiting
	s.mu.Unlock()
}

func (s *threadFixStore) load(ctx context.Context, client *backend.Client) error {
	list, err := client.ThreadFixes(ctx)
	if err != nil {
		return err
	}
	s.set(list)
	return nil
}

// has is whether thread id waits for a push.
func (s *threadFixStore) has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.waiting[id]
}
