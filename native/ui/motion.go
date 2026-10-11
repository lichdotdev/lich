package ui

import (
	"time"

	"gioui.org/op"
)

// Timing functions by their CSS names.
var (
	easeOut = CubicBezier(0, 0, 0.2, 1) // Tailwind ease-out
	easeIn  = CubicBezier(0.4, 0, 1, 1) // Tailwind ease-in
)

// CubicBezier is CSS's cubic-bezier(x1, y1, x2, y2): it solves x(u) = x for
// u by bisection, then returns y(u).
func CubicBezier(x1, y1, x2, y2 float32) func(float32) float32 {
	axis := func(u, c1, c2 float32) float32 {
		return 3*(1-u)*(1-u)*u*c1 + 3*(1-u)*u*u*c2 + u*u*u
	}
	return func(x float32) float32 {
		lo, hi := float32(0), float32(1)
		for range 24 {
			mid := (lo + hi) / 2
			if axis(mid, x1, x2) < x {
				lo = mid
			} else {
				hi = mid
			}
		}
		return axis((lo+hi)/2, y1, y2)
	}
}

// tween is a CSS transition of one number: a new target restarts it from
// where it stands, over the duration and curve given with that target.
type tween struct {
	from, to float32
	start    time.Time
	duration time.Duration
	curve    func(float32) float32
	seeded   bool
}

// at advances the tween to gtx.Now, headed for target, and asks for another
// frame while it moves. The first call lands on target without moving: a
// widget drawn for the first time is not a change.
func (t *tween) at(gtx C, target float32, duration time.Duration, curve func(float32) float32) float32 {
	if !t.seeded {
		t.seeded, t.from, t.to = true, target, target
	}
	if target != t.to {
		t.from, t.to = t.value(gtx.Now), target
		t.start, t.duration, t.curve = gtx.Now, duration, curve
	}
	v := t.value(gtx.Now)
	if v != t.to {
		gtx.Execute(op.InvalidateCmd{})
	}
	return v
}

// seed puts the tween at v without moving, so the next target animates from
// there: an item that enters from nothing.
func (t *tween) seed(v float32) { t.seeded, t.from, t.to = true, v, v }

func (t *tween) value(now time.Time) float32 {
	if t.from == t.to {
		return t.to
	}
	p := float32(now.Sub(t.start)) / float32(t.duration)
	if p >= 1 {
		t.from = t.to
		return t.to
	}
	return t.from + (t.to-t.from)*t.curve(max(p, 0))
}
