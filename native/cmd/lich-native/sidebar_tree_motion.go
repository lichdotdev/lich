package main

import (
	"image"
	"image/color"
	"slices"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/lichdotdev/lich/native/ui"
)

// The tree's motion when rows come into a block or leave it (a mother opens
// or closes, a session opens or closes, a checkout gets its first session).
// Leaving: each row's glyph and name go, its ring undraws, and the line runs
// back up past it while the rows close. Coming in: a row opens with its name
// already on it, so nothing waits on the motion, and the line then runs down
// slowly to its ring, draws it, and the glyph comes in last. A root row hangs
// from no line: its ring draws alone.
const (
	treeRingDraw = 220 * time.Millisecond
	treeBodyFade = 200 * time.Millisecond
	// treeLineShare is the share of the rows' opening the line takes to run
	// down them, so it stays ahead.
	treeLineShare = 0.7
	// treeBodyAfter is how far into its ring's draw a row's body starts in.
	treeBodyAfter = 0.75
	// treeCloseStretch slows the close as Presence's exit is to its enter.
	treeCloseStretch = 180.0 / 160
)

// The coming in's pace (treeOpenLine aside).
const (
	treeEnterRing  = 300 * time.Millisecond
	treeEnterName  = 160 * time.Millisecond
	treeEnterDelay = 80 * time.Millisecond // a row's name after its row starts opening
)

// treeOpenLine is how long the line takes to run down n rows coming in:
// 100ms more a row, up to 800ms.
func treeOpenLine(n int) time.Duration {
	return min(450*time.Millisecond+time.Duration(n-1)*100*time.Millisecond, 800*time.Millisecond)
}

var treeCurve = ui.CubicBezier(0.4, 0, 0.2, 1)

// treeEase is treeCurve landing on its ends exactly, which its bisection
// only nears: a row not begun draws nothing.
func treeEase(x float32) float32 {
	if x <= 0 || x >= 1 {
		return clamp01(x)
	}
	return treeCurve(x)
}

// treeOpen is how long a run of n rows takes to open: 50ms more a row, up to
// 480ms.
func treeOpen(n int) time.Duration {
	return min(250*time.Millisecond+time.Duration(n)*50*time.Millisecond, 480*time.Millisecond)
}

// treeSweep is how far a run's line has come down a row, in rows of its
// height, along the column it runs in.
type treeSweep struct {
	col  int
	rows float32
}

// treeEntry is how much of a row's ring, its glyph and its body (the line
// past the ring) is still to come in, 0 at rest.
type treeEntry struct{ ring, glyph, body float32 }

// rowMotion is a row's share of its run at one frame.
type rowMotion struct {
	height  float32 // the share of its height drawn
	sweep   treeSweep
	entry   treeEntry
	closing bool
}

// treeRun is a stretch of rows coming in under a row already drawn, or
// leaving from under one that stays.
type treeRun struct {
	rows    []sessionRow
	after   string // the row the run hangs under
	start   time.Time
	closing bool
}

// span is the leaving's whole length played forwards: the last ring's line,
// its draw and its body.
func (r treeRun) span() time.Duration {
	n := float64(len(r.rows))
	line := float64(treeOpen(len(r.rows))) * treeLineShare * (n - 0.5) / n
	return time.Duration(line) + time.Duration(float64(treeRingDraw)*treeBodyAfter) + treeBodyFade
}

// length is how long the run moves: coming in, until the last row's glyph is
// in; leaving, its span slowed.
func (r treeRun) length() time.Duration {
	if r.closing {
		return time.Duration(float64(r.span()) * treeCloseStretch)
	}
	n := float64(len(r.rows))
	reach := time.Duration(float64(treeOpenLine(len(r.rows))) * (n - 0.5) / n)
	return max(treeOpen(len(r.rows)), reach+time.Duration(float64(treeEnterRing)*treeBodyAfter)+treeEnterName)
}

