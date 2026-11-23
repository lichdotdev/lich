package markdown

// Where each rune of a cell is drawn, for selecting it. styledtext draws a
// cell span by span and tells its callback only a piece's size, not where
// the piece stands nor which runes it holds, so this file lays the spans out
// again the way styledtext.TextStyle.Layout does (gioui.org/x v0.10.3) and
// keeps every rune's box. TestTheGeometryLaysOutAsStyledtextDraws pins the
// two together.

import (
	"image"
	"slices"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

// runeBox is one rune's room on its line, in the cell's pixels: from x0 to
// x1, the line's glyphs from top down height.
type runeBox struct {
	x0, x1, top, height int
}

// cellGeometry is the box of every rune of a cell laid out at max and metric.
type cellGeometry struct {
	boxes  []runeBox
	max    image.Point
	metric unit.Metric
	shaper *text.Shaper
}

// shapedPiece is what styledtext learns of a span laid out on what is left
// of a line: the piece it draws there and how many runes that took.
type shapedPiece struct {
	width, height    int
	runes            int
	multiLine        bool
	endedWithNewline bool
	// boxes are the piece's runes, x from its left; a rune on its first
	// line has top 0 and takes the line's height once the line is done.
	boxes []runeBox
}

// queuedSpan is what is left of span i to lay out.
type queuedSpan struct {
	i       int
	content string
}

// geometry is the box of each of the cell's runes as gtx lays the cell out,
// kept until the constraints or the metric change.
func (c *markdownCell) geometry(gtx C, shaper *text.Shaper) *cellGeometry {
	if g := c.geo; g != nil && g.max == gtx.Constraints.Max && g.metric == gtx.Metric && g.shaper == shaper {
		return g
	}
	g := &cellGeometry{max: gtx.Constraints.Max, metric: gtx.Metric, shaper: shaper}
	spans := make([]queuedSpan, len(c.spans))
	for i, s := range c.spans {
		spans[i] = queuedSpan{i, s.Content}
	}
	var line lineRunes
	for k := 0; k < len(spans); k++ {
		q := spans[k]
		maxWidth := gtx.Constraints.Max.X - line.x
		res := c.shapeSpan(gtx, shaper, maxWidth, q, true)
		if res.multiLine && res.runes == 0 {
			// a word wider than the line: styledtext lays it out wrapped
			res = c.shapeSpan(gtx, shaper, maxWidth, q, false)
		}
		force := line.x > 0 && res.width > maxWidth
		if !force {
			line.add(res)
		}
		if res.multiLine || res.endedWithNewline || k == len(spans)-1 || force {
			g.boxes = line.end(g.boxes, c.lineHeightScale())
		}
		switch {
		case res.multiLine && !force:
			rest := queuedSpan{q.i, q.content[runeBytes(q.content, res.runes):]}
			spans = slices.Insert(spans, k+1, rest)
		case force:
			k--
		}
	}
	c.geo = g
	return g
}

// lineRunes is the line being laid out: its runes so far, how far they reach
// and the height of its tallest piece, and the top of the line.
type lineRunes struct {
	boxes   []runeBox
	x, h, y int
}

// add puts a piece at the end of the line.
func (l *lineRunes) add(res shapedPiece) {
	if n := len(res.boxes); n > 0 && res.boxes[n-1].top == 0 {
		// the next piece starts at this one's width, a rounded-up edge:
		// the last rune reaches it, or a gap shows between
		res.boxes[n-1].x1 = max(res.boxes[n-1].x1, res.width)
	}
	for _, b := range res.boxes {
		b.x0, b.x1 = b.x0+l.x, b.x1+l.x
		l.boxes = append(l.boxes, b)
	}
	l.x += res.width
	l.h = max(l.h, res.height)
}

// end appends the line's runes to boxes, each piece standing on the line's
// top as tall as its tallest, and starts the next line under it.
func (l *lineRunes) end(boxes []runeBox, scale float32) []runeBox {
	for _, b := range l.boxes {
		if b.top == 0 {
			b.height = l.h
		}
		b.top += l.y
		boxes = append(boxes, b)
	}
	l.boxes = l.boxes[:0]
	l.y += int(float32(l.h) * scale)
	l.x, l.h = 0, 0
	return boxes
}

// shapeSpan is styledtext's layoutSpan and iterateSpan for what is left of a
// span, on maxWidth of line.
func (c *markdownCell) shapeSpan(gtx C, shaper *text.Shaper, maxWidth int, q queuedSpan, truncate bool) shapedPiece {
	span := c.spans[q.i]
	maxLines := 0
	if truncate {
		maxLines = 1
	}
	shaper.LayoutString(text.Parameters{
		Font:            span.Font,
		PxPerEm:         fixed.I(gtx.Sp(span.Size)),
		MaxLines:        maxLines,
		MaxWidth:        maxWidth,
		Truncator:       "\u200b",
		Locale:          gtx.Locale,
		WrapPolicy:      text.WrapWords,
		LineHeightScale: c.lineHeightScale(),
	}, q.content)
	res := readPiece(shaper, image.Rectangle{Max: gtx.Constraints.Max})
	if res.runes < utf8.RuneCountInString(q.content) {
		res.multiLine = true
		if r, _ := utf8.DecodeRuneInString(q.content[runeBytes(q.content, res.runes):]); r == '\n' {
			res.endedWithNewline = true
			res.runes++
			end := 0
			if n := len(res.boxes); n > 0 {
				end = res.boxes[n-1].x1
			}
			res.boxes = append(res.boxes, runeBox{x0: end, x1: end})
		}
	}
	return res
}

// readPiece is styledtext's textIterator over the glyphs shaper holds: the
// first line's runes, up to its paragraph break, and their bounds in
// viewport.
func readPiece(shaper *text.Shaper, viewport image.Rectangle) shapedPiece {
	var (
		res             shapedPiece
		bounds          image.Rectangle
		seen, started   bool
		baseline        int32
		firstX, cluster fixed.Int26_6
		linesSeen       int
	)
	for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
		logical := image.Rectangle{
			Min: image.Pt(g.X.Floor(), int(g.Y)-g.Ascent.Ceil()),
			Max: image.Pt((g.X + g.Advance).Ceil(), int(g.Y)+g.Descent.Ceil()),
		}
		if g.Flags&text.FlagTruncator != 0 {
			res.endedWithNewline = res.endedWithNewline || res.runes == 0
			bounds.Min.Y, bounds.Max.Y = min(bounds.Min.Y, logical.Min.Y), max(bounds.Max.Y, logical.Max.Y)
			break
		}
		if !seen {
			seen, firstX, cluster, baseline = true, g.X, g.X, g.Y
		}
		res.runes += int(g.Runes)
		res.boxes = appendCluster(res.boxes, g, &cluster, firstX, baseline)
		res.endedWithNewline = res.endedWithNewline || g.Flags&text.FlagLineBreak != 0 && g.Flags&text.FlagParagraphBreak != 0
		if g.Flags&text.FlagLineBreak != 0 {
			linesSeen++
		}
		if linesSeen == 1 && g.Flags&text.FlagParagraphBreak != 0 {
			break
		}
		if !started {
			started, bounds = true, logical
		}
		below := logical.Min.Y > viewport.Max.Y
		if logical.Max.Y >= 0 && !below && logical.Max.X >= 0 && logical.Min.X <= viewport.Max.X {
			bounds = bounds.Union(logical)
		}
		if below {
			break
		}
	}
	res.width, res.height = bounds.Dx(), bounds.Dy()
	return res
}

