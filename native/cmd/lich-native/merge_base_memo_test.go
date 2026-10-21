package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestMergeBaseMemoAsksOncePerKey(t *testing.T) {
	var m mergeBaseMemo
	var calls atomic.Int32
	release := make(chan struct{})
	fetch := func() (string, error) {
		calls.Add(1)
		<-release
		return "abc1234", nil
	}
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if oid, err := m.get(context.Background(), "pr@head1", fetch); err != nil || oid != "abc1234" {
				t.Errorf("get = %q, %v; want abc1234", oid, err)
			}
		}()
	}
	close(release)
	wg.Wait()
	if _, err := m.get(context.Background(), "pr@head1", fetch); err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("fetched %d times for one key, want 1", n)
	}
	if _, err := m.get(context.Background(), "pr@head2", fetch); err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("fetched %d times after a new head, want 2", n)
	}
}

func TestMergeBaseMemoRetriesAfterAFailure(t *testing.T) {
	var m mergeBaseMemo
	fail := errors.New("rate limited")
	if _, err := m.get(context.Background(), "k", func() (string, error) { return "", fail }); !errors.Is(err, fail) {
		t.Fatalf("err = %v, want the fetch's failure", err)
	}
	oid, err := m.get(context.Background(), "k", func() (string, error) { return "def5678", nil })
	if err != nil || oid != "def5678" {
		t.Errorf("after a failure: %q, %v; want a new fetch", oid, err)
	}
}

func TestMergeBaseMemoWaiterGivesUpAlone(t *testing.T) {
	var m mergeBaseMemo
	release := make(chan struct{})
	fetch := func() (string, error) { <-release; return "abc1234", nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.get(ctx, "k", fetch); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled waiter: err = %v, want context.Canceled", err)
	}
	close(release)
	if oid, err := m.get(context.Background(), "k", fetch); err != nil || oid != "abc1234" {
		t.Errorf("another waiter: %q, %v; want the fetch's answer", oid, err)
	}
}