// frame is the i-th row's share of the run at now.
func (r treeRun) frame(now time.Time, i int) rowMotion {
	if r.closing {
		return r.leaving(now, i)
	}
	open, line, n := treeOpen(len(r.rows)), treeOpenLine(len(r.rows)), float32(len(r.rows))
	t := now.Sub(r.start)
	share := func(from, length time.Duration) float32 { return clamp01(float32(t-from) / float32(length)) }
	reach := time.Duration(float32(line) * (float32(i) + 0.5) / n)
	start := time.Duration(float32(open) * float32(i) / n)
	if r.rows[0].depth == 0 {
		// A root hangs from no line: its ring draws as its row opens.
		reach = start
	}
	return rowMotion{
		height: clamp01(treeEase(share(0, open))*n - float32(i)),
		sweep:  treeSweep{col: r.rows[0].depth - 1, rows: treeEase(share(0, line))*n - float32(i)},
		entry: treeEntry{
			ring:  1 - treeEase(share(reach, treeEnterRing)),
			glyph: 1 - share(reach+time.Duration(float32(treeEnterRing)*treeBodyAfter), treeEnterName),
			body:  1 - share(start+treeEnterDelay, treeEnterName),
		},
	}
}

// leaving is the i-th row's share of a closing run at now: its own
// timeline played backwards, glyph and body going together.
func (r treeRun) leaving(now time.Time, i int) rowMotion {
	open, n := treeOpen(len(r.rows)), float32(len(r.rows))
	t := time.Duration(float64(r.span()) * float64(1-clamp01(float32(now.Sub(r.start))/float32(r.length()))))
	share := func(from, length time.Duration) float32 { return clamp01(float32(t-from) / float32(length)) }
	line := time.Duration(float32(open) * treeLineShare)
	reach := time.Duration(float32(line) * (float32(i) + 0.5) / n)
	ring := treeEase(share(reach, treeRingDraw))
	body := share(reach+time.Duration(float32(treeRingDraw)*treeBodyAfter), treeBodyFade)
	return rowMotion{
		height:  clamp01(treeEase(share(0, open))*n - float32(i)),
		sweep:   treeSweep{col: r.rows[0].depth - 1, rows: share(0, line)*n - float32(i)},
		entry:   treeEntry{ring: 1 - ring, glyph: 1 - body, body: 1 - body},
		closing: true,
	}
}

func clamp01(v float32) float32 { return max(0, min(v, 1)) }

// treeMotion is a block's runs, read off its rows frame to frame.
type treeMotion struct {
	prev []sessionRow
	seen bool
	// from is when the runs a block arriving starts with begin, zero for now.
	from time.Time
	runs []treeRun
}

// update reads the rows live now against the last frame's: a stretch that
// came in is a run opening, and one that left from under a row still drawn,
// or from the block's top, is a run closing. It reports whether a run still
// moves.
func (m *treeMotion) update(now time.Time, live []sessionRow) bool {
	m.runs = slices.DeleteFunc(m.runs, func(r treeRun) bool { return now.Sub(r.start) >= r.length() })
	if m.seen {
		start := now
		if !m.from.IsZero() {
			start, m.from = m.from, time.Time{}
		}
		for _, s := range stretches(live, m.prev) {
			m.start(treeRun{rows: live[s.X:s.Y], after: keyBefore(live, s.X), start: start})
		}
		still := keySet(live)
		for _, s := range stretches(m.prev, live) {
			run := treeRun{rows: m.prev[s.X:s.Y], after: keyBefore(m.prev, s.X), start: now, closing: true}
			if run.after == "" || still[run.after] {
				m.start(run)
			}
		}
	}
	m.prev, m.seen = live, true
	return len(m.runs) > 0
}

// start drops the runs that move any of run's rows, then adds run.
func (m *treeMotion) start(run treeRun) {
	keys := keySet(run.rows)
	m.runs = slices.DeleteFunc(m.runs, func(r treeRun) bool {
		return slices.ContainsFunc(r.rows, func(row sessionRow) bool { return keys[row.key] })
	})
	m.runs = append(m.runs, run)
}

// arrive marks the block as one that came into a list already drawn: its
// first rows are news, and come in as a run from at (zero for now).
func (m *treeMotion) arrive(at time.Time) {
	if !m.seen {
		m.seen, m.from = true, at
	}
}

// keyBefore is the key of the row before rows[i], "" at the block's top.
func keyBefore(rows []sessionRow, i int) string {
	if i == 0 {
		return ""
	}
	return rows[i-1].key
}

// stretches are the runs of rows not in other, as [X, Y) index ranges: each
// ends where a row rises above its first.
func stretches(rows, other []sessionRow) []image.Point {
	in := keySet(other)
	var out []image.Point
	for i := 0; i < len(rows); i++ {
		if in[rows[i].key] {
			continue
		}
		j := i + 1
		for j < len(rows) && !in[rows[j].key] && rows[j].depth >= rows[i].depth {
			j++
		}
		out = append(out, image.Pt(i, j))
		i = j - 1
	}
	return out
}

