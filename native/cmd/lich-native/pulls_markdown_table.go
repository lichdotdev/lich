package main

import (
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/x/richtext"
	extast "github.com/yuin/goldmark/extension/ast"

	"github.com/lichdotdev/lich/native/ui"
)

// markdownTable is a rendered GFM table, its header row first.
type markdownTable struct {
	rows [][]*markdownCell
	// natural and minimum are each column's width on one line and at its
	// longest word, padding included, measured at metric.
	natural, minimum []int
	metric           unit.Metric
}

// renderTable renders each cell as Markdown, a short row padded to the
// header's columns; the header is mdProse's th, font-medium in the
// foreground.
func renderTable(th *ui.Theme, source []byte, table *extast.Table) *markdownTable {
	t := &markdownTable{}
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		_, header := row.(*extast.TableHeader)
		fg := th.MutedForeground
		if header {
			fg = th.Foreground
		}
		cells := make([]*markdownCell, 0, len(table.Alignments))
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			spans := renderNode(th, source, cell, fg)
			if header {
				for i := range spans {
					spans[i].Font.Weight = font.Medium
				}
			}
			cells = append(cells, &markdownCell{spans: spans})
		}
		for len(cells) < len(table.Alignments) {
			cells = append(cells, &markdownCell{})
		}
		t.rows = append(t.rows, cells)
	}
	return t
}

// markdownUnbounded is the width a cell is measured at for its natural size.
const markdownUnbounded = 1 << 20

// layout is mdProse's table: w-max up to the column's width, px-2 py-1
// cells, a 1px border around each. A table wider than the column shares the
// room out by each column's natural width, and its cells wrap.
func (t *markdownTable) layout(gtx C, th *ui.Theme, open func(url string)) D {
	padX, padY, line := gtx.Dp(ui.Space(2)), gtx.Dp(ui.Space(1)), gtx.Dp(1)
	columns := len(t.rows[0])
	t.measure(gtx, th, padX)
	widths := fitColumns(t.natural, t.minimum, gtx.Constraints.Max.X-(columns+1)*line)
	width := (columns + 1) * line
	for _, w := range widths {
		width += w
	}
	edges := []int{0}
	y := line
	for _, row := range t.rows {
		calls := make([]op.CallOp, len(row))
		height := 0
		for c, cell := range row {
			macro := op.Record(gtx.Ops)
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(max(widths[c]-2*padX, 0), markdownUnbounded)}
			height = max(height, cell.layout(g, th, open).Size.Y)
			calls[c] = macro.Stop()
		}
		x := line
		for c := range row {
			stack := op.Offset(image.Pt(x+padX, y+padY)).Push(gtx.Ops)
			calls[c].Add(gtx.Ops)
			stack.Pop()
			x += widths[c] + line
		}
		y += height + 2*padY
		edges = append(edges, y)
		y += line
	}
	border := th.Over(th.Border)
	for _, e := range edges {
		paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(0, e, width, e+line)).Op())
	}
	x := 0
	for c := 0; c <= columns; c++ {
		paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(x, 0, x+line, y)).Op())
		if c < columns {
			x += line + widths[c]
		}
	}
	return D{Size: image.Pt(width, y)}
}

// measure takes the columns' widths again when the metric changed.
func (t *markdownTable) measure(gtx C, th *ui.Theme, padX int) {
	if t.natural != nil && t.metric == gtx.Metric {
		return
	}
	columns := len(t.rows[0])
	t.natural, t.minimum, t.metric = make([]int, columns), make([]int, columns), gtx.Metric
	for _, row := range t.rows {
		for c, cell := range row {
			t.natural[c] = max(t.natural[c], measureCell(gtx, th, cell)+2*padX)
			t.minimum[c] = max(t.minimum[c], longestWord(gtx, th, cell)+2*padX)
		}
	}
}

// measureCell is the cell's width on one line, laid out where nothing sees
// it.
func measureCell(gtx C, th *ui.Theme, cell *markdownCell) int {
	return measureSpans(gtx, th, &cell.state, cell.spans)
}

// longestWord is the width of the cell's widest word: laid out narrower
// than its constraints, a text reports the constraints, not the words.
func longestWord(gtx C, th *ui.Theme, cell *markdownCell) int {
	var scratch richtext.InteractiveText
	widest := 0
	for _, span := range cell.spans {
		for _, word := range strings.Fields(span.Content) {
			alone := span.DeepCopy()
			alone.Content = word
			widest = max(widest, measureSpans(gtx, th, &scratch, []richtext.SpanStyle{alone}))
		}
	}
	return widest
}

func measureSpans(gtx C, th *ui.Theme, state *richtext.InteractiveText, spans []richtext.SpanStyle) int {
	macro := op.Record(gtx.Ops)
	defer macro.Stop()
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(markdownUnbounded, markdownUnbounded)}
	return richtext.Text(state, th.Shaper, spans...).Layout(g).Size.X
}

// fitColumns is the columns' natural widths when they fit in room. Else each
// column keeps its longest word and the room left is shared out by how much
// more each one wants, so a narrow column is never squeezed into broken
// words; a table whose words alone overflow room is drawn at them.
func fitColumns(natural, minimum []int, room int) []int {
	totalNatural, totalMinimum := 0, 0
	for i := range natural {
		totalNatural, totalMinimum = totalNatural+natural[i], totalMinimum+minimum[i]
	}
	spare := room - totalMinimum
	if totalNatural <= room {
		return natural
	}
	if spare <= 0 {
		return minimum
	}
	out := make([]int, len(natural))
	for i := range natural {
		out[i] = minimum[i] + (natural[i]-minimum[i])*spare/(totalNatural-totalMinimum)
	}
	return out
}
