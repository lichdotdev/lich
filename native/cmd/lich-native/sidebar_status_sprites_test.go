package main

import (
	"strings"
	"testing"
	"time"
)

func TestStatusSpritesAreElevenByElevenWithAStepEach(t *testing.T) {
	for name, s := range map[string]statusSprite{"hourglass": hourglassSprite, "glass enter": glassEnter, "check": checkFusion, "check done": {frames: []string{checkDone}, steps: []time.Duration{time.Second}}} {
		if len(s.frames) == 0 || len(s.frames) != len(s.steps) {
			t.Fatalf("%s: %d frames, %d steps", name, len(s.frames), len(s.steps))
		}
		for i, f := range s.frames {
			rows := strings.Split(f, " | ")
			if len(rows) != soulsCells {
				t.Errorf("%s frame %d: %d rows", name, i, len(rows))
			}
			for _, r := range rows {
				if len(r) != soulsCells || strings.Trim(r, ".1345678") != "" {
					t.Errorf("%s frame %d: row %q", name, i, r)
				}
			}
		}
	}
}

func TestAStatusSpriteHoldsEachFrameForItsStep(t *testing.T) {
	s := statusSprite{frames: []string{"a", "b", "c"}, steps: []time.Duration{500 * time.Millisecond, 90 * time.Millisecond, 90 * time.Millisecond}}
	cases := []struct {
		since time.Duration
		frame int
		left  time.Duration
	}{
		{0, 0, 500 * time.Millisecond},
		{499 * time.Millisecond, 0, time.Millisecond},
		{500 * time.Millisecond, 1, 90 * time.Millisecond},
		{620 * time.Millisecond, 2, 60 * time.Millisecond},
		{680 * time.Millisecond, 0, 500 * time.Millisecond},
		{-10 * time.Millisecond, 2, 10 * time.Millisecond},
	}
	for _, c := range cases {
		if f, left := s.at(c.since); f != c.frame || left != c.left {
			t.Errorf("at(%v) = %d, %v; want %d, %v", c.since, f, left, c.frame, c.left)
		}
	}
}

func TestTheFusionStartsWhenTheSpinnerMeetsInTheCenter(t *testing.T) {
	for _, end := range []time.Time{time.Unix(0, 0), time.Unix(100, 37), time.Unix(0, int64(soulsStep)*soulsMeet)} {
		start := fusionStart(end)
		if f, _ := soulsFrame(start); f != soulsMeet {
			t.Errorf("end %v: fusion at the spinner's frame %d, want %d", end, f, soulsMeet)
		}
		if start.Before(end.Truncate(soulsStep)) || start.Sub(end) >= time.Duration(len(soulsFrames))*soulsStep {
			t.Errorf("end %v: fusion at %v, want within one turn of the spinner", end, start)
		}
	}
}
