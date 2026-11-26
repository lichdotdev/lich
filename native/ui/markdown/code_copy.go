package markdown

import (
	"image"
	"io"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"github.com/yuin/goldmark/ast"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icCodeCopy   = icons.Lucide("copy")
	icCodeCopied = icons.Lucide("check")
	// codeCopySide is ui.SizeIconXS's square, the room the button takes at
	// the panel's right.
	codeCopySide = ui.Space(6)
)

// codeCopiedFor is how long the button shows its check, as a message's copy
// button does.
const codeCopiedFor = 1500 * time.Millisecond

// codePanel is a code block's copy button, its fold and the pointer over
// the block. It outlives the block: a streaming answer renders its blocks
// again on every change, and a hover, a check or an opened fold must not
// flicker away with them.
type codePanel struct {
	click    ui.Clickable
	hovered  bool
	copiedAt time.Time
	more     ui.Clickable
	open     bool
}

func (c *codePanel) done(now time.Time) bool {
	return !c.copiedAt.IsZero() && now.Sub(c.copiedAt) < codeCopiedFor
}

// rawCode is the code of n as written: its tabs, and no line end after its
// last line.
func rawCode(source []byte, n ast.Node) string {
	var b strings.Builder
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(source))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// layout draws the button at at while the pointer is over the panel of size,
// putting code on the clipboard the way the window does everywhere.
func (c *codePanel) layout(gtx C, th *ui.Theme, code string, size, at image.Point) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &c.hovered, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			c.hovered = e.Kind == pointer.Enter
		}
	}
	if c.click.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(code))})
		c.copiedAt = gtx.Now
	}
	glyph := icCodeCopy
	if c.done(gtx.Now) {
		gtx.Execute(op.InvalidateCmd{At: c.copiedAt.Add(codeCopiedFor)})
		glyph = icCodeCopied
	}
	if c.hovered {
		off := op.Offset(at).Push(gtx.Ops)
		b := th.Button(&c.click, "")
		b.Icon, b.Variant, b.Size = glyph, ui.VariantGhost, ui.SizeIconXS
		gtx.Constraints = layout.Constraints{}
		b.Layout(gtx)
		off.Pop()
	}
	// Over the button, passing presses on: under it, the button would end
	// the hover it shows on.
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &c.hovered)
	pass.Pop()
	area.Pop()
}

// keepCopies hands each code block of blocks, in order, the button state the
// block at its place had before, so the state survives a new render.
func (t *markdownText) keepCopies() {
	i := 0
	eachPanel(t.blocks, func(b *markdownBlock) {
		if i < len(t.copies) {
			b.copy = t.copies[i]
		} else {
			t.copies = append(t.copies, b.copy)
		}
		i++
	})
}

func eachPanel(blocks []markdownBlock, f func(*markdownBlock)) {
	for i := range blocks {
		b := &blocks[i]
		if b.panel {
			f(b)
		}
		if b.list != nil {
			for j := range b.list.items {
				eachPanel(b.list.items[j].blocks, f)
			}
		}
		eachPanel(b.blocks, f)
	}
}