// appendCluster adds the boxes of the runes g ends a cluster of, sharing out
// the cluster's advance; a glyph inside a cluster holds no rune of its own. A
// glyph on a line under the piece's first, a word wrapped inside the piece,
// keeps that line's top and its own height.
func appendCluster(boxes []runeBox, g text.Glyph, cluster *fixed.Int26_6, firstX fixed.Int26_6, baseline int32) []runeBox {
	if g.Runes == 0 {
		return boxes
	}
	start, end := *cluster, g.X+g.Advance
	if g.X < start {
		start = g.X
	}
	*cluster = end
	step := (end - start) / fixed.Int26_6(g.Runes)
	top, height := 0, 0
	if g.Y != baseline {
		top, height = int(g.Y-baseline), (g.Ascent + g.Descent).Ceil()
	}
	for r := range int(g.Runes) {
		x0 := start + step*fixed.Int26_6(r) - firstX
		boxes = append(boxes, runeBox{x0: x0.Round(), x1: (x0 + step).Round(), top: top, height: height})
	}
	return boxes
}

func runeBytes(s string, runes int) int {
	b := 0
	for range runes {
		_, n := utf8.DecodeRuneInString(s[b:])
		b += n
	}
	return b
}

// hit is the rune boundary nearest p, on the line p is over or the nearest
// one. under is the rune p is over instead, for picking a word.
func (g *cellGeometry) hit(p f32.Point, under bool) int {
	if len(g.boxes) == 0 {
		return 0
	}
	y, x := int(p.Y), int(p.X)
	top := g.boxes[0].top
	for _, b := range g.boxes {
		if b.top <= y {
			top = max(top, b.top)
		}
	}
	best, bestD := -1, 0
	for i, b := range g.boxes {
		if b.top != top {
			continue
		}
		if under && x >= b.x0 && x < b.x1 {
			return i
		}
		for k, edge := range [2]int{b.x0, b.x1} {
			if d := abs(x - edge); best < 0 || d < bestD {
				best, bestD = i+k, d
			}
		}
	}
	if under && best == len(g.boxes) {
		best--
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
