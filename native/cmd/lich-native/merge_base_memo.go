package main

import (
	"context"
	"sync"
)

// mergeBaseMemo holds one pull request's merge base for every image card that
// stands on it: each ask spends GitHub calls of the hourly rate limit, so the
// cards share one, asked again only for a new key (another head) or after a
// failure.
type mergeBaseMemo struct {
	mu   sync.Mutex
	key  string
	call *mergeBaseCall
}

type mergeBaseCall struct {
	done chan struct{}
	oid  string
	err  error
}

// get is key's merge base, fetched once whoever asks first. fetch runs on
// its own, not under ctx: a card that stops waiting must not fail the others.
func (m *mergeBaseMemo) get(ctx context.Context, key string, fetch func() (string, error)) (string, error) {
	m.mu.Lock()
	call := m.call
	if call == nil || m.key != key {
		call = &mergeBaseCall{done: make(chan struct{})}
		m.key, m.call = key, call
		go m.run(call, fetch)
	}
	m.mu.Unlock()
	select {
	case <-call.done:
		return call.oid, call.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (m *mergeBaseMemo) run(call *mergeBaseCall, fetch func() (string, error)) {
	call.oid, call.err = fetch()
	if call.err != nil {
		m.mu.Lock()
		if m.call == call {
			m.call = nil
		}
		m.mu.Unlock()
	}
	close(call.done)
}
