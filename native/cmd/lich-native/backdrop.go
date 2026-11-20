package main

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/ui"
)

// The backdrop is the field the user approved running (backdrop-spread.html,
// pixel/backdrop-field.js): long bands that flow slowly from an ember toward
// the far corner, dithered by Bayer 8x8 on a lattice of dots. Every pane draws
// the same field over its own area, from its top to its middle; a chat that
// has nothing in it yet draws it over the whole pane. It is inked in the
// theme's primary, the user's ask: a field in every theme's near-black grays
// gave no theme a face of its own.

const (
	// backdropSeed is the one the user's reference print shows. They wrote
	// 11916 but the print reads 11918; confirm with them.
	backdropSeed = 11918
	// seedSpread is how far apart two seeds put their hashes (seed * 131).
	seedSpread = 131
	// emberX and emberY are where the ember glows, as shares of the area: the
	// bottom-right corner, chosen in the centre-mix mockup (option B), so the
	// bands cross the screen along its edges and leave its middle calm.
	emberX, emberY = 0.92, 0.94
	// backdropCoarse is the field's own step, in cells: it is smooth, so it
	// is worked out every few cells and the cells between are interpolated.
	backdropCoarse = 4
	// backdropBand is the share of the pane, from its top, a pane's field
	// reaches.
	backdropBand = 0.5
)

// The start screen's field is calm in the middle, where its text is:
// centreFloor of it shows around the start screen's centre, rising to all of
// it from centreIn to centreOut of the half-width and half-height away
// (centre-mix mockup).
const (
	centreFloor = 0.2
	centreIn    = 0.25
	centreOut   = 0.9
)

// backdropOpacity is how much of the band shows behind a session's or the
// editor's text: the user's pick, 0.3, over the full field.
const backdropOpacity = 0.3

// backdropHot is how far the field's brightest dots go from the theme's
// primary toward white, so the ember keeps the theme's hue.
const backdropHot = 0.4

// backdropAwake is how long a session's backdrop keeps moving after the agent's
// turn is done, or the user last typed: then it holds still until the next
// (the user's call).
const backdropAwake = 10 * time.Second

// backdropStir is when a session's backdrop stops moving, chat or terminal
// alike: backdropAwake after the last frame its agent worked, its user typed,
// or its chat's start screen was up, which moves for as long as it shows.
type backdropStir struct {
	until time.Time
	typed int
}

// keep moves until on by what the session is doing at now: working, the
// length typed, and whether its chat's start screen is up.
func (s *backdropStir) keep(now time.Time, working bool, typed int, starting bool) {
	if working || typed != s.typed || starting {
		s.until = now.Add(backdropAwake)
	}
	s.typed = typed
}

// backdropStep is how long a frame of the field holds: the bands move a
// fraction of a cell in that time, so a faster step shows nothing more.
const backdropStep = 66 * time.Millisecond

// backdropCell is the lattice's step; a dot is backdropDot, a dp short of
// it, so the lattice shows as dots, not as a fill.
const (
	backdropCell unit.Dp = 3
	backdropDot  unit.Dp = 2
)

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

// noise2 is value noise on the integer lattice, smoothstepped between.
func noise2(x, y float64, s int) float64 {
	fx0, fy0 := math.Floor(x), math.Floor(y)
	ix, iy := int(fx0), int(fy0)
	fx, fy := x-fx0, y-fy0
	sx, sy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a, b := hash2(ix, iy, s), hash2(ix+1, iy, s)
	c, d := hash2(ix, iy+1, s), hash2(ix+1, iy+1, s)
	return a + (b-a)*sx + (c-a)*sy + (a-b-c+d)*sx*sy
}

// fbm sums oct octaves of noise2, each twice the frequency at half the weight.
func fbm(x, y float64, oct, s int) float64 {
	v, amp, f, sum := 0.0, 0.5, 1.0, 0.0
	for i := range oct {
		v += noise2(x*f, y*f, s+i*17) * amp
		sum += amp
		amp *= 0.5
		f *= 2
	}
	return v / sum
}

