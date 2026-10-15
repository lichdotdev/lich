package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeStallWatch is a stallWatch on a clock the test moves, recording what it
// logs.
func fakeStallWatch() (*stallWatch, *time.Duration, *[]string) {
	var clock time.Duration
	var logged []string
	s := newStallWatch()
	s.now = func() time.Duration { return clock }
	s.logf = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	return s, &clock, &logged
}

func TestStallWatchIgnoresAnIdleLoop(t *testing.T) {
	s, clock, logged := fakeStallWatch()
	*clock = time.Hour
	s.check()
	if len(*logged) != 0 {
		t.Errorf("an idle loop logged %q", *logged)
	}
}

func TestStallWatchIgnoresAQuickEvent(t *testing.T) {
	s, clock, logged := fakeStallWatch()
	s.handling()
	*clock += stallAfter - time.Millisecond
	s.check()
	s.waiting()
	*clock += time.Hour
	s.check()
	if len(*logged) != 0 {
		t.Errorf("an event under the threshold logged %q", *logged)
	}
}

func TestStallWatchReportsAStallOnceWithStacksThenItsEnd(t *testing.T) {
	s, clock, logged := fakeStallWatch()
	*clock = time.Minute
	s.handling()
	*clock += 4 * time.Second
	s.check()
	*clock += time.Second
	s.check()
	*clock += 2 * time.Second
	s.waiting()
	s.check()

	if len(*logged) != 2 {
		t.Fatalf("logged %d lines, want the stall and its end: %q", len(*logged), *logged)
	}
	stalled := (*logged)[0]
	if !strings.HasPrefix(stalled, "event loop stalled for 4s\n") || !strings.Contains(stalled, "goroutine ") {
		t.Errorf("stall line = %.80q, want its length and the stacks", stalled)
	}
	if (*logged)[1] != "event loop recovered after 7s" {
		t.Errorf("recovery line = %q", (*logged)[1])
	}
}

// TestStallWatchReportsTheNextEventsStall proves a loop that recovered into
// another stuck event is reported again, not taken for the old stall.
func TestStallWatchReportsTheNextEventsStall(t *testing.T) {
	s, clock, logged := fakeStallWatch()
	s.handling()
	*clock += 5 * time.Second
	s.check()
	s.handling()
	*clock += 5 * time.Second
	s.check()

	if len(*logged) != 3 || !strings.HasPrefix((*logged)[2], "event loop stalled for 5s") {
		t.Errorf("logged %q, want stall, recovery, stall", *logged)
	}
}
