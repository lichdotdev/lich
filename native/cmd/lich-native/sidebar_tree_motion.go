package main

import (
	"image"
	"slices"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/lichdotdev/lich/native/ui"
)

// The tree's motion when a mother opens, closes or gains a child: the line
// runs down ahead of her children while their rows open behind it, turning
// into each ring and drawing it, and each row's glyph and name come in once
// its ring is closed. Closing plays the same motion backwards.
const (
	treeRingDraw = 130 * time.Millisecond
	treeBodyFade = 120 * time.Millisecond
	// treeLineShare is the share of the rows' opening the line takes to run
	// down them, so it stays ahead.
	treeLineShare = 0.7
	// treeBodyAfter is how far into its ring's draw a row's body starts in.
	treeBodyAfter = 0.75
	// treeCloseStretch slows the close as Presence's exit is to its enter.
	treeCloseStretch = 180.0 / 160
)

var treeCurve = ui.CubicBezier(0.4, 0, 0.2, 1)

// treeEase is treeCurve landing on its ends exactly, which its bisection
// only nears: a row not begun draws nothing.
func treeEase(x float32) float32 {
	if x <= 0 || x >= 1 {
		return clamp01(x)
	}
	return treeCurve(x)
}

// treeOpen is how long a run of n rows takes to open: 30ms more a row, up to
// 300ms.
func treeOpen(n int) time.Duration {
	return min(160*time.Millisecond+time.Duration(n)*30*time.Millisecond, 300*time.Millisecond)
}

// treeSweep is how far a run's line has come down a row, in rows of its
// height, along the column it runs in.
type treeSweep struct {
	col  int
	rows float32
}

// treeEntry is how much of a row's ring and body is still to come in, 0 at
// rest.
type treeEntry struct{ ring, body float32 }

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

// span is the opening's whole length: the last ring's line, its draw and its
// body.
func (r treeRun) span() time.Duration {
	n := float64(len(r.rows))
	line := float64(treeOpen(len(r.rows))) * treeLineShare * (n - 0.5) / n
	return time.Duration(line) + time.Duration(float64(treeRingDraw)*treeBodyAfter) + treeBodyFade
}

func (r treeRun) length() time.Duration {
	if r.closing {
		return time.Duration(float64(r.span()) * treeCloseStretch)
	}
	return r.span()
}

// at is where on the opening the run is at now: a closing run plays it
// backwards.
func (r treeRun) at(now time.Time) time.Duration {
	t := now.Sub(r.start)
	if r.closing {
		return time.Duration(float64(r.span()) * float64(1-clamp01(float32(t)/float32(r.length()))))
	}
	return t
}

// frame is the i-th row's share of the run at now.
func (r treeRun) frame(now time.Time, i int) rowMotion {
	open, n, t := treeOpen(len(r.rows)), float32(len(r.rows)), r.at(now)
	share := func(from, length time.Duration) float32 { return clamp01(float32(t-from) / float32(length)) }
	line := time.Duration(float32(open) * treeLineShare)
	reach := time.Duration(float32(line) * (float32(i) + 0.5) / n)
	ring := treeEase(share(reach, treeRingDraw))
	body := share(reach+time.Duration(float32(treeRingDraw)*treeBodyAfter), treeBodyFade)
	return rowMotion{
		height:  clamp01(treeEase(share(0, open))*n - float32(i)),
		sweep:   treeSweep{col: r.rows[0].depth - 1, rows: share(0, line)*n - float32(i)},
		entry:   treeEntry{ring: 1 - ring, body: 1 - body},
		closing: r.closing,
	}
}

func clamp01(v float32) float32 { return max(0, min(v, 1)) }

// treeMotion is a block's runs, read off its rows frame to frame.
type treeMotion struct {
	prev []sessionRow
	seen bool
	runs []treeRun
}

// update reads the rows live now against the last frame's: a stretch that
// came in under a row is a run opening, and one that left from under a row
// still drawn, its sessions still open, is a run closing. open holds the
// block's open sessions. It reports whether a run still moves.
func (m *treeMotion) update(now time.Time, live []sessionRow, open map[string]bool) bool {
	m.runs = slices.DeleteFunc(m.runs, func(r treeRun) bool { return now.Sub(r.start) >= r.length() })
	if m.seen {
		for _, s := range stretches(live, m.prev) {
			m.start(treeRun{rows: live[s.X:s.Y], after: live[s.X-1].key, start: now})
		}
		still := keySet(live)
		for _, s := range stretches(m.prev, live) {
			run := treeRun{rows: m.prev[s.X:s.Y], after: m.prev[s.X-1].key, start: now, closing: true}
			if still[run.after] && allOpen(run.rows, open) {
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

// stretches are the runs of rows not in other that hang from a line, as
// [X, Y) index ranges: each starts below a row, at a depth past 0, and ends
// where a row rises above it.
func stretches(rows, other []sessionRow) []image.Point {
	in := keySet(other)
	var out []image.Point
	for i := 1; i < len(rows); i++ {
		if in[rows[i].key] || rows[i].depth == 0 {
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

func keySet(rows []sessionRow) map[string]bool {
	keys := make(map[string]bool, len(rows))
	for _, r := range rows {
		keys[r.key] = true
	}
	return keys
}

// allOpen reports whether every session among rows is still open: a session
// closed leaves by its own motion (closingCards), not its group's.
func allOpen(rows []sessionRow, open map[string]bool) bool {
	return !slices.ContainsFunc(rows, func(r sessionRow) bool { return r.kind == rowCard && !open[r.sv.ID] })
}

// place gives rows their motion, with each closing run's rows put back under
// the row they hang from so they draw while they leave.
func (m *treeMotion) place(now time.Time, rows []sessionRow) []sessionRow {
	for _, run := range m.runs {
		if !run.closing {
			continue
		}
		at := slices.IndexFunc(rows, func(r sessionRow) bool { return r.key == run.after })
		if at >= 0 && !keySet(rows)[run.rows[0].key] {
			rows = slices.Insert(slices.Clone(rows), at+1, run.rows...)
		}
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
		split := int(treeColumn(gtx, r.depth) + float32(gtx.Dp(sessionTreeRingR)))
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
