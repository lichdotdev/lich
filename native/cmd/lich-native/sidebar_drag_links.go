package main

import (
	"image"
	"maps"
	"math"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// The tree's line while a card is dragged in its block: the card lets go of
// its line as it lifts, a dashed slot shows where it will join while it is in
// the air, and once it lands the line draws back to its ring.
const (
	dragLetGo  = 200 * time.Millisecond
	dragSlotIn = 160 * time.Millisecond
	dragJoin   = 220 * time.Millisecond
	// dragSettle is ui's sortSettle, the card settling into its slot after
	// the drop: the line waits for it.
	dragSettle = 250 * time.Millisecond
)

// dragDash is the on and off length of the slot's dashes.
var dragDash = unit.Dp(3)

// dragRows is rows as a drag in their list shows them: active moved to the
// slot over takes it to, every row linked where it stands. An over that is
// no row of the list, a header the card is carried to, moves nothing.
func dragRows(rows []sessionRow, active, over string) []sessionRow {
	key := func(k string) func(sessionRow) bool { return func(r sessionRow) bool { return r.key == k } }
	from, to := slices.IndexFunc(rows, key(active)), slices.IndexFunc(rows, key(over))
	if from < 0 || to < 0 || from == to {
		return rows
	}
	moved := slices.Delete(slices.Clone(rows), from, from+1)
	return relink(slices.Insert(moved, to, rows[from]))
}

// dragLine is a block's card drag as its tree draws it: the card in the air,
// or the one that landed while its line draws back.
type dragLine struct {
	key            string
	lifted, landed time.Time
	glide          slotGlide
	// row, at and length are the slot the card would land in, as the last
	// frame in the air left it: where it settles once let go.
	row        sessionRow
	at, length int
}

// update reads the card the block's list holds in the air, "" for none.
func (d *dragLine) update(now time.Time, dragging string) {
	switch {
	case dragging != "" && dragging != d.key:
		*d = dragLine{key: dragging, lifted: now}
	case dragging == "" && d.key != "" && d.landed.IsZero():
		d.landed = now
	case d.key != "" && !d.landed.IsZero() && now.Sub(d.landed) >= dragSettle+dragJoin:
		*d = dragLine{}
	}
}

// hold keeps the slot the card in the air would land in.
func (d *dragLine) hold(r sessionRow, at, length int) { d.row, d.at, d.length = r, at, length }

// loose reports whether row key is the card in the air, or settling after
// it, which draws none of its lines: the slot draws them where it lands.
func (d *dragLine) loose(key string, now time.Time) bool {
	return key == d.key && key != "" && (d.landed.IsZero() || now.Sub(d.landed) < dragSettle)
}

// moving reports whether the line still changes.
func (d *dragLine) moving(now time.Time) bool {
	switch {
	case d.key == "":
		return false
	case d.landed.IsZero():
		return now.Sub(d.lifted) < dragLetGo+dragSlotIn || d.glide.moving(now)
	}
	return now.Sub(d.landed) < dragSettle+dragJoin
}

// cut is how much of row key's own elbow is not drawn: all of it while the
// card is in the air or settling, then less as the line draws back.
func (d *dragLine) cut(key string, now time.Time) float32 {
	if key != d.key || d.key == "" {
		return 0
	}
	if d.landed.IsZero() {
		return 1
	}
	return 1 - treeEase(clamp01(float32(now.Sub(d.landed)-dragSettle)/float32(dragJoin)))
}

// slot is the slot's own line let go of, from its ring back to its parent's
// line, and how far its dashes have come in: out again as the card settles.
func (d *dragLine) slot(now time.Time) (letGo, dashes float32) {
	t := now.Sub(d.lifted)
	letGo, dashes = treeEase(clamp01(float32(t)/float32(dragLetGo))), clamp01(float32(t-dragLetGo)/float32(dragSlotIn))
	if !d.landed.IsZero() {
		dashes *= 1 - clamp01(float32(now.Sub(d.landed))/float32(dragSlotIn))
	}
	return letGo, dashes
}

// showsSlot reports whether the slot draws: while the card is in the air or
// settling into it.
func (d *dragLine) showsSlot(now time.Time) bool {
	return d.key != "" && d.length > 0 && d.loose(d.key, now)
}

// slotGlide moves the slot to where the card would land as the list's rows
// step aside, in their time.
type slotGlide struct {
	from, to float32
	at       time.Time
	seen     bool
}

func (g *slotGlide) y(now time.Time, to float32) float32 {
	if !g.seen {
		*g = slotGlide{from: to, to: to, at: now, seen: true}
	}
	if to != g.to {
		g.from, g.to, g.at = g.y(now, g.to), to, now
	}
	share := treeEase(clamp01(float32(now.Sub(g.at)) / float32(dragSettle)))
	return g.from + (g.to-g.from)*share
}

func (g *slotGlide) moving(now time.Time) bool { return g.seen && now.Sub(g.at) < dragSettle }

// dragSlot draws where the card in the air would land, at its top in the list:
// the row's lines where it stands, its own elbow letting go as it lifts, then
// that elbow and the ring dashed.
func (s *sidebar) dragSlot(gtx C, th *ui.Theme, d *dragLine) {
	r, height := d.row, d.length
	y := d.glide.y(gtx.Now, float32(d.at))
	defer op.Offset(image.Pt(0, int(math.Round(float64(y))))).Push(gtx.Ops).Pop()
	size := image.Pt(gtx.Constraints.Max.X, height)
	letGo, dashes := d.slot(gtx.Now)
	l := r.place(true).sessionRowLinks
	l.cut = letGo
	treeLinks(gtx, th, l, size)
	if dashes == 0 {
		return
	}
	defer paint.PushOpacity(gtx.Ops, dashes).Pop()
	c := th.Over(ui.Alpha(th.MutedForeground, treeLinkAlpha))
	if r.depth > 0 {
		width := float32(gtx.Dp(sessionTreeStroke))
		// One stroke a dash, as slotRing's.
		for _, dash := range dashed(elbowPoints(gtx, r.depth, float32(height)), float32(gtx.Dp(dragDash))) {
			strokeLinks(gtx, []linkSeg{dash}, size, c, width)
		}
	}
	ui.DashedCircle(gtx, f32.Pt(treeColumn(gtx, r.depth), float32(height)/2), sessionTreeRingR, sessionTreeStroke, dragDash, c)
}

// elbowPoints is a row's elbow as a polyline: down its parent's line to the
// bend, round it, and across to its ring.
func elbowPoints(gtx C, depth int, h float32) []f32.Point {
	px, mid := treeColumn(gtx, depth-1), h/2
	elbow := float32(gtx.Dp(sessionTreeElbow))
	ring := treeColumn(gtx, depth) - float32(gtx.Dp(sessionTreeRingR))
	const steps = 6
	pts := make([]f32.Point, 0, steps+2)
	for i := range steps + 1 {
		t := float32(i) / steps
		// The quadratic from (px, mid-elbow) through (px, mid) to (px+elbow, mid).
		x := px + t*t*elbow
		y := mid - elbow + (2*t-t*t)*elbow
		pts = append(pts, f32.Pt(x, y))
	}
	return append(pts, f32.Pt(ring, mid))
}

// dashed is the polyline pts cut into dashes dash long, dash apart.
func dashed(pts []f32.Point, dash float32) []linkSeg {
	var out []linkSeg
	on, left := true, dash
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		length := float32(math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y)))
		for done := float32(0); done < length; {
			step := min(left, length-done)
			p, q := lerpPt(a, b, done/length), lerpPt(a, b, (done+step)/length)
			if on {
				out = append(out, func(path *clip.Path) { path.MoveTo(p); path.LineTo(q) })
			}
			done += step
			if left -= step; left <= 0 {
				on, left = !on, dash
			}
		}
	}
	return out
}

