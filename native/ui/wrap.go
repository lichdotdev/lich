package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// Justify is Tailwind's justify-* on a Wrap.
type Justify uint8

const (
	JustifyStart Justify = iota
	JustifyEnd
	JustifyBetween
)

// Wrap is Tailwind's `flex flex-wrap items-center gap-x-* gap-y-*`: children
// left to right at their natural size, a new line when the width runs out,
// each centered on its line.
//
// Its width follows a flex container that is itself a flex item: one line is
// as wide as its children, more than one fills the maximum, as the browser
// clamps max-content to the space available. A block-level container (w-full,
// a footer) sets Constraints.Min.X to the maximum first.
type Wrap struct {
	GapX, GapY unit.Dp
	Justify    Justify
	// ContentStart is content-start: the lines sit at the top of a taller
	// minimum height. Without it they share that height out, CSS's default
	// align-content, so FooterBar's one line is centered in its min-h-11.
	ContentStart bool
}

// WrapChild is one child of a Wrap.
type WrapChild struct {
	widget     layout.Widget
	marginAuto bool
}

// WrapItem is a plain child.
func WrapItem(w layout.Widget) WrapChild { return WrapChild{widget: w} }

// WrapMLAuto is a child with ml-auto: it takes its line's free space before
// it, which overrides Justify on that line. FooterBar's right section wears it
// so it still hugs the right edge once it wraps onto a line of its own, where
// justify-between alone would put it at the start.
func WrapMLAuto(w layout.Widget) WrapChild { return WrapChild{widget: w, marginAuto: true} }

type wrapped struct {
	call       op.CallOp
	size       image.Point
	marginAuto bool
}

func (w Wrap) Layout(gtx C, children ...WrapChild) D {
	lines := w.lines(gtx, w.measure(gtx, children))
	width := gtx.Constraints.Min.X
	if len(lines) > 1 {
		width = gtx.Constraints.Max.X
	} else if len(lines) == 1 {
		width = min(max(width, w.lineWidth(gtx, lines[0])), gtx.Constraints.Max.X)
	}
	heights := w.lineHeights(gtx, lines)
	y := 0
	for i, line := range lines {
		if i > 0 {
			y += gtx.Dp(w.GapY)
		}
		w.place(gtx, line, width, y, heights[i])
		y += heights[i]
	}
	return D{Size: image.Pt(width, max(y, gtx.Constraints.Min.Y))}
}

// lineHeights is each line's tallest child, plus its share of what the
// minimum height leaves over unless ContentStart.
func (w Wrap) lineHeights(gtx C, lines [][]wrapped) []int {
	heights := make([]int, len(lines))
	total := gtx.Dp(w.GapY) * max(len(lines)-1, 0)
	for i, line := range lines {
		for _, it := range line {
			heights[i] = max(heights[i], it.size.Y)
		}
		total += heights[i]
	}
	extra := gtx.Constraints.Min.Y - total
	if w.ContentStart || extra <= 0 {
		return heights
	}
	for i := range heights {
		heights[i] += extra*(i+1)/len(heights) - extra*i/len(heights)
	}
	return heights
}

func (w Wrap) measure(gtx C, children []WrapChild) []wrapped {
	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Max: gtx.Constraints.Max}
	items := make([]wrapped, len(children))
	for i, c := range children {
		m := op.Record(gtx.Ops)
		d := c.widget(cgtx)
		items[i] = wrapped{call: m.Stop(), size: d.Size, marginAuto: c.marginAuto}
	}
	return items
}

// lines breaks items greedily, as flex-wrap does: an item that does not fit
// after the last one on a line starts the next, and an item wider than the
// whole width still gets a line to itself.
func (w Wrap) lines(gtx C, items []wrapped) [][]wrapped {
	var lines [][]wrapped
	gap, x := gtx.Dp(w.GapX), 0
	for i, it := range items {
		if i == 0 || x+gap+it.size.X > gtx.Constraints.Max.X {
			lines = append(lines, nil)
			x = it.size.X
		} else {
			x += gap + it.size.X
		}
		lines[len(lines)-1] = append(lines[len(lines)-1], it)
	}
	return lines
}

func (w Wrap) lineWidth(gtx C, line []wrapped) int {
	width := gtx.Dp(w.GapX) * (len(line) - 1)
	for _, it := range line {
		width += it.size.X
	}
	return width
}

// place draws line in the band height tall at y, width wide.
func (w Wrap) place(gtx C, line []wrapped, width, y, height int) {
	autos := 0
	for _, it := range line {
		if it.marginAuto {
			autos++
		}
	}
	free := max(width-w.lineWidth(gtx, line), 0)
	x, spread := 0, 0
	switch {
	case autos > 0:
	case w.Justify == JustifyEnd:
		x = free
	case w.Justify == JustifyBetween && len(line) > 1:
		spread = free
	}
	for i, it := range line {
		if it.marginAuto {
			x += free / autos
		}
		at := x
		if spread > 0 {
			at += spread * i / (len(line) - 1)
		}
		off := op.Offset(image.Pt(at, y+(height-it.size.Y)/2)).Push(gtx.Ops)
		it.call.Add(gtx.Ops)
		off.Pop()
		x += it.size.X + gtx.Dp(w.GapX)
	}
}
