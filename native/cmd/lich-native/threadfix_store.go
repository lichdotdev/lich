package main

import (
	"context"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
)

// threadFixStore is the review threads handed to a session, as the backend
// last announced them (api.ThreadFixesEvent): the Pulls screen reads it, and
// only the event writes it.
type threadFixStore struct {
	mu sync.Mutex
	// fixed is whether a sent thread's agent marked the commit that fixes it.
	fixed map[string]bool
}

func (s *threadFixStore) set(list []backend.ThreadFix) {
	fixed := make(map[string]bool, len(list))
	for _, f := range list {
		fixed[f.ThreadID] = f.Commit != ""
	}
	s.mu.Lock()
	s.fixed = fixed
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

// state is whether thread id was handed to a session, and whether its agent
// marked it fixed, so it waits for the push of that fix.
func (s *threadFixStore) state(id string) (sent, fixed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fixed, sent = s.fixed[id]
	return sent, fixed
}