// backdropFade is how much of a pane's field shows at y, the share of the
// way down: gone by the band's end.
func backdropFade(y float64) float64 {
	if y >= backdropBand {
		return 0
	}
	f := 1 - y/backdropBand
	return f * f
}

// backdropField is the field over cols x rows cells: its flow, from the
// ember to the corner across from it, and the coarse values of one frame.
type backdropField struct {
	cols, rows   int
	ox, oy       float64
	fx, fy, diag float64
	coarseW      int
	coarse       []float32
}

func newBackdropField(cols, rows int) *backdropField {
	f := &backdropField{cols: cols, rows: rows, diag: math.Hypot(float64(cols), float64(rows))}
	f.ox, f.oy = emberX*float64(cols), emberY*float64(rows)
	dx, dy := -1.0, -1.0
	if float64(cols)-f.ox > f.ox {
		dx = 1
	}
	if float64(rows)-f.oy > f.oy {
		dy = 1
	}
	f.fx, f.fy = dx*float64(cols)/f.diag, dy*float64(rows)/f.diag
	f.coarseW = ceilDiv(cols, backdropCoarse) + 2
	f.coarse = make([]float32, f.coarseW*(ceilDiv(rows, backdropCoarse)+2))
	return f
}

// step works the coarse values out at t, in ms, under the first rows rows.
func (f *backdropField) step(t float64, rows int) {
	s := backdropSeed * seedSpread
	for ay := range ceilDiv(rows, backdropCoarse) + 2 {
		for ax := range f.coarseW {
			x, y := float64(ax*backdropCoarse), float64(ay*backdropCoarse)
			u, w := x*f.fx+y*f.fy, -x*f.fy+y*f.fx
			warp := fbm(u/150, w/90+t*0.00002, 2, 70+s) * 3
			band := min(1, max(0, (fbm(w/34+warp, u/190-t*0.00009, 3, 40+s)-0.3)/0.4))
			glow := math.Exp(-math.Hypot(x-f.ox, y-f.oy) / (0.3 * f.diag))
			f.coarse[ay*f.coarseW+ax] = float32(0.03 + 0.26*band*band + glow*glow*(0.25+0.75*band))
		}
	}
}

// at is the field's value at cell x, y, between the coarse values around it.
func (f *backdropField) at(x, y int) float64 {
	x0, y0 := x/backdropCoarse, y/backdropCoarse
	tx := float64(x)/backdropCoarse - float64(x0)
	ty := float64(y)/backdropCoarse - float64(y0)
	a, w, c := y0*f.coarseW+x0, f.coarseW, f.coarse
	top := float64(c[a])*(1-tx) + float64(c[a+1])*tx
	bottom := float64(c[a+w])*(1-tx) + float64(c[a+w+1])*tx
	return top*(1-ty) + bottom*ty
}

// backdropInks is a lit dot's color by its value in 64ths: the theme's
// primary up to half, so each theme's field wears the color of its own
// buttons, toward hot past it, at an opacity that grows with the
// value, composited onto the ground in sRGB.
type backdropInks [65]color.NRGBA

func newBackdropInks(primary, hot, ground color.NRGBA) (l backdropInks) {
	ch := func(m, f, g uint8, v float64) uint8 {
		c := float64(m)
		if v > 0.5 {
			c += (float64(f) - c) * (2*v - 1)
		}
		return uint8(math.Round(float64(g) + (c-float64(g))*(0.22+0.78*v)))
	}
	for k := range l {
		v := float64(k) / 64
		l[k] = color.NRGBA{R: ch(primary.R, hot.R, ground.R, v), G: ch(primary.G, hot.G, ground.G, v), B: ch(primary.B, hot.B, ground.B, v), A: 255}
	}
	return l
}

