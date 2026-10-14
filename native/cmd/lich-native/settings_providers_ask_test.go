package main

import (
	"sync"
	"testing"
	"time"
)

func TestAskOnceAsksEachKeyOncePerVisit(t *testing.T) {
	var mu sync.Mutex
	var a askOnce[string]
	answered := make(chan struct{}, 4)
	calls := 0
	fetch := func(v string, ok bool) func() (string, bool) {
		return func() (string, bool) {
			mu.Lock()
			calls++
			mu.Unlock()
			return v, ok
		}
	}
	invalidate := func() { answered <- struct{}{} }

	if _, ok := a.get(&mu, "k", invalidate, fetch("v", true)); ok {
		t.Fatal("answered before fetch ran")
	}
	waitAnswer(t, answered)
	if v, ok := a.get(&mu, "k", invalidate, fetch("other", true)); !ok || v != "v" {
		t.Fatalf("get after answer = %q, %v; want \"v\", true", v, ok)
	}

	a.get(&mu, "fails", invalidate, fetch("", false))
	a.get(&mu, "fails", invalidate, fetch("", false))
	waitCalls(t, &mu, &calls, 2)

	mu.Lock()
	a.reset()
	mu.Unlock()
	a.get(&mu, "k", invalidate, fetch("again", true))
	waitAnswer(t, answered)
	if v, ok := a.get(&mu, "k", invalidate, fetch("", true)); !ok || v != "again" {
		t.Fatalf("get after reset = %q, %v; want \"again\", true", v, ok)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 3 {
		t.Fatalf("fetch ran %d times, want 3", calls)
	}
}

func waitAnswer(t *testing.T, answered <-chan struct{}) {
	t.Helper()
	select {
	case <-answered:
	case <-time.After(time.Second):
		t.Fatal("fetch never answered")
	}
}

func waitCalls(t *testing.T, mu *sync.Mutex, calls *int, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		n := *calls
		mu.Unlock()
		if n == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("fetch ran %d times, want %d", n, want)
		}
		time.Sleep(time.Millisecond)
	}
}
