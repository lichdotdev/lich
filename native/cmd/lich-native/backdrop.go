package main

import (
	"hash/fnv"
	"image"
	"image/color"
	"math"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// The Grain backdrop is a port of the mockup the user approved
// (backdrop-embers.html: hash2, BAYER, fade, effective, VEIL, LAYERS, wisps,
// sparks, emberField and drawKind's grain branch). Every coordinate is the
// pane's own: the mockup offsets its lattice by the island's place in the
// window, so a pane moved or resized would show another field; here the
// pane's seed alone picks it, and the band counts from the pane's top.

// grainSeeds are the 16 seeds the user had drawn at random; a pane takes one
// by its id (grainSeed).
var grainSeeds = [16]int{
	11918, 13134, 16212, 17242, 20156, 31009, 36095, 46317,
	57780, 66777, 68745, 75115, 76708, 83767, 85635, 95744,
}

const (
	// grainTurn and grainAmount were on screen when the user approved the
	// mockup: up to 45 degrees off upright either way, and 1.5 times the
	// lattices' share of cells holding a mark.
	grainTurn   = 45.0
	grainAmount = 1.5
	// grainVeil is the even veil of dots under the marks, at the top.
	grainVeil = 0.07
	// grainSolid is a dot's opacity at Pane strength 1 (KINDS.grain.solid).
	grainSolid = 0.2
	// grainPatch is how much of a letter-sized patch the grain can cover,
	// for the read check (KINDS.grain.patch).
	grainPatch = 0.55
	// grainStrength is Pane strength until Settings sets it.
	grainStrength = 1.0
	// grainBand is the share of the pane, from its top, the band reaches.
	grainBand = 0.5
	// grainHold is fade's exponent for the marks, so they hold lower than
	// the veil.
	grainHold = 0.6
)

// grainCell is the lattice's step; a dot is grainDot, a dp short of it, so
// the lattice shows as dots, not as a fill.
const (
	grainCell unit.Dp = 3
	grainDot  unit.Dp = 2
)

type grainLayer struct {
	step   int
	share  float64
	h0, h1 float64
	salt   int
	ox, oy int
}

// grainLayers are the mockup's LAYERS: big marks, small ones, and, past
// amount 1, smaller ones filling in at (amount-1)*0.9.
var grainLayers = [...]grainLayer{
	{step: 22, share: min(1, 0.85*grainAmount), h0: 14, h1: 8, salt: 60},
	{step: 17, share: min(1, 0.55*grainAmount), h0: 10, h1: 5, salt: 360, ox: 9, oy: 13},
	{step: 13, share: min(1, (grainAmount-1)*0.9), h0: 8, h1: 4, salt: 160, ox: 5, oy: 4},
}

const (
	sparkStep  = 11
	sparkShare = 0.12
	sparkSalt  = 260
	// seedSpread is how far apart two seeds put their hashes (SEED * 131).
	seedSpread = 131
)

// grainSeed is pane id's seed: the same on every run, so a pane shows the
// same field after a resize and a restart.
func grainSeed(id string) int {
	h := fnv.New32a()
	h.Write([]byte(id))
	return grainSeeds[h.Sum32()%uint32(len(grainSeeds))]
}

// hash2 is the mockup's, in the uint32 arithmetic Math.imul and >>> do.
func hash2(x, y, s int) float64 {
	h := uint32(int32(x))*374761393 + uint32(int32(y))*668265263 + uint32(int32(s))*2246822519
	h = (h ^ h>>13) * 1274126177
	return float64(h^h>>16) / 4294967296
}

// bayer8 is the 8x8 Bayer threshold, built by its recursion, each entry
// centered in its 1/64 step.
var bayer8 = func() (m [8][8]float64) {
	k := [][]int{{0}}
	for n := 1; n < 8; n *= 2 {
		next := make([][]int, 2*n)
		for y := range next {
			next[y] = make([]int, 2*n)
			for x := range next[y] {
				q := [2][2]int{{0, 2}, {3, 1}}[y/n][x/n]
				next[y][x] = 4*k[y%n][x%n] + q
			}
		}
		k = next
	}
	for y := range m {
		for x := range m[y] {
			m[y][x] = (float64(k[y][x]) + 0.5) / 64
		}
	}
	return m
}()

// grainFade is how much of the backdrop shows at y, the share of the way
// down the pane: gone by the band's end.
func grainFade(y float64) float64 {
	if y >= grainBand {
		return 0
	}
	f := 1 - y/grainBand
	return f * f
}

// mark is one lattice cell's flame mark, resolved once per paint: whether the
// cell holds one and, if so, where its foot is and how it leans.
type mark struct {
	on               bool
	h, ca, sa        float64
	fx, fy           float64
	side, bend, wide float64
}

func newMark(l grainLayer, lx, ly, seed int) mark {
	r := func(n int) float64 { return hash2(lx, ly, l.salt+n+seed*seedSpread) }
	if r(0) > l.share {
		return mark{}
	}
	ang := (r(9) - 0.5) * 2 * grainTurn * math.Pi / 180
	side := 1.0
	if r(5) < 0.5 {
		side = -1
	}
	return mark{
		on: true, h: l.h0 + r(1)*l.h1, ca: math.Cos(ang), sa: math.Sin(ang),
		fx: (float64(lx) + r(3)) * float64(l.step), fy: (float64(ly) + r(4)) * float64(l.step),
		side: side, bend: 0.85 + 0.3*r(6), wide: 0.2 + 0.04*r(2),
	}
}

// covers is whether the mark covers lattice point x, y: a fat foot that curls
// into a hooked point, a shorter tongue forking off the other side, and a
// small spur under the hook, each a stroke tapering to a point.
func (m mark) covers(x, y float64) bool {
	dx, dy := x-m.fx, y-m.fy
	// every stroke stays within 0.8 of the mark's height from its foot
	if dx*dx+dy*dy > m.h*m.h {
		return false
	}
	across, v := dx*m.ca+dy*m.sa, 0.5-(dy*m.ca-dx*m.sa)/m.h
	if v < 0 || v > 1 {
		return false
	}
	q, k := across*m.side/m.h, m.bend
	// mid, the stroke's line across the mark, is c0 + c2*w*w at w of the way along
	tongue := func(v0, v1, c0, c2, wide float64) bool {
		if v < v0 || v > v1 {
			return false
		}
		w := (v - v0) / (v1 - v0)
		off := math.Abs(q - (c0 + c2*w*w))
		// the taper only narrows the stroke, so a point past wide is out
		return off <= wide && off <= wide*math.Pow(1-w, 0.8)*math.Sqrt(min(1, w/0.14))
	}
	return tongue(0, 1, -0.05*k, 0.36*k, m.wide) || tongue(0.06, 0.66, -0.1, -0.3*k, 0.13) || tongue(0.02, 0.42, 0.19, 0.26*k, 0.085)
}

// markGrid is one layer's marks over the lattice cells a pane reaches.
type markGrid struct {
	l          grainLayer
	x0, y0, nx int
	marks      []mark
}

func newMarkGrid(l grainLayer, cols, rows, seed int) markGrid {
	x0, y0 := (l.ox)/l.step-2, (l.oy)/l.step-2
	nx, ny := (cols-1+l.ox)/l.step+3-x0, (rows-1+l.oy)/l.step+3-y0
	g := markGrid{l: l, x0: x0, y0: y0, nx: nx, marks: make([]mark, nx*ny)}
	for j := range ny {
		for i := range nx {
			g.marks[j*nx+i] = newMark(l, x0+i, y0+j, seed)
		}
	}
	return g
}

func (g markGrid) covers(x, y int) bool {
	X, Y := x+g.l.ox, y+g.l.oy
	cx, cy := X/g.l.step, Y/g.l.step
	for b := -2; b <= 2; b++ {
		for a := -2; a <= 2; a++ {
			m := g.marks[(cy+b-g.y0)*g.nx+cx+a-g.x0]
			if m.on && m.covers(float64(X), float64(Y)) {
				return true
			}
		}
	}
	return false
}

// spark is a loose ember between the marks: a round dot of its own density.
func spark(x, y, seed int) float64 {
	cx, cy, best := x/sparkStep, y/sparkStep, 0.0
	s := sparkSalt + seed*seedSpread
	for b := -1; b <= 1; b++ {
		for a := -1; a <= 1; a++ {
			lx, ly := cx+a, cy+b
			if hash2(lx, ly, s) > sparkShare {
				continue
			}
			px, py := (float64(lx)+hash2(lx, ly, s+1))*sparkStep, (float64(ly)+hash2(lx, ly, s+2))*sparkStep
			if math.Hypot(float64(x)-px, float64(y)-py)/(0.8+0.8*hash2(lx, ly, s+3)) < 1 {
				best = max(best, 0.6+0.35*hash2(lx, ly, s+4))
			}
		}
	}
	return best
}

// grainField is the embers' density at every lattice point of a cols x rows
// pane, read through ember(x, y).
type grainField struct {
	seed  int
	grids [len(grainLayers)]markGrid
}

func newGrainField(cols, rows, seed int) *grainField {
	f := &grainField{seed: seed}
	for i, l := range grainLayers {
		f.grids[i] = newMarkGrid(l, cols, rows, seed)
	}
	return f
}

func (f *grainField) ember(x, y int) float64 {
	for _, g := range f.grids {
		if g.covers(x, y) {
			return 1
		}
	}
	return spark(x, y, f.seed)
}

// grainImage is the band of a pane size device pixels big, its dots cell
// apart and dot wide, in ink, transparent between them. Only the band is
// stored: below it nothing shows.
func grainImage(size image.Point, cell, dot int, ink color.NRGBA, seed int) *image.RGBA {
	cols, rows := ceilDiv(size.X, cell), ceilDiv(size.Y, cell)
	band := min(rows, ceilDiv(int(math.Ceil(float64(size.Y)*grainBand)), cell))
	img := image.NewRGBA(image.Rect(0, 0, size.X, min(size.Y, band*cell)))
	if cols == 0 || band == 0 {
		return img
	}
	field := newGrainField(cols, rows, seed)
	for j := range band {
		f := grainFade((float64(j*cell) + float64(cell)/2) / float64(size.Y))
		if f <= 0 {
			continue
		}
		veil, hold := f*grainVeil, math.Pow(f, grainHold)
		for i := range cols {
			t := bayer8[j%8][i%8]
			// the field is at most 1, so it is read only where it can lift a dot
			if veil > t || hold > t && hold*field.ember(i, j) > t {
				dotAt(img, image.Rect(i*cell, j*cell, i*cell+dot, j*cell+dot), ink)
			}
		}
	}
	return img
}

func dotAt(img *image.RGBA, r image.Rectangle, c color.NRGBA) {
	r = r.Intersect(img.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := img.Pix[img.PixOffset(r.Min.X, y):]
		for x := 0; x < r.Dx(); x++ {
			row[4*x], row[4*x+1], row[4*x+2], row[4*x+3] = c.R, c.G, c.B, 255
		}
	}
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

// grainAlpha is a dot's opacity on ground: grainSolid at the strength
// asked, lowered until text still reads over it (the mockup's effective).
// The foreground keeps 4.5:1 over a letter-sized patch the grain covers by
// grainPatch, the muted foreground 3:1, and 1.5:1 over a single dot, each
// capped at nine tenths of what the bare ground gives.
func grainAlpha(fg, dim, ground color.NRGBA, strength float64) float64 {
	need := func(t color.NRGBA, want float64) float64 { return min(want, 0.9*contrast(t, ground)) }
	holds := func(s float64) bool {
		a := min(1, grainSolid*s)
		patch, dot := ui.Over(ground, ui.Alpha(dim, float32(a*grainPatch))), ui.Over(ground, ui.Alpha(dim, float32(a)))
		return contrast(fg, patch) >= need(fg, 4.5) && contrast(dim, patch) >= need(dim, 3) && contrast(dim, dot) >= need(dim, 1.5)
	}
	s := strength
	if !holds(s) {
		lo, hi := 0.0, s
		for range 24 {
			if mid := (lo + hi) / 2; holds(mid) {
				lo = mid
			} else {
				hi = mid
			}
		}
		s = math.Floor(lo*100) / 100
	}
	return min(1, grainSolid*s)
}

// backdropKey is what a pane's backdrop is painted from; any change paints
// it again.
type backdropKey struct {
	size      image.Point
	cell, dot int
	ink       color.NRGBA
	seed      int
}

type backdropEntry struct {
	key  backdropKey
	op   paint.ImageOp
	size image.Point
	// pending is the size a resize asked for, painted once a frame sees it
	// again.
	pending image.Point
	seen    bool
}

// dockBackdropID seeds the dock's backdrop: one dock, one field.
const dockBackdropID = "dock"

// backdrops holds each pane's painted backdrop between frames.
type backdrops struct {
	byID map[string]*backdropEntry
	// painted counts paints, for the tests.
	painted int
}

// backdropLayer is a pane's backdrop as its content sees it: the image and
// where its top-left lies from the content's own origin.
type backdropLayer struct {
	op paint.ImageOp
	at image.Point
	ok bool
}

// below is the layer as content dy lower down sees it.
func (l backdropLayer) below(dy int) backdropLayer {
	l.at.Y -= dy
	return l
}

// sweep forgets the backdrops no frame drew since the last sweep: a pane
// closed, or out of sight.
func (b *backdrops) sweep() {
	for id, e := range b.byID {
		if !e.seen {
			delete(b.byID, id)
		}
		e.seen = false
	}
}

// layout paints pane id's backdrop, size big on ground, at gtx's origin, and
// returns it for content that paints its own ground (the terminal) to lay
// under its text. It is the one place a pane's backdrop is chosen.
func (b *backdrops) layout(gtx C, th *ui.Theme, id string, size image.Point, ground color.NRGBA) backdropLayer {
	e := b.entry(gtx, th, id, size, ground)
	if e.size.Y == 0 {
		return backdropLayer{}
	}
	defer clip.Rect{Max: image.Pt(min(e.size.X, size.X), min(e.size.Y, size.Y))}.Push(gtx.Ops).Pop()
	e.op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return backdropLayer{op: e.op, ok: true}
}

func (b *backdrops) entry(gtx C, th *ui.Theme, id string, size image.Point, ground color.NRGBA) *backdropEntry {
	a := grainAlpha(ui.Over(ground, th.Foreground), ui.Over(ground, th.MutedForeground), ground, grainStrength)
	key := backdropKey{
		size: size, cell: max(1, gtx.Dp(grainCell)), dot: max(1, gtx.Dp(grainDot)),
		ink: ui.Over(ground, ui.Alpha(ui.Over(ground, th.MutedForeground), float32(a))), seed: grainSeed(id),
	}
	if b.byID == nil {
		b.byID = map[string]*backdropEntry{}
	}
	e, ok := b.byID[id]
	var resized backdropKey
	if ok {
		resized = e.key
		resized.size = size
	}
	// A pane being dragged or animated changes size every frame, and a paint
	// takes longer than one: it keeps its old field until a frame finds the
	// size it asked about.
	if ok && e.key != key && resized == key && e.pending != size {
		e.pending = size
		gtx.Execute(op.InvalidateCmd{})
	} else if !ok || e.key != key {
		img := grainImage(size, key.cell, key.dot, key.ink, key.seed)
		e = &backdropEntry{key: key, op: paint.NewImageOp(img), size: img.Rect.Size()}
		b.byID[id] = e
		b.painted++
	}
	e.seen = true
	return e
}
