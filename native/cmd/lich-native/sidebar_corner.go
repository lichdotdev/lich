package main

import (
	"image"
	"image/color"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// cornerMode is what a session's status corner is doing. Work, wait and done
// are the statuses it shows; fill and drain carry the snake into the block
// and back out of it.
type cornerMode uint8

const (
	cornerWork cornerMode = iota
	cornerWait
	cornerDone
	cornerFill
	cornerDrain
)

const (
	allCells uint16 = 1<<cellCount - 1
	// toneBlend is how long the corner's color takes to cross into a new status's.
	toneBlend = 380 * time.Millisecond
	// cornerSettle is how long the block glides once it is whole; past it a
	// finished corner asks for no frame.
	cornerSettle = 4 * cellGlide
	// cornerCatchUp is the most steps a corner replays after it was not drawn,
	// under the pointer or out of sight; past it it skips to now.
	cornerCatchUp = 32
)

// corner is one session's status corner. The snake never restarts: a new
// status waits for the step where the motion can carry on into it, so the
// corner keeps a history the clock alone cannot give.
type corner struct {
	mode cornerMode
	// i is the head's step on spinnerPath, or on ringPath while waiting.
	i int
	// egg is the head's step on eggSteps while the snake writes lich, -1
	// otherwise.
	egg int
	// trail is the head and the cells behind it, newest first; -1 is none.
	trail [spinnerTrail]int
	// keep is the cells lit apart from the trail while filling or draining.
	keep uint16
	// at is when the current step started, and from the cells' brightness then.
	at   time.Time
	from cellAlphas
	// tone is the status the color belongs to, crossing from toneFrom since toneAt.
	tone, toneFrom cornerMode
	toneAt         time.Time
}

// newCorner is a corner first drawn at now: the status as it is, with no
// motion into it.
func newCorner(mode cornerMode, now time.Time) *corner {
	c := &corner{mode: mode, i: -1, egg: -1, trail: [spinnerTrail]int{-1, -1, -1}, at: now, tone: mode, toneFrom: mode}
	c.from = c.levels().faded()
	return c
}

func (c *corner) stepLen() time.Duration {
	if c.mode == cornerWait {
		return ringStep
	}
	return spinnerStep
}

// advance plays the corner's steps up to now, heading for want.
func (c *corner) advance(now time.Time, want cornerMode) {
	if c.mode == cornerDone {
		if want == cornerDone {
			return
		}
		// The block has been still: its drain starts now, not on a step long gone.
		c.from, c.at = c.alphas(now), now
		c.step(want, now)
	}
	for n := 0; ; n++ {
		next := c.at.Add(c.stepLen())
		if now.Before(next) {
			return
		}
		if n == cornerCatchUp {
			c.at = now
			return
		}
		c.from, c.at = c.alphas(next), next
		c.step(want, next)
	}
}

func (c *corner) step(want cornerMode, at time.Time) {
	c.turn(want, at)
	if c.mode == cornerDone {
		return
	}
	if c.mode == cornerWait {
		c.i = (c.i + 1) % len(ringPath)
		c.push(ringPath[c.i])
	} else {
		c.walk(at)
	}
	switch c.mode {
	case cornerFill:
		c.keep |= 1 << c.trail[0]
		if c.keep == allCells {
			c.mode = cornerDone
		}
	case cornerDrain:
		c.keep &^= 1 << c.trail[0]
		if c.keep == 0 {
			c.mode = cornerWork
		}
	}
}

// walk moves the head a step along the snake's lap, or along the egg's path
// while it runs; an egg starts only where a lap would.
func (c *corner) walk(at time.Time) {
	switch {
	case c.egg >= 0 && c.egg+1 < len(eggSteps):
		c.egg++
		c.push(eggSteps[c.egg].head)
		return
	case c.egg >= 0:
		c.egg, c.i = -1, pathIndex(c.trail[0])
	case c.mode == cornerWork && c.i == len(spinnerPath)-1 && eggStarts(at):
		c.egg = 0
		c.push(eggSteps[0].head)
		return
	}
	c.i = (c.i + 1) % len(spinnerPath)
	c.push(spinnerPath[c.i])
}

func (c *corner) push(cell int) {
	copy(c.trail[1:], c.trail[:spinnerTrail-1])
	c.trail[0] = cell
}

// turn moves toward want where the motion allows it: the ring takes over the
// snake on any border cell and hands it back on the cell the snake starts
// from; the snake fills the block from wherever it is, and drains it from the
// start of its path.
func (c *corner) turn(want cornerMode, at time.Time) {
	head := c.trail[0]
	switch {
	case c.mode == cornerWork && want == cornerWait && head >= 0 && head != cellCenter:
		c.mode, c.i = cornerWait, ringIndex(head)
	case c.mode == cornerWait && want == cornerWork && head == spinnerPath[0]:
		c.mode, c.i = cornerWork, 0
	case (c.mode == cornerWork || c.mode == cornerWait) && want == cornerDone:
		if c.mode == cornerWait || c.egg >= 0 {
			c.i = pathIndex(head)
		}
		c.mode, c.keep = cornerFill, c.trailCells()
	case (c.mode == cornerDone || c.mode == cornerFill) && want != cornerDone:
		if c.mode == cornerDone {
			c.keep, c.i, c.trail = allCells, -1, [spinnerTrail]int{-1, -1, -1}
		}
		c.mode = cornerDrain
	default:
		return
	}
	c.egg = -1
	if t := cornerTone(c.mode); t != c.tone {
		c.toneFrom, c.tone, c.toneAt = c.tone, t, at
	}
}

func (c *corner) trailCells() uint16 {
	var cells uint16
	for _, cell := range c.trail {
		if cell >= 0 {
			cells |= 1 << cell
		}
	}
	return cells
}

func ringIndex(cell int) int {
	for i, c := range ringPath {
		if c == cell {
			return i
		}
	}
	return -1
}

func pathIndex(cell int) int {
	for i, c := range spinnerPath {
		if c == cell {
			return i
		}
	}
	return -1
}

// cornerTone is the status whose color a mode wears.
func cornerTone(m cornerMode) cornerMode {
	switch m {
	case cornerFill, cornerDone:
		return cornerDone
	case cornerDrain:
		return cornerWork
	}
	return m
}

func (c *corner) levels() cellLevels {
	var l cellLevels
	if c.mode == cornerDone {
		for i := range l {
			l[i] = spinnerTrail
		}
		return l
	}
	// The block's cells are full while it drains, and a step under the head
	// while it fills, so the head still leads.
	kept := uint8(spinnerTrail - 1)
	if c.mode == cornerDrain {
		kept = spinnerTrail
	}
	for cell := range cellCount {
		if c.keep&(1<<cell) != 0 {
			l[cell] = kept
		}
	}
	if c.egg >= 0 {
		l.lightGhost(eggSteps[c.egg].ghost)
	}
	l.lightTrail(c.trail[:])
	return l
}

func (c *corner) alphas(now time.Time) cellAlphas {
	return glide(c.from, c.levels(), now.Sub(c.at))
}

// color is the corner's tone at now, crossing from the last status's.
func (c *corner) color(th *ui.Theme, now time.Time) color.NRGBA {
	return blendTone(th, c.toneFrom, c.tone, now.Sub(c.toneAt))
}

// blendTone is the color of a status crossing from from's to to's, since into the crossing.
func blendTone(th *ui.Theme, from, to cornerMode, since time.Duration) color.NRGBA {
	if from == to {
		return cornerColor(th, to)
	}
	return ui.MixOKLab(cornerColor(th, from), cornerColor(th, to), float64(treeEase(float32(since)/float32(toneBlend))))
}

// cornerColor is a status's color: the theme's Primary while it works,
// ToneWait while it waits on you, TonePass for a finished turn.
func cornerColor(th *ui.Theme, m cornerMode) color.NRGBA {
	switch m {
	case cornerWait:
		return th.ToneWait
	case cornerDone:
		return th.TonePass
	}
	return th.Primary
}

// due is when the corner needs its next frame, false for a whole block
// that has stopped moving.
func (c *corner) due(now time.Time) (time.Time, bool) {
	if c.mode == cornerDone {
		if now.Sub(c.at) >= cornerSettle && now.Sub(c.toneAt) >= toneBlend {
			return time.Time{}, false
		}
		return now.Add(glideFrame), true
	}
	return earliest(c.at.Add(c.stepLen()), now.Add(glideFrame)), true
}

// cornerWant is the mode a session's status shows, false for one with no
// corner: the snake while it works, the ring while it waits on you, and the
// block for a finished turn nobody read.
func cornerWant(sv *sessionView) (cornerMode, bool) {
	switch {
	case spins(sv.status):
		return cornerWork, true
	case sv.status == statusWaiting:
		return cornerWait, true
	case sv.status == statusDone && sv.unread:
		return cornerDone, true
	}
	return 0, false
}

// statusCorner draws what a session's corner shows at rest, false for a
// status it has nothing for. A session's corner lives as long as it has a
// status, so a change plays out of what was on screen; one first drawn shows
// its status at once.
func (s *sidebar) statusCorner(gtx C, th *ui.Theme, sv *sessionView) (D, bool) {
	c, ok := s.cornerAt(gtx, th, sv)
	if !ok {
		return D{}, false
	}
	return drawCorner(gtx, th.Surface, c.color(th, gtx.Now), c.alphas(gtx.Now)), true
}

// cornerAt is the session's corner advanced to the frame, false for a status
// it has nothing for. The card and the rail's badge share it, so a session
// keeps its motion across the morph.
func (s *sidebar) cornerAt(gtx C, th *ui.Theme, sv *sessionView) (*corner, bool) {
	want, ok := cornerWant(sv)
	if !ok {
		delete(s.corners, sv.ID)
		return nil, false
	}
	c := s.corners[sv.ID]
	if c == nil {
		c = newCorner(want, gtx.Now)
		s.corners[sv.ID] = c
	}
	c.advance(gtx.Now, want)
	if at, ok := c.due(gtx.Now); ok {
		nextFrame(gtx, th, at.Sub(gtx.Now))
	}
	return c, true
}

// railBadge is the session's corner on a rail node's bottom right, at
// railBadgeSize on a plate of the node's ground that cuts it off the
// provider mark under it.
func (s *sidebar) railBadge(gtx C, th *ui.Theme, sv *sessionView, ground color.NRGBA) D {
	box := gtx.Constraints.Min
	c, ok := s.cornerAt(gtx, th, sv)
	if !ok {
		return D{Size: box}
	}
	side := gtx.Dp(railBadgeSize)
	plate := side + 2*gtx.Dp(railBadgePlate)
	at := box.Sub(image.Pt(plate, plate))
	defer op.Offset(at).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, ground, clip.UniformRRect(image.Rectangle{Max: image.Pt(plate, plate)}, gtx.Dp(railBadgePlate)*2).Op(gtx.Ops))
	defer op.Offset(image.Pt(gtx.Dp(railBadgePlate), gtx.Dp(railBadgePlate))).Push(gtx.Ops).Pop()
	drawCells(gtx, side, ground, c.color(th, gtx.Now), c.alphas(gtx.Now))
	return D{Size: box}
}

// railBadgeSize is the rail's corner: small enough to leave the provider
// mark readable, large enough that the snake still reads as one.
var (
	railBadgeSize  = unit.Dp(12)
	railBadgePlate = unit.Dp(1.5)
)

// nextFrame asks for the frame after left, or a still window's slower look.
func nextFrame(gtx C, th *ui.Theme, left time.Duration) {
	at := gtx.Now.Add(left)
	if th.Still {
		at = gtx.Now.Add(ui.StillFrame)
	}
	gtx.Execute(op.InvalidateCmd{At: at})
}
