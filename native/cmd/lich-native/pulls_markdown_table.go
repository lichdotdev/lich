package main

import (
	"image"
	"regexp"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/x/richtext"

	"github.com/lichdotdev/lich/native/ui"
)

// markdownSegment is a stretch of a Markdown source: prose, or a GFM table's
// cells, its header first.
type markdownSegment struct {
	text  string
	table [][]string
}

var (
	tableDelimiterRow = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
	codeFence         = regexp.MustCompile("^ {0,3}(```|~~~)")
)

// splitMarkdownTables cuts src into prose and GFM tables: a header row and a
// delimiter row, both with a pipe, then every following line that has one.
// Fenced code is never a table.
func splitMarkdownTables(src string) []markdownSegment {
	lines := strings.Split(src, "\n")
	var out []markdownSegment
	var prose []string
	flush := func() {
		if text := strings.Join(prose, "\n"); strings.TrimSpace(text) != "" {
			out = append(out, markdownSegment{text: text})
		}
		prose = nil
	}
	inFence := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if codeFence.MatchString(line) {
			inFence = !inFence
		}
		if inFence || !startsTable(lines, i) {
			prose = append(prose, line)
			continue
		}
		flush()
		header := tableCells(line)
		table := [][]string{header}
		i += 2
		for ; i < len(lines) && strings.Contains(lines[i], "|"); i++ {
			table = append(table, fitRow(tableCells(lines[i]), len(header)))
		}
		i--
		out = append(out, markdownSegment{table: table})
	}
	flush()
	return out
}

func startsTable(lines []string, i int) bool {
	return i+1 < len(lines) && strings.Contains(lines[i], "|") &&
		strings.Contains(lines[i+1], "|") && tableDelimiterRow.MatchString(lines[i+1])
}

// tableCells splits a row on its unescaped pipes, the outer ones dropped.
func tableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	if strings.HasSuffix(line, "|") && !strings.HasSuffix(line, `\|`) {
		line = line[:len(line)-1]
	}
	var cells []string
	var cell strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			cell.WriteByte('|')
			i++
		case line[i] == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteByte(line[i])
		}
	}
	return append(cells, strings.TrimSpace(cell.String()))
}

// fitRow pads or cuts a body row to the header's columns, as GFM does.
func fitRow(row []string, columns int) []string {
	if len(row) >= columns {
		return row[:columns]
	}
	return append(row, make([]string, columns-len(row))...)
}

// markdownTable is a rendered GFM table, its header row first.
type markdownTable struct {
	rows [][]*markdownCell
}

// renderTable renders each cell as Markdown; the header is mdProse's th,
// font-medium in the foreground.
func renderTable(th *ui.Theme, cells [][]string) *markdownTable {
	t := &markdownTable{rows: make([][]*markdownCell, len(cells))}
	for r, row := range cells {
		for _, text := range row {
			fg := th.MutedForeground
			if r == 0 {
				fg = th.Foreground
			}
			spans := renderMarkdown(th, text, fg)
			if r == 0 {
				for i := range spans {
					spans[i].Font.Weight = font.Medium
				}
			}
			t.rows[r] = append(t.rows[r], &markdownCell{spans: spans})
		}
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
	natural := make([]int, columns)
	for _, row := range t.rows {
		for c, cell := range row {
			natural[c] = max(natural[c], measureCell(gtx, th, cell)+2*padX)
		}
	}
	widths := fitColumns(natural, gtx.Constraints.Max.X-(columns+1)*line)
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

// measureCell is the cell's width on one line, laid out where nothing sees it.
func measureCell(gtx C, th *ui.Theme, cell *markdownCell) int {
	macro := op.Record(gtx.Ops)
	defer macro.Stop()
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(markdownUnbounded, markdownUnbounded)}
	return richtext.Text(&cell.state, th.Shaper, cell.spans...).Layout(g).Size.X
}

// fitColumns is the columns' natural widths when they fit in room, else room
// shared out in their proportion.
func fitColumns(natural []int, room int) []int {
	total := 0
	for _, w := range natural {
		total += w
	}
	if total <= room || total == 0 {
		return natural
	}
	out := make([]int, len(natural))
	for i, w := range natural {
		out[i] = max(w*max(room, 0)/total, 1)
	}
	return out
}