// paint draws rows rows of the field into img, a pixel a cell, lit cells in
// their ink and the rest transparent. cover is 1 for a pane's band, faded
// from its top to its middle, and 0 for the whole pane, calm around centre.
func (f *backdropField) paint(img *image.RGBA, rows int, cover float64, centre image.Point, quiet image.Rectangle, inks *backdropInks) {
	for y := range rows {
		keep := 1 - cover*(1-backdropFade((float64(y)+0.5)/float64(f.rows)))
		row := img.Pix[y*img.Stride:]
		for x := range f.cols {
			v := f.at(x, y) * keep * (1 - (1-cover)*(1-f.calm(x, y, centre)*hush(x, y, quiet)))
			p := row[4*x : 4*x+4 : 4*x+4]
			if v > bayer8[y&7][x&7] {
				c := inks[min(64, int(math.Round(v*64)))]
				p[0], p[1], p[2], p[3] = c.R, c.G, c.B, 255
			} else {
				p[0], p[1], p[2], p[3] = 0, 0, 0, 0
			}
		}
	}
}

// calm is how much of the field shows at cell x, y on the start screen,
// around centre.
func (f *backdropField) calm(x, y int, centre image.Point) float64 {
	d := math.Hypot(float64(x-centre.X)/(float64(f.cols)/2), float64(y-centre.Y)/(float64(f.rows)/2))
	k := min(1, max(0, (d-centreIn)/(centreOut-centreIn)))
	return centreFloor + (1-centreFloor)*k*k*(3-2*k)
}

// quietEdge is how many cells the field takes to come back around the start
// screen's quiet box.
const quietEdge = 6

