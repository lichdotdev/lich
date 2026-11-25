package main

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// spinnerPath is the cells of the 3x3 the working snake's head walks, 0 top
// left, row by row: down the left column, up the middle, down the right, then
// back along the bottom row, the middle and the top to where it started.
var spinnerPath = [...]int{0, 3, 6, 7, 4, 1, 2, 5, 8, 7, 6, 3, 4, 5, 2, 1}

// ringPath is the border of the 3x3 clockwise from the top left: the cells a
// session waiting on you circles, the center left out.
var ringPath = [...]int{0, 1, 2, 5, 8, 7, 6, 3}

const (
	cellCount  = 9
	cellCenter = 4
	// spinnerTrail is how many cells, the head among them, stay lit behind it,
	// each a level fainter.
	spinnerTrail = 3
	spinnerStep  = 90 * time.Millisecond
	// ringStep is slower than the snake, so waiting differs from working by its
	// pace as well as its color.
	ringStep = 110 * time.Millisecond
	// cellGlide is how fast a cell's brightness closes on its level: about two
	// thirds of the way in this long, so a step reads as motion, not a blink.
	cellGlide = 45 * time.Millisecond
	// glideFrame is how often a gliding spinner is redrawn between its steps.
	glideFrame = 30 * time.Millisecond
	// spriteCells is the spinner's side in sprite pixels: three cells of three
	// with a pixel of gap between them.
	spriteCells = 11
)

// spriteSize is the side the status corner is drawn at: smaller than the
// provider mark's box, so the corner reads as a mark beside the name and not
// as a second icon.
var spriteSize = unit.Dp(16)

// cellFade is how much of its tone a cell keeps over the ground at each level:
// 3 the head, 2 and 1 its tail, 0 a cell at rest.
var cellFade = [spinnerTrail + 1]float32{0.10, 0.35, 0.65, 1}

type (
	cellLevels [cellCount]uint8
	cellAlphas [cellCount]float32
)

func (l cellLevels) faded() cellAlphas {
	var a cellAlphas
	for c, v := range l {
		a[c] = cellFade[v]
	}
	return a
}

// glide is from moved toward to's levels for since.
func glide(from cellAlphas, to cellLevels, since time.Duration) cellAlphas {
	for c := range from {
		from[c] = glideCell(from[c], to[c], since)
	}
	return from
}

// glideCell is a cell's brightness moved from toward level for since.
func glideCell(from float32, level uint8, since time.Duration) float32 {
	g := 1 - float32(math.Exp(-float64(since)/float64(cellGlide)))
	return from + (cellFade[level]-from)*g
}

// lightTrail lights trail, newest first, from the head's level down; -1 is no
// cell.
func (l *cellLevels) lightTrail(trail []int) {
	for k, c := range trail {
		if c >= 0 {
			l[c] = max(l[c], uint8(spinnerTrail-k))
		}
	}
}

// spinnerGlideSteps is how many steps back a spinner read off the clock
// starts its glide: older ones have faded out of it.
const spinnerGlideSteps = 4

// spinnerAt is the working snake read off the clock at now, for a place that
// keeps no history of its own, and when it is next due.
func spinnerAt(now time.Time) (cellAlphas, time.Time) {
	k := floorDiv(now.UnixNano(), int64(spinnerStep))
	start := time.Unix(0, k*int64(spinnerStep))
	a := clockLevels(k - spinnerGlideSteps).faded()
	for j := k - spinnerGlideSteps + 1; j < k; j++ {
		a = glide(a, clockLevels(j), spinnerStep)
	}
	next := start.Add(spinnerStep)
	return glide(a, clockLevels(k), now.Sub(start)), earliest(next, now.Add(glideFrame))
}

// floorDiv rounds toward minus infinity, so a time before the epoch (a
// context that never set Now) still lands on a step.
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func earliest(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// pixelSpinner draws the working snake centered in gtx's minimum size and asks
// for its next frame.
func pixelSpinner(gtx C, th *ui.Theme) D {
	a, next := spinnerAt(gtx.Now)
	nextFrame(gtx, th, next.Sub(gtx.Now))
	return drawCorner(gtx, th.Surface, th.Primary, a)
}

// drawCorner paints the 3x3 at spriteSize, centered in gtx's minimum size,
// each cell tone at its alpha over ground. Each cell's edges land on whole
// device pixels, so it never blurs; at a size 11 does not divide, some cells
// are a pixel wider.
func drawCorner(gtx C, ground, tone color.NRGBA, a cellAlphas) D {
	side := max(spriteCells, gtx.Dp(spriteSize))
	box := gtx.Constraints.Min
	defer op.Offset(image.Pt((box.X-side)/2, (box.Y-side)/2)).Push(gtx.Ops).Pop()
	drawCells(gtx, side, ground, tone, a)
	return D{Size: box}
}

// drawCells paints the 3x3 in a square of side pixels: cells of three sprite
// pixels with one of gap, the sprite's eleven spread over side.
func drawCells(gtx C, side int, ground, tone color.NRGBA, a cellAlphas) {
	edge := func(i int) int { return (i*side + spriteCells/2) / spriteCells }
	for c, alpha := range a {
		x, y := c%3*4, c/3*4
		ink := ui.MixOKLab(ground, tone, float64(alpha))
		paint.FillShape(gtx.Ops, ink, clip.Rect(image.Rect(edge(x), edge(y), edge(x+3), edge(y+3))).Op())
	}
}
