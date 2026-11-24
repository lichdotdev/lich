package main

import (
	"image"
	"image/color"
	"time"

	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/ui"
)

// tabRun is the dot a working tab's head steps on: left to right, 3 and 4
// past the edge so the tail leaves before the head comes back.
var tabRun = [...]int{0, 1, 2, 3, 4}

// tabPong is a waiting tab's dots: there and back, never jumping ends.
var tabPong = [...]int{0, 1, 2, 1}

const (
	dotCount = 3
	allDots  = 1<<dotCount - 1
	// dotRunStep and dotPongStep differ so waiting reads apart from working by
	// its pace as well as its color.
	dotRunStep  = 110 * time.Millisecond
	dotPongStep = 160 * time.Millisecond
)

type dotAlphas [dotCount]float32

// tabDots is a project tab's status: the sidebar corner's rules on three
// dots. The head never restarts, a new status waits for the step where the
// motion can carry on into it, and brightness and color glide between them.
type tabDots struct {
	mode  cornerMode
	i     int // the head's step on tabRun, or on tabPong while waiting
	trail [spinnerTrail]int
	keep  uint8 // dots lit apart from the trail while filling or draining
	at    time.Time
	from  dotAlphas
	// tone is the status the color belongs to, crossing from toneFrom since toneAt.
	tone, toneFrom cornerMode
	toneAt         time.Time
}

func newTabDots(mode cornerMode, now time.Time) *tabDots {
	d := &tabDots{mode: mode, i: -1, trail: [spinnerTrail]int{-1, -1, -1}, at: now, tone: mode, toneFrom: mode}
	d.from = d.faded()
	return d
}

func (d *tabDots) stepLen() time.Duration {
	if d.mode == cornerWait {
		return dotPongStep
	}
	return dotRunStep
}

// advance plays the dots' steps up to now, heading for want.
func (d *tabDots) advance(now time.Time, want cornerMode) {
	if d.mode == cornerDone {
		if want == cornerDone {
			return
		}
		d.from, d.at = d.alphas(now), now
		d.step(want, now)
	}
	for n := 0; ; n++ {
		next := d.at.Add(d.stepLen())
		if now.Before(next) {
			return
		}
		if n == cornerCatchUp {
			d.at = now
			return
		}
		d.from, d.at = d.alphas(next), next
		d.step(want, next)
	}
}

func (d *tabDots) step(want cornerMode, at time.Time) {
	d.turn(want, at)
	if d.mode == cornerDone {
		return
	}
	if d.mode == cornerWait {
		d.i = (d.i + 1) % len(tabPong)
		d.push(tabPong[d.i])
	} else {
		d.i = (d.i + 1) % len(tabRun)
		d.push(tabRun[d.i])
	}
	head := d.trail[0]
	if head >= dotCount {
		return
	}
	switch d.mode {
	case cornerFill:
		d.keep |= 1 << head
		if d.keep == allDots {
			d.mode = cornerDone
		}
	case cornerDrain:
		d.keep &^= 1 << head
		if d.keep == 0 {
			d.mode = cornerWork
		}
	}
}

func (d *tabDots) push(dot int) {
	copy(d.trail[1:], d.trail[:spinnerTrail-1])
	d.trail[0] = dot
}

// turn moves toward want where the motion allows it: the run turns back as the
// pong while its head heads right on a dot, and the pong leaves through the
// right edge as the run once it reaches it; the run fills the dots from where
// it is, and drains them from the left.
func (d *tabDots) turn(want cornerMode, at time.Time) {
	head := d.trail[0]
	switch {
	case d.mode == cornerWork && want == cornerWait && head >= 0 && head < dotCount-1:
		d.mode, d.i = cornerWait, head
	case d.mode == cornerWait && want == cornerWork && head == dotCount-1:
		d.mode, d.i = cornerWork, dotCount-1
	case (d.mode == cornerWork || d.mode == cornerWait) && want == cornerDone:
		if d.mode == cornerWait {
			d.i = head
		}
		// Only the head stays: on three dots the whole trail would light at once.
		d.mode, d.keep = cornerFill, 0
		if head >= 0 && head < dotCount {
			d.keep = 1 << head
		}
	case (d.mode == cornerDone || d.mode == cornerFill) && want != cornerDone:
		if d.mode == cornerDone {
			d.keep, d.i, d.trail = allDots, -1, [spinnerTrail]int{-1, -1, -1}
		}
		d.mode = cornerDrain
	default:
		return
	}
	if t := cornerTone(d.mode); t != d.tone {
		d.toneFrom, d.tone, d.toneAt = d.tone, t, at
	}
}

