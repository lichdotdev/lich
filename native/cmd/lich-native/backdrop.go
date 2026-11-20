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
// has nothing in it yet draws it over the whole pane.

const (
	// backdropSeed is the one the user's reference print shows. They wrote
	// 11916 but the print reads 11918; confirm with them.
	backdropSeed = 11918
	// seedSpread is how far apart two seeds put their hashes (seed * 131).
	seedSpread = 131
	// emberX and emberY are where the ember glows, as shares of the area:
	// read off the user's print, about 52% across and 64% down.
	emberX, emberY = 0.52, 0.64
	// backdropCoarse is the field's own step, in cells: it is smooth, so it
	// is worked out every few cells and the cells between are interpolated.
	backdropCoarse = 4
	// backdropBand is the share of the pane, from its top, a pane's field
	// reaches.
	backdropBand = 0.5
)

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

// backdropInks is a lit dot's color by its value in 64ths: the muted
// foreground up to half, toward the foreground past it, at an opacity that
// grows with the value, composited onto the ground in sRGB.
type backdropInks [65]color.NRGBA

func newBackdropInks(muted, fg, ground color.NRGBA) (l backdropInks) {
	ch := func(m, f, g uint8, v float64) uint8 {
		c := float64(m)
		if v > 0.5 {
			c += (float64(f) - c) * (2*v - 1)
		}
		return uint8(math.Round(float64(g) + (c-float64(g))*(0.22+0.78*v)))
	}
	for k := range l {
		v := float64(k) / 64
		l[k] = color.NRGBA{R: ch(muted.R, fg.R, ground.R, v), G: ch(muted.G, fg.G, ground.G, v), B: ch(muted.B, fg.B, ground.B, v), A: 255}
	}
	return l
}

// paint draws rows rows of the field into img, a pixel a cell, lit cells in
// their ink and the rest transparent. cover is 1 for a pane's band, faded
// from its top to its middle, and 0 for the whole pane.
func (f *backdropField) paint(img *image.RGBA, rows int, cover float64, inks *backdropInks) {
	for y := range rows {
		keep := 1 - cover*(1-backdropFade((float64(y)+0.5)/float64(f.rows)))
		row := img.Pix[y*img.Stride:]
		for x := range f.cols {
			v := f.at(x, y) * keep
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
	tints       [2]color.NRGBA
	// tick, cover and mark are what op was painted for.
	tick  int64
	cover float32
	mark  backdropMark
	// gaps draws the ground between the dots, recorded once per size.
	gaps    op.Ops
	gapCall op.CallOp
	seenAt  time.Time
}

// backdrops holds each pane's backdrop, and the clock they all move on.
type backdrops struct {
	byID  map[string]*backdropEntry
	epoch time.Time
	// painted counts the frames of the field painted, for the tests.
	painted int
}

// backdropKeep is how long a pane out of sight keeps its backdrop: going back
// to a card, or out of Settings, finds it ready, and a closed pane's goes.
const backdropKeep = time.Minute

// sweep forgets the backdrops no frame drew in the backdropKeep before now.
func (b *backdrops) sweep(now time.Time) {
	for id, e := range b.byID {
		if now.Sub(e.seenAt) > backdropKeep {
			delete(b.byID, id)
		}
	}
}

// backdrop paints the backdrop of the session or island id, size big on
// ground, at gtx's origin, its content top down from there: the field over
// the whole pane while its chat has nothing in it, with the start screen's
// mark in its dots, and its band otherwise. It is the one place a pane's
// backdrop is chosen.
func (u *window) backdrop(gtx C, id string, size image.Point, ground color.NRGBA, top int) {
	u.backdrops.layout(gtx, u.theme, id, size, ground, u.backdropCover(gtx, id), u.backdropMark(gtx, id, top))
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
	at := s.mark.Add(image.Pt(0, top))
	return backdropMark{at: image.Pt((at.X+cell/2)/cell, (at.Y+cell/2)/cell), fade: s.markFade}
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

// layout paints id's field at the frame of gtx.Now, size big on ground, and
// asks for the frame that moves it on.
func (b *backdrops) layout(gtx C, th *ui.Theme, id string, size image.Point, ground color.NRGBA, cover float32, mark backdropMark) {
	if b.epoch.IsZero() {
		b.epoch = gtx.Now
	}
	tick := int64(gtx.Now.Sub(b.epoch) / backdropStep)
	next := b.epoch.Add(time.Duration(tick+1) * backdropStep)
	// A window without focus keeps the frame it has and looks in less often.
	if th.Still {
		next = gtx.Now.Add(ui.StillFrame)
	}
	gtx.Execute(op.InvalidateCmd{At: next})
	e := b.entry(gtx, id, size, ground)
	if e.cells.X == 0 || e.cells.Y == 0 {
		return
	}
	if tints := [2]color.NRGBA{ui.Over(ground, th.MutedForeground), ui.Over(ground, th.Foreground)}; tints != e.tints {
		e.tints, e.inks, e.marks = tints, newBackdropInks(tints[0], tints[1], ground), newMarkInks(th, ground)
		e.tick = -1
	}
	rows := e.cells.Y
	if cover == 1 {
		rows = min(rows, int(math.Ceil(float64(rows)*backdropBand)))
	}
	if tick != e.tick || cover != e.cover || mark != e.mark {
		e.field.step(float64(time.Duration(tick)*backdropStep)/float64(time.Millisecond), rows)
		e.field.paint(e.img, rows, float64(cover), &e.inks)
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
		e = &backdropEntry{size: size, cell: cell, dot: dot, ground: ground, tick: -1}
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