// hush is how much of the field shows at cell x, y around the quiet box:
// none inside it, all of it quietEdge cells out.
func hush(x, y int, quiet image.Rectangle) float64 {
	if quiet.Empty() {
		return 1
	}
	dx := max(quiet.Min.X-x, 0, x-quiet.Max.X)
	dy := max(quiet.Min.Y-y, 0, y-quiet.Max.Y)
	k := min(1, math.Hypot(float64(dx), float64(dy))/quietEdge)
	return k * k * (3 - 2*k)
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

// backdropEntry is one pane's backdrop between frames.
type backdropEntry struct {
	size, cells image.Point
	cell, dot   int
	ground      color.NRGBA
	field       *backdropField
	img         *image.RGBA
	op          paint.ImageOp
	inks        backdropInks
	marks       markInks
	primary     color.NRGBA
	// tick, cover and mark are what op was painted for.
	tick  int64
	cover float32
	mark  backdropMark
	// gaps draws the ground between the dots, recorded once per size.
	gaps    op.Ops
	gapCall op.CallOp
	seenAt  time.Time
	clock   backdropClock
}

// backdropClock is how long a backdrop has moved for: it runs while the
// backdrop moves and holds while it does not, so a field that starts again
// goes on from where it stopped.
type backdropClock struct {
	moved time.Duration
	last  time.Time
}

// backdropCatchUp is the most one frame moves the clock: a window that drew
// nothing for a while (hidden, asleep) goes on from where it was.
const backdropCatchUp = ui.StillFrame

// advance runs the clock to now when the backdrop moves, and holds it when
// it does not.
func (c *backdropClock) advance(now time.Time, moving bool) {
	if moving && !c.last.IsZero() {
		c.moved += min(now.Sub(c.last), backdropCatchUp)
	}
	c.last = time.Time{}
	if moving {
		c.last = now
	}
}

// backdrops holds each pane's backdrop, and each session's stir.
type backdrops struct {
	byID  map[string]*backdropEntry
	stirs map[string]*backdropStir
	// painted counts the frames of the field painted, for the tests.
	painted int
}

// backdropKeep is how long a pane out of sight keeps its backdrop: going back
// to a card, or out of Settings, finds it ready, and a closed pane's goes.
const backdropKeep = time.Minute

// sweep forgets the backdrops no frame drew in the backdropKeep before now,
// and the stirs of sessions that drew none either.
func (b *backdrops) sweep(now time.Time) {
	for id, e := range b.byID {
		if now.Sub(e.seenAt) > backdropKeep {
			delete(b.byID, id)
		}
	}
	for id, s := range b.stirs {
		if _, drawn := b.byID[id]; !drawn && now.After(s.until.Add(backdropKeep)) {
			delete(b.stirs, id)
		}
	}
}

// stirOf is session id's stir.
func (b *backdrops) stirOf(id string) *backdropStir {
	if b.stirs == nil {
		b.stirs = map[string]*backdropStir{}
	}
	s, ok := b.stirs[id]
	if !ok {
		s = &backdropStir{}
		b.stirs[id] = s
	}
	return s
}

// backdrop paints the backdrop of the session or island id, size big on
// ground, at gtx's origin, its content top down from there: the field over
// the whole pane while its chat has nothing in it, with the start screen's
// mark in its dots, and its band otherwise. It is the one place a pane's
// backdrop is chosen.
func (u *window) backdrop(gtx C, id string, size image.Point, ground color.NRGBA, top int) {
	cover := u.backdropCover(gtx, id)
	// the band behind a session's text is faint; a chat's start screen, its
	// mark drawn in the field's dots, keeps the field whole
	defer paint.PushOpacity(gtx.Ops, 1-(1-backdropOpacity)*cover).Pop()
	u.backdrops.layout(gtx, u.theme, id, size, ground, cover, u.backdropMark(gtx, id, top), u.backdropMoves(gtx, id))
}

// backdropMoves is whether id's backdrop moves at gtx.Now: a session's while
// it is stirred (backdropStir), the stage's own always.
func (u *window) backdropMoves(gtx C, id string) bool {
	if _, ok := u.model.sessionViewOf(id); !ok {
		return true
	}
	return gtx.Now.Before(u.backdrops.stirOf(id).until)
}

// backdropMark is where session id's start screen put its mark, in cells of
// a pane whose content starts top down, or a mark faded out.
func (u *window) backdropMark(gtx C, id string, top int) backdropMark {
	sv, ok := u.model.sessionViewOf(id)
	if !ok || sv.View != api.ViewChat {
		return backdropMark{}
	}
	s := u.chats.pane(id).start
	cell := max(1, gtx.Dp(backdropCell))
	at, centre, quiet := s.mark.Add(image.Pt(0, top)), s.centre.Add(image.Pt(0, top)), s.quiet.Add(image.Pt(0, top))
	return backdropMark{
		at: image.Pt((at.X+cell/2)/cell, (at.Y+cell/2)/cell), centre: centre.Div(cell),
		quiet: image.Rectangle{Min: quiet.Min.Div(cell), Max: quiet.Max.Div(cell)}, fade: s.markFade,
	}
}

// backdropCover is 0 while session id is a chat with nothing in it, 1 once
// its start screen is gone, and between while it fades.
func (u *window) backdropCover(gtx C, id string) float32 {
	sv, ok := u.model.sessionViewOf(id)
	if !ok || sv.View != api.ViewChat {
		return 1
	}
	s := u.chats.pane(id).start
	if s.empty {
		return 0
	}
	if s.left.IsZero() {
		return 1
	}
	return chatPhase(gtx, s.left, 0, chatStartFade, paneEaseIn)
}

// backdropNext is the frame a backdrop that has moved for moved asks for at
// now: its next step while it moves, none while it holds still.
func backdropNext(now time.Time, moved time.Duration, moving, still bool) (time.Time, bool) {
	if !moving {
		return time.Time{}, false
	}
	// A window without focus keeps the frame it has and looks in less often.
	if still {
		return now.Add(ui.StillFrame), true
	}
	return now.Add(backdropStep - moved%backdropStep), true
}

// layout paints id's field, size big on ground, and while it moves asks for
// the frame that moves it on; a field that does not move holds its frame and
// asks for none.
func (b *backdrops) layout(gtx C, th *ui.Theme, id string, size image.Point, ground color.NRGBA, cover float32, mark backdropMark, moving bool) {
	e := b.entry(gtx, id, size, ground)
	e.clock.advance(gtx.Now, moving)
	tick := int64(e.clock.moved / backdropStep)
	if next, ok := backdropNext(gtx.Now, e.clock.moved, moving, th.Still); ok {
		gtx.Execute(op.InvalidateCmd{At: next})
	}
	if e.cells.X == 0 || e.cells.Y == 0 {
		return
	}
	if primary := ui.Over(ground, th.Primary); primary != e.primary || e.tick < 0 {
		hot := mixSRGB(primary, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, backdropHot)
		e.primary, e.inks, e.marks = primary, newBackdropInks(primary, hot, ground), newMarkInks(primary, hot, ground)
		e.tick = -1
	}
	rows := e.cells.Y
	if cover == 1 {
		rows = min(rows, int(math.Ceil(float64(rows)*backdropBand)))
	}
	if tick != e.tick || cover != e.cover || mark != e.mark {
		e.field.step(float64(time.Duration(tick)*backdropStep)/float64(time.Millisecond), rows)
		e.field.paint(e.img, rows, float64(cover), mark.centre, mark.quiet, &e.inks)
		if mark.fade > 0 {
			e.field.paintMark(e.img, rows, mark, &e.marks)
		}
		e.op = paint.NewImageOp(e.img)
		e.op.Filter = paint.FilterNearest
		e.tick, e.cover, e.mark = tick, cover, mark
		b.painted++
	}
	defer clip.Rect{Max: image.Pt(min(size.X, e.cells.X*e.cell), min(size.Y, rows*e.cell))}.Push(gtx.Ops).Pop()
	scale := op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(float32(e.cell), float32(e.cell)))).Push(gtx.Ops)
	cells := clip.Rect{Max: image.Pt(e.cells.X, rows)}.Push(gtx.Ops)
	e.op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	cells.Pop()
	scale.Pop()
	e.gapCall.Add(gtx.Ops)
}

