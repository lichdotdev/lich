package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/ui"
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

func TestEachStatusPaintsInItsOwnPalette(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Surface: ui.Dark.Sidebar}
	if c, _ := soulsInk(th, '4'); c != th.Primary {
		t.Errorf("a soul's head %v, want the theme's Primary %v", c, th.Primary)
	}
	if c, _ := glassInk(th, '4'); c != th.ToneWait {
		t.Errorf("the sand %v, want ToneWait %v", c, th.ToneWait)
	}
	if c, _ := checkInk(th, '7'); c != th.TonePass {
		t.Errorf("the check %v, want TonePass %v", c, th.TonePass)
	}
	for _, s := range []statusSprite{hourglassSprite, glassEnter, checkFusion} {
		if s.ink == nil {
			t.Fatal("a status sprite has no palette")
		}
		for _, f := range s.frames {
			for _, c := range strings.ReplaceAll(strings.ReplaceAll(f, " | ", ""), ".", "") {
				if _, ok := s.ink(th, byte(c)); !ok {
					t.Errorf("symbol %q has no color in its sprite's palette", c)
				}
			}
		}
	}
}
