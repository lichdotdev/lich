package main

import (
	"context"
	"log"
	"runtime"
	"sync/atomic"
	"time"
)

// stallAfter is how long handling one window event may take before it is
// reported as a stall. A handler that runs this long is not answering the
// compositor's pings, and Hyprland offers to Terminate an application that
// misses them for a few seconds: the suspected sender of a SIGKILL nothing else
// claims. 3s gets the stacks into the log before anyone can press it, well
// above any frame that is merely slow.
const stallAfter = 3 * time.Second

// stallCheckEvery is how often the watcher looks; a stall's reported length is
// as coarse as this.
const stallCheckEvery = time.Second

// notHandling marks the loop as waiting inside w.Event(), where a window with
// nothing to do may sit for minutes without being stalled.
const notHandling = -1

// stallWatch reports an event loop stuck handling one event. The loop pays two
// atomic stores per event; the watcher does the rest on its own goroutine.
type stallWatch struct {
	// now is a monotonic clock, injected by tests.
	now func() time.Duration
	// handlingSince is when the loop took the event it is handling, or
	// notHandling.
	handlingSince atomic.Int64
	// reported is the handlingSince of the stall last reported, or notHandling;
	// only the watcher reads or writes it.
	reported int64
	logf     func(format string, args ...any)
}

func newStallWatch() *stallWatch {
	start := time.Now()
	s := &stallWatch{now: func() time.Duration { return time.Since(start) }, reported: notHandling, logf: log.Printf}
	s.handlingSince.Store(notHandling)
	return s
}

func (s *stallWatch) handling() { s.handlingSince.Store(int64(s.now())) }

func (s *stallWatch) waiting() { s.handlingSince.Store(notHandling) }

// watch checks the loop until ctx ends.
func (s *stallWatch) watch(ctx context.Context) {
	tick := time.NewTicker(stallCheckEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.check()
		}
	}
}

// check reports a stall once, with every goroutine's stack, and its end once
// the loop moved past the event it was stuck on.
func (s *stallWatch) check() {
	since := s.handlingSince.Load()
	now := s.now()
	if s.reported != notHandling {
		if since == s.reported {
			return
		}
		s.logf("event loop recovered after %s", (now - time.Duration(s.reported)).Round(time.Second))
		s.reported = notHandling
	}
	if since == notHandling {
		return
	}
	held := now - time.Duration(since)
	if held < stallAfter {
		return
	}
	s.reported = since
	s.logf("event loop stalled for %s\n%s", held.Round(time.Second), allStacks())
}

// allStacks is every goroutine's stack, the buffer grown until it fits.
func allStacks() []byte {
	buf := make([]byte, 64<<10)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}
