package main

import "sync"

// askOnce holds what the backend answered per key, each key asked once per
// visit and off the frame.
type askOnce[V any] struct {
	got   map[string]V
	asked map[string]bool
}

// get returns key's answer, ok false until fetch answered. It takes mu, the
// lock guarding a; fetch runs on its own goroutine without it, and an ok false
// from fetch leaves key unanswered for the rest of the visit.
func (a *askOnce[V]) get(mu *sync.Mutex, key string, invalidate func(), fetch func() (V, bool)) (V, bool) {
	mu.Lock()
	defer mu.Unlock()
	if v, ok := a.got[key]; ok {
		return v, true
	}
	if !a.asked[key] {
		if a.asked == nil {
			a.asked = map[string]bool{}
		}
		a.asked[key] = true
		go func() {
			v, ok := fetch()
			if !ok {
				return
			}
			mu.Lock()
			a.put(key, v)
			mu.Unlock()
			invalidate()
		}()
	}
	var zero V
	return zero, false
}

// put records v as key's answer; the caller holds the lock guarding a.
func (a *askOnce[V]) put(key string, v V) {
	if a.got == nil {
		a.got = map[string]V{}
	}
	a.got[key] = v
}

// reset forgets every answer and question; the caller holds the lock guarding a.
func (a *askOnce[V]) reset() {
	a.got, a.asked = nil, nil
}
