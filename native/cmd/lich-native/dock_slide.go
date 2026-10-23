package main

import (
	"time"

	"gioui.org/op"
)

// dockSlide eases the dock in from the window's right edge and back out, on
// the sidebar morph's 180ms curve. The dock keeps its width while it moves,
// so only the island beside it changes size.
type dockSlide struct {
	open  bool
	from  float32 // the share shown when the current slide began
	start time.Time
	shown float32
	// began and landed are whether this frame started or ended a slide.
	began, landed bool
}

// at returns the share of the dock on screen this frame, 0 to 1, starting a
// slide toward open when the target changed; a slide reversed halfway turns
// back from where it is.
func (s *dockSlide) at(gtx C, open bool) float32 {
	s.began = open != s.open
	if s.began {
		s.open, s.from, s.start = open, s.shown, gtx.Now
	}
	target := float32(0)
	if open {
		target = 1
	}
	t := float32(gtx.Now.Sub(s.start)) / float32(sidebarMorphDuration)
	if t >= 1 {
		s.landed = s.shown != target
		s.shown = target
		return s.shown
	}
	s.landed = false
	s.shown = s.from + (target-s.from)*sidebarMorphCurve(max(t, 0))
	gtx.Execute(op.InvalidateCmd{})
	return s.shown
}

// sliding is whether the last frame caught the slide between its ends.
func (s *dockSlide) sliding() bool { return s.shown > 0 && s.shown < 1 }

// snap ends any slide at open, for the full-screen dock, which covers the
// terminal rather than sliding beside it.
func (s *dockSlide) snap(open bool) {
	s.open, s.shown = open, 0
	if open {
		s.shown = 1
	}
}
