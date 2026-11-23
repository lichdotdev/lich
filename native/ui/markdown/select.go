package markdown

import (
	"image"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

const (
	// multiClick is how close in time presses count as one double or triple
	// click: gesture.Click's doubleClickDuration, which the text fields read.
	multiClick = 400 * time.Millisecond
	// dragSlop is how far a press travels before it selects instead of
	// clicking: gesture.Drag's touchSlop.
	dragSlop = unit.Dp(3)
	// selectionSlack is the room past the text a press still lands in, so a
	// glyph overhanging its advance is not clipped.
	selectionSlack = unit.Dp(2)
)

// textSelection is the selected runes of one cell, from anchor to caret, and
// the press that is making it. It outlives the cell: a streaming answer
// renders its cells again on every change, and a drag must go on over them.
type textSelection struct {
	anchor, caret int
	pressed       bool
	// dragged is whether the last press moved past dragSlop: it selected,
	// and a link under it does not open.
	dragged   bool
	pid       pointer.ID
	pressAt   f32.Point
	pressedAt time.Duration
	clicks    int
	// elsewhere hears a press anywhere in the window, which drops the
	// selection unless it lands in bounds, the cell's area.
	elsewhere struct{ _ byte }
	bounds    image.Rectangle
}

func (s *textSelection) empty() bool { return s == nil || s.anchor == s.caret }

func (s *textSelection) clear() { s.anchor, s.caret = 0, 0 }

func (s *textSelection) span() (from, to int) {
	return min(s.anchor, s.caret), max(s.anchor, s.caret)
}

// cellText is the cell's runes as drawn, the room left for marks included.
func (c *markdownCell) cellText() string {
	var b strings.Builder
	for _, s := range c.spans {
		b.WriteString(s.Content)
	}
	return b.String()
}

// visibleText is the cell's runes [from, to) without the room the renderer
// leaves around inline code and before a mark: what a person reads there.
func (c *markdownCell) visibleText(from, to int) string {
	var b strings.Builder
	at := 0
	for i, s := range c.spans {
		n := utf8.RuneCountInString(s.Content)
		lead, trail := 0, 0
		if i < len(c.decor) {
			lead, trail = c.decor[i].lead, c.decor[i].trail
		}
		k := 0
		for _, r := range s.Content {
			if at+k >= from && at+k < to && k >= lead && k < n-trail {
				b.WriteRune(r)
			}
			k++
		}
		at += n
	}
	return b.String()
}

// selectEvents reads the pointer over the cell and the keys a selection
// answers before the cell is drawn.
func (c *markdownCell) selectEvents(gtx C, th *ui.Theme) {
	s := c.sel
	if s == nil {
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &s.elsewhere, Kinds: pointer.Press})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && !e.Position.Round().In(s.bounds) {
			s.clear()
		}
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: s, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			c.selectPointer(gtx, th, e)
		}
	}
	c.selectKeys(gtx)
}

func (c *markdownCell) selectPointer(gtx C, th *ui.Theme, e pointer.Event) {
	s := c.sel
	switch e.Kind {
	case pointer.Press:
		if !e.Buttons.Contain(pointer.ButtonPrimary) {
			return
		}
		near := e.Position.Sub(s.pressAt)
		if s.clicks > 0 && e.Time-s.pressedAt < multiClick && near.X*near.X+near.Y*near.Y <= slop(gtx) {
			s.clicks++
		} else {
			s.clicks = 1
		}
		s.pressed, s.dragged, s.pid, s.pressAt, s.pressedAt = true, false, e.PointerID, e.Position, e.Time
		g := c.geometry(gtx, th.Shaper)
		switch s.clicks {
		case 1:
			s.anchor = g.hit(e.Position, false)
			s.caret = s.anchor
		case 2:
			s.anchor, s.caret = wordAt([]rune(c.cellText()), g.hit(e.Position, true))
		default:
			s.anchor, s.caret = 0, utf8.RuneCountInString(c.cellText())
		}
	case pointer.Drag:
		if !s.pressed || e.PointerID != s.pid {
			return
		}
		if !s.dragged {
			d := e.Position.Sub(s.pressAt)
			if d.X*d.X+d.Y*d.Y <= slop(gtx) {
				return
			}
			s.dragged = true
			gtx.Execute(pointer.GrabCmd{Tag: s, ID: e.PointerID})
		}
		s.caret = c.geometry(gtx, th.Shaper).hit(e.Position, false)
	case pointer.Release, pointer.Cancel:
		s.pressed = false
	}
}

func slop(gtx C) float32 {
	d := float32(gtx.Dp(dragSlop))
	return d * d
}

// selectKeys copies the selection on Ctrl+C (Cmd+C on macOS) and drops it on
// Escape. Both are asked for only while there is one, so the keys reach a
// focused field otherwise.
func (c *markdownCell) selectKeys(gtx C) {
	s := c.sel
	for !s.empty() {
		ev, ok := gtx.Event(key.Filter{Name: "C", Required: key.ModShortcut}, key.Filter{Name: key.NameEscape})
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		if e.Name == key.NameEscape {
			s.clear()
			return
		}
		from, to := s.span()
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(c.visibleText(from, to)))})
	}
}