func lerpPt(a, b f32.Point, t float32) f32.Point {
	return f32.Pt(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
}

// dragLinks reads the block's card drag into its tree: while a card is in the
// air its rows are linked in the order the list shows them. It answers the
// card in the air, "" for none.
func (s *sidebar) dragLinks(gtx C, st *blockState, keys []string, byKey map[string]sessionRow) string {
	dragging := st.cards.Dragging()
	st.drag.update(gtx.Now, dragging)
	if dragging != "" {
		rows := make([]sessionRow, len(keys))
		for i, k := range keys {
			rows[i] = byKey[k]
		}
		for _, r := range dragRows(rows, dragging, st.cards.Over()) {
			byKey[r.key] = r
		}
	}
	if st.drag.moving(gtx.Now) {
		gtx.Execute(op.InvalidateCmd{})
	}
	return dragging
}

// carriedFold is b with the mother in the air folded over her children, so
// they go with her: the list moves her row alone, and they open under her
// where she lands.
func carriedFold(b blockView, dragging string) blockView {
	b.carried = dragging
	if dragging == "" || b.motherFolds[dragging] {
		return b
	}
	folds := maps.Clone(b.motherFolds)
	if folds == nil {
		folds = map[string]bool{}
	}
	folds[dragging] = true
	b.motherFolds = folds
	return b
}

// treeSnap keeps a drag in b's list of rows keys to the slots its tree keeps:
// one where the rows the drop leaves stand in the order the list showed
// while the card was in the air. A child stays under her mother and a
// session under its checkout, whatever the stored order says. Off such a
// slot, the drag takes the nearest one, towards its own on a tie.
func treeSnap(b blockView, keys []string) func(a, o int) int {
	legal := func(a, o int) bool {
		if a == o {
			return true
		}
		ids := arrayMove(blockIDs(b), keys[a], keys[o])
		byID := make(map[string]sessionView, len(b.sessions))
		for _, sv := range b.sessions {
			byID[sv.ID] = sv
		}
		ordered := make([]sessionView, 0, len(ids))
		for _, id := range ids {
			ordered = append(ordered, byID[id])
		}
		got := map[string]int{}
		for i, r := range blockRows(b, ordered) {
			got[r.key] = i
		}
		last := -1
		for _, k := range arrayMove(keys, keys[a], keys[o]) {
			at, ok := got[k]
			if !ok {
				continue
			}
			if at < last {
				return false
			}
			last = at
		}
		return true
	}
	return func(a, o int) int {
		toward := 1
		if a < o {
			toward = -1
		}
		for d := 0; d < len(keys); d++ {
			for _, c := range []int{o + toward*d, o - toward*d} {
				if c >= 0 && c < len(keys) && legal(a, c) {
					return c
				}
			}
		}
		return a
	}
}