// hasRows is whether the block drew rows last frame or still draws ones
// leaving.
func (m *treeMotion) hasRows() bool { return len(m.prev) > 0 || len(m.runs) > 0 }

func keySet(rows []sessionRow) map[string]bool {
	keys := make(map[string]bool, len(rows))
	for _, r := range rows {
		keys[r.key] = true
	}
	return keys
}

// place gives rows their motion, with each closing run's rows put back under
// the row they hang from, or at the top, so they draw while they leave.
func (m *treeMotion) place(now time.Time, rows []sessionRow) []sessionRow {
	spliced := false
	for _, run := range m.runs {
		if !run.closing {
			continue
		}
		at := slices.IndexFunc(rows, func(r sessionRow) bool { return r.key == run.after })
		if (at >= 0 || run.after == "") && !keySet(rows)[run.rows[0].key] {
			rows = slices.Insert(slices.Clone(rows), at+1, run.rows...)
			spliced = true
		}
	}
	if spliced {
		// The rows around a run put back were linked without it: a sibling
		// above it ended its line there, and the run's own line would float.
		rows = relink(rows)
	}
	for _, run := range m.runs {
		for i, r := range run.rows {
			if at := slices.IndexFunc(rows, func(row sessionRow) bool { return row.key == r.key }); at >= 0 {
				motion := run.frame(now, i)
				rows[at].motion = &motion
			}
		}
	}
	return rows
}

// relink sets every row's tail and lanes from the rows' depths, the tree as
// sessionRows links it: a row ends its parent's line when no sibling follows
// before the tree rises above it, and a level's line runs past a row while a
// row at that level follows before the tree rises above it.
func relink(rows []sessionRow) []sessionRow {
	// next[d] is whether a row at depth d follows before one shallower,
	// scanned from the bottom up.
	var next []bool
	for i := len(rows) - 1; i >= 0; i-- {
		d := rows[i].depth
		for len(next) <= d {
			next = append(next, false)
		}
		if d > 0 {
			rows[i].tail = !next[d]
			lanes := make([]bool, d-1)
			copy(lanes, next[1:d])
			rows[i].lanes = lanes
		}
		next[d] = true
		for k := d + 1; k < len(next); k++ {
			next[k] = false
		}
	}
	return rows
}

// moveRow draws a row in its run: opened to its share of height, and the
// body past its ring at the opacity it has come in to. A moving row takes no
// input.
func moveRow(gtx C, r sessionRow, w layout.Widget) D {
	m := r.motion
	gtx = gtx.Disabled()
	return ui.Reveal(gtx, m.height, func(gtx C) D {
		rec := op.Record(gtx.Ops)
		d := w(gtx)
		call := rec.Stop()
		split := bodySplit(gtx, r.depth)
		left := clip.Rect{Max: image.Pt(split, d.Size.Y)}.Push(gtx.Ops)
		call.Add(gtx.Ops)
		left.Pop()
		right := clip.Rect{Min: image.Pt(split, 0), Max: d.Size}.Push(gtx.Ops)
		fade := paint.PushOpacity(gtx.Ops, 1-m.entry.body)
		call.Add(gtx.Ops)
		fade.Pop()
		right.Pop()
		return d
	})
}

// bodySplit is the x where a row's body starts, past its ring.
func bodySplit(gtx C, depth int) int {
	return int(treeColumn(gtx, depth) + float32(gtx.Dp(sessionTreeRingR)))
}

// treeRowFill is a row's fill under its links. moveRow fades only what is past
// the ring, so the share of the fill before it fades here.
func treeRowFill(gtx C, fill color.NRGBA, p treePlace) D {
	if p.entry.body == 0 {
		return ui.Fill(gtx, fill, ui.RadiusMD)
	}
	size, split := gtx.Constraints.Min, bodySplit(gtx, p.depth)
	left := clip.Rect{Max: image.Pt(split, size.Y)}.Push(gtx.Ops)
	ui.Fill(gtx, ui.Alpha(fill, 1-p.entry.body), ui.RadiusMD)
	left.Pop()
	defer clip.Rect{Min: image.Pt(split, 0), Max: size}.Push(gtx.Ops).Pop()
	return ui.Fill(gtx, fill, ui.RadiusMD)
}
