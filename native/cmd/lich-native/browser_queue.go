package main

import (
	"context"
	"sync"

	"github.com/lichdotdev/lich/api"
)

// browserQueue sends one pane's input to the backend in order, one call at
// a time. Chrome takes ~15ms to answer each pointer event (measured
// 2026-10-26), less often than a touchpad or a moving mouse fires them, so
// what piles up behind a call in flight is merged: moves and resizes keep
// the last one and scrolls add up. Presses, releases, keys and text are never
// merged.
type browserQueue struct {
	mu    sync.Mutex
	calls []browserCall
	wake  chan struct{}
}

// browserCall is one queued call: a pointer event to merge (mouse set), a
// resize to merge (resize set), or any other call.
type browserCall struct {
	mouse  *api.BrowserMouseOptions
	resize bool
	send   func(context.Context) error
}

func newBrowserQueue() *browserQueue {
	return &browserQueue{wake: make(chan struct{}, 1)}
}

func (q *browserQueue) push(c browserCall) {
	q.mu.Lock()
	if n := len(q.calls); n > 0 && c.resize && q.calls[n-1].resize {
		q.calls[n-1] = c
		q.mu.Unlock()
		return
	}
	if n := len(q.calls); n > 0 && c.mouse != nil && q.calls[n-1].mouse != nil {
		last := q.calls[n-1].mouse
		switch {
		case c.mouse.Type == "move" && last.Type == "move":
			q.calls[n-1] = c
			q.mu.Unlock()
			return
		case c.mouse.Type == "wheel" && last.Type == "wheel":
			last.DX, last.DY = last.DX+c.mouse.DX, last.DY+c.mouse.DY
			q.mu.Unlock()
			return
		}
	}
	q.calls = append(q.calls, c)
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// next takes the oldest call, false when none is queued.
func (q *browserQueue) next() (browserCall, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.calls) == 0 {
		return browserCall{}, false
	}
	c := q.calls[0]
	q.calls = q.calls[1:]
	return c, true
}

// run sends the queued calls until ctx ends, reporting each failure to fail.
func (q *browserQueue) run(ctx context.Context, fail func(error)) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		}
		for c, ok := q.next(); ok; c, ok = q.next() {
			if err := c.send(ctx); err != nil {
				fail(err)
			}
		}
	}
}
