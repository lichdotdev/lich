package main

import (
	"context"
	"testing"
	"time"
)

// A new key keeps the last answer on hand until its own lands, so the panel
// can go on showing it instead of a skeleton for the length of a round trip.
func TestGitFetchKeepsTheLastAnswerWhileTheNextLoads(t *testing.T) {
	landed := make(chan struct{}, 4)
	s := &gitStore{ctx: context.Background(), invalidate: func() { landed <- struct{}{} }}
	var slot gitLoaded[string]
	gitFetch(s, &slot, "a", func(context.Context) (string, error) { return "first", nil })
	select {
	case <-landed:
	case <-time.After(5 * time.Second):
		t.Fatal("the first answer never landed")
	}
	release := make(chan struct{})
	defer close(release)
	got := gitFetch(s, &slot, "b", func(context.Context) (string, error) {
		<-release
		return "second", nil
	})
	if got.loaded {
		t.Fatal("the second answer cannot have landed yet")
	}
	if !got.prev || got.val != "first" {
		t.Errorf("while b loads: prev %v, val %q; want the last answer, %q", got.prev, got.val, "first")
	}
}