// entry is id's backdrop for size on ground at gtx's scale, built again when
// any of them changed.
func (b *backdrops) entry(gtx C, id string, size image.Point, ground color.NRGBA) *backdropEntry {
	if b.byID == nil {
		b.byID = map[string]*backdropEntry{}
	}
	cell, dot := max(1, gtx.Dp(backdropCell)), max(1, gtx.Dp(backdropDot))
	e, ok := b.byID[id]
	if !ok || e.size != size || e.cell != cell || e.dot != dot || e.ground != ground {
		clock := backdropClock{}
		if ok {
			clock = e.clock
		}
		e = &backdropEntry{size: size, cell: cell, dot: dot, ground: ground, tick: -1, clock: clock}
		e.cells = image.Pt(ceilDiv(size.X, cell), ceilDiv(size.Y, cell))
		e.field = newBackdropField(e.cells.X, e.cells.Y)
		e.img = image.NewRGBA(image.Rectangle{Max: e.cells})
		e.recordGaps()
		b.byID[id] = e
	}
	e.seenAt = gtx.Now
	return e
}

// recordGaps records the ground's lines between the dots: the last cell-dot
// pixels of every column and row of cells.
func (e *backdropEntry) recordGaps() {
	e.gaps.Reset()
	m := op.Record(&e.gaps)
	full := e.cells.Mul(e.cell)
	if e.dot < e.cell {
		for i := range e.cells.X {
			paint.FillShape(&e.gaps, e.ground, clip.Rect{Min: image.Pt(i*e.cell+e.dot, 0), Max: image.Pt((i+1)*e.cell, full.Y)}.Op())
		}
		for j := range e.cells.Y {
			paint.FillShape(&e.gaps, e.ground, clip.Rect{Min: image.Pt(0, j*e.cell+e.dot), Max: image.Pt(full.X, (j+1)*e.cell)}.Op())
		}
	}
	e.gapCall = m.Stop()
}