func (d *tabDots) levels() [dotCount]uint8 {
	var l [dotCount]uint8
	if d.mode == cornerDone {
		return [dotCount]uint8{spinnerTrail, spinnerTrail, spinnerTrail}
	}
	kept := uint8(spinnerTrail - 1)
	if d.mode == cornerDrain {
		kept = spinnerTrail
	}
	for dot := range dotCount {
		if d.keep&(1<<dot) != 0 {
			l[dot] = kept
		}
	}
	for k, dot := range d.trail {
		if dot >= 0 && dot < dotCount {
			l[dot] = max(l[dot], uint8(spinnerTrail-k))
		}
	}
	return l
}

func (d *tabDots) faded() dotAlphas {
	var a dotAlphas
	for i, l := range d.levels() {
		a[i] = cellFade[l]
	}
	return a
}

func (d *tabDots) alphas(now time.Time) dotAlphas {
	a := d.from
	for i, l := range d.levels() {
		a[i] = glideCell(a[i], l, now.Sub(d.at))
	}
	return a
}

// due is when the dots need their next frame, false once a finished tab's
// dots have stopped moving.
func (d *tabDots) due(now time.Time) (time.Time, bool) {
	if d.mode == cornerDone {
		if now.Sub(d.at) >= cornerSettle && now.Sub(d.toneAt) >= toneBlend {
			return time.Time{}, false
		}
		return now.Add(glideFrame), true
	}
	return earliest(d.at.Add(d.stepLen()), now.Add(glideFrame)), true
}

// tabWant is the mode a project's status shows on its tab, false for none.
func tabWant(status string) (cornerMode, bool) {
	switch {
	case spins(status):
		return cornerWork, true
	case status == statusWaiting:
		return cornerWait, true
	case status == statusDone:
		return cornerDone, true
	}
	return 0, false
}

// tabBadge is ProjectTab's status, its three dots, nothing for a tab with
// nothing to say. The dots live in the tab's state for as long as it has a
// status, so a change plays out of what was on screen.
func (t *topbar) tabBadge(gtx C, th *ui.Theme, st *tabState, status string) D {
	want, ok := tabWant(status)
	if !ok {
		st.dots = nil
		return D{}
	}
	if st.dots == nil {
		st.dots = newTabDots(want, gtx.Now)
	}
	d := st.dots
	d.advance(gtx.Now, want)
	if at, ok := d.due(gtx.Now); ok {
		nextFrame(gtx, th, at.Sub(gtx.Now))
	}
	side := gtx.Dp(tabGlyph)
	drawDots(gtx, side, th, blendTone(th, d.toneFrom, d.tone, gtx.Now.Sub(d.toneAt)), d.alphas(gtx.Now))
	return D{Size: image.Pt(side, side)}
}

// drawDots paints the three dots across a square of side pixels, centred on
// its height: dots of three sprite pixels with one of gap, as the corner's cells.
func drawDots(gtx C, side int, th *ui.Theme, tone color.NRGBA, a dotAlphas) {
	edge := func(i int) int { return (i*side + spriteCells/2) / spriteCells }
	top := (side - edge(3)) / 2
	for dot, alpha := range a {
		x := dot * 4
		ink := ui.MixOKLab(th.Surface, tone, float64(alpha))
		paint.FillShape(gtx.Ops, ink, clip.Rect(image.Rect(edge(x), top, edge(x+3), top+edge(3))).Op())
	}
}
