package main

import (
	"context"
	"sync"
)

// remote is use-remote-resource.ts for the Code tab: the answer to the latest
// key, fetched off the UI goroutine. A new key keeps the last answer on screen
// until its own lands, so a poll tick or a keystroke never flashes the panel
// empty; a new reset (another checkout) drops it, so one checkout's answer
// never stands in for another's. An empty key holds nothing.
type remote[T any] struct {
	invalidate func()

	mu      sync.Mutex
	key     string
	reset   string
	cancel  context.CancelFunc
	data    T
	has     bool // data answers some key under reset
	err     error
	settled bool // the latest key has answered
}

// want asks for key's answer, fetching it unless it is the one already held
// or on its way.
func (r *remote[T]) want(key, reset string, fetch func(context.Context) (T, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if key == r.key && reset == r.reset {
		return
	}
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if reset != r.reset || key == "" {
		var zero T
		r.data, r.has, r.err = zero, false, nil
	}
	r.key, r.reset, r.settled = key, reset, key == ""
	if key == "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.fetch(ctx, key, fetch)
}

func (r *remote[T]) fetch(ctx context.Context, key string, fetch func(context.Context) (T, error)) {
	data, err := fetch(ctx)
	r.mu.Lock()
	if ctx.Err() != nil || key != r.key {
		r.mu.Unlock()
		return
	}
	r.settled, r.err = true, err
	if err == nil {
		r.data, r.has = data, true
	}
	r.mu.Unlock()
	r.invalidate()
}

// get is what to draw: the last answer, whether there is none yet to stand
// on (loading), and the latest key's error.
func (r *remote[T]) get() (data T, loading bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data, !r.has && !r.settled, r.err
}

// stop drops the request in flight and forgets the key, so the next want
// fetches afresh.
func (r *remote[T]) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	r.key = ""
}
