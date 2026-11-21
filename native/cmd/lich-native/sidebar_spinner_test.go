package main

import (
	"testing"
	"time"
)

func TestSoulsFrameLandsOnAFrameAtAnyTime(t *testing.T) {
	for _, now := range []time.Time{{}, time.Unix(0, 0), time.Unix(0, int64(soulsStep)*21)} {
		if f, _ := soulsFrame(now); f < 0 || f >= len(soulsFrames) {
			t.Errorf("frame at %v = %d, want one of %d", now, f, len(soulsFrames))
		}
	}
	if f, next := soulsFrame(time.Unix(0, int64(soulsStep)*21)); f != 1 || !next.Equal(time.Unix(0, int64(soulsStep)*22)) {
		t.Errorf("frame %d next %v, want frame 1 and the next step", f, next)
	}
}