// SelectionKeys reads Ctrl+C and Escape for a selection in m. A screen whose
// focused field reads those keys before m is drawn calls it first, so a
// selection gets them: the chat's composer always holds the keyboard.
func (m Cache) SelectionKeys(gtx C) {
	for _, t := range m {
		for _, c := range t.cells {
			c.selectKeys(gtx)
		}
	}
}

// paintSelection fills the selected runes' boxes in the code views'
// selection color, under the glyphs.
func (c *markdownCell) paintSelection(gtx C, th *ui.Theme) {
	if c.sel.empty() {
		return
	}
	from, to := c.sel.span()
	g := c.geometry(gtx, th.Shaper)
	fill := th.Over(th.Code.Selection)
	for i := from; i < min(to, len(g.boxes)); i++ {
		b := g.boxes[i]
		paint.FillShape(gtx.Ops, fill, clip.Rect{Min: image.Pt(b.x0, b.top), Max: image.Pt(b.x1, b.top+b.height)}.Op())
	}
}

// selectArea makes size the cell's selectable area, the text cursor over it,
// and draws text inside it; while there is a selection, a press anywhere
// else in the window is heard to drop it.
func (c *markdownCell) selectArea(gtx C, size image.Point, text op.CallOp) {
	if c.sel == nil {
		text.Add(gtx.Ops)
		return
	}
	slack := gtx.Dp(selectionSlack)
	c.sel.bounds = image.Rectangle{Max: size}.Inset(-slack)
	area := clip.Rect(c.sel.bounds).Push(gtx.Ops)
	event.Op(gtx.Ops, c.sel)
	// The text cursor sits on an area of its own under the text: on the one
	// that holds it, every fill drawn over a link or a path would take its
	// cursor from there and hide their hand.
	beam := clip.Rect(c.sel.bounds).Push(gtx.Ops)
	pointer.CursorText.Add(gtx.Ops)
	beam.Pop()
	text.Add(gtx.Ops)
	area.Pop()
	if c.sel.empty() {
		return
	}
	// Deferred, the area is laid over the whole window, past the clips of
	// whatever the cell is drawn in, and passes every press on.
	m := op.Record(gtx.Ops)
	whole := clip.Rect{Min: image.Pt(-markdownUnbounded, -markdownUnbounded), Max: image.Pt(markdownUnbounded, markdownUnbounded)}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &c.sel.elsewhere)
	pass.Pop()
	whole.Pop()
	op.Defer(gtx.Ops, m.Stop())
}

// wordClass sorts runes into words, spaces and the rest, so a double click
// takes a run of one of them.
func wordClass(r rune) int {
	switch {
	case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_':
		return 0
	case unicode.IsSpace(r):
		return 1
	}
	return 2
}

// wordAt is the run of runes of one class around rune i.
func wordAt(runes []rune, i int) (from, to int) {
	if len(runes) == 0 {
		return 0, 0
	}
	i = min(max(i, 0), len(runes)-1)
	class := wordClass(runes[i])
	from, to = i, i+1
	for from > 0 && wordClass(runes[from-1]) == class {
		from--
	}
	for to < len(runes) && wordClass(runes[to]) == class {
		to++
	}
	return from, to
}

// keepSelections hands each cell of t, in order, the selection the cell at its
// place had before, so a drag survives a new render. A selection stays only
// while the text up to its end is what it was: a streaming answer grows at
// its end and blinks its caret there.
func (t *markdownText) keepSelections() {
	before := t.cells
	t.cells = cellsOf(t.blocks)
	for i, c := range t.cells {
		if i == len(t.sels) {
			t.sels = append(t.sels, new(textSelection))
		}
		s := t.sels[i]
		c.sel = s
		if s.empty() {
			continue
		}
		_, to := s.span()
		if i >= len(before) || !samePrefix(before[i].cellText(), c.cellText(), to) {
			s.clear()
		}
	}
}

// samePrefix is whether a and b hold the same first n runes.
func samePrefix(a, b string, n int) bool {
	if utf8.RuneCountInString(a) < n || utf8.RuneCountInString(b) < n {
		return false
	}
	return a[:runeBytes(a, n)] == b[:runeBytes(b, n)]
}

// cellsOf is every cell of blocks a selection can be made in, in the order
// they are written: a list's markers are not text.
func cellsOf(blocks []markdownBlock) []*markdownCell {
	var out []*markdownCell
	for i := range blocks {
		b := &blocks[i]
		if b.prose != nil {
			out = append(out, b.prose)
		}
		if b.table != nil {
			for _, row := range b.table.rows {
				out = append(out, row...)
			}
		}
		if b.list != nil {
			for j := range b.list.items {
				out = append(out, cellsOf(b.list.items[j].blocks)...)
			}
		}
		out = append(out, cellsOf(b.blocks)...)
	}
	return out
}
