package markdown

import (
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"
)

// Links is where what a body links to goes: Open takes a clicked URL, and
// Paths, when set, makes the paths written in code links too.
type Links struct {
	Open  func(url string)
	Paths *Paths
	// Quote, when set, draws a control for the body's selection while there
	// is one and no press is still making it, text being what Ctrl+C copies.
	// The body stands it over the selection's first line, outside its
	// layout, and a press on it keeps the selection.
	Quote func(gtx C, text string) D
	// quoteAt is the cell Quote stands over, -1 for none, and quoteText the
	// selection it is handed.
	quoteAt   int
	quoteText string
}

// Paths makes the paths written in code that Link takes open with Open on a
// click with the shortcut modifier (Ctrl, Cmd on macOS), as a terminal's do.
// Held is that modifier held down: only then does the hovered one underline
// and take a hand, so prose reads as prose until it is asked to link.
type Paths struct {
	Link func(path string) bool
	Open func(path string)
	Held bool
	// Version is what Link answers from, a comparable value such as the
	// listing's pointer: a body checks its path marks again when it changes.
	Version any
}

// unmarkUnlinked takes the file or folder mark off code that Link does not
// find and gives its room back: with a listing to look in, only code that
// names something there wears one, not a branch, a repository or a dotted
// name that only looks like a path.
func unmarkUnlinked(blocks []markdownBlock, link func(string) bool) {
	for i := range blocks {
		b := &blocks[i]
		if b.prose != nil {
			b.prose.unmarkUnlinked(link)
		}
		if b.table != nil {
			for _, row := range b.table.rows {
				for _, c := range row {
					c.unmarkUnlinked(link)
				}
			}
		}
		if b.list != nil {
			for j := range b.list.items {
				unmarkUnlinked(b.list.items[j].blocks, link)
			}
		}
		unmarkUnlinked(b.blocks, link)
	}
}

func (c *markdownCell) unmarkUnlinked(link func(string) bool) {
	for i := range c.decor {
		d := &c.decor[i]
		if d.path == "" || link(d.path) {
			continue
		}
		d.icon, d.path = nil, ""
		d.lead--
		c.spans[i].Content = "\u00a0" + strings.TrimPrefix(c.spans[i].Content, codeIconRoom)
	}
}

// pathPointer is whether the pointer is over a path's span, and the
// modifiers its last event there held.
type pathPointer struct {
	over bool
	mods key.Modifiers
	// pressed is a shortcut press on the path not yet let go: the path
	// opens on its release, unless the press became a selection.
	pressed bool
}

// followPath gives span i, a path size big, the pointer: it opens on a
// shortcut click that selected nothing, and reports whether it shows as a
// link this frame.
func (c *markdownCell) followPath(gtx C, i int, size image.Point, paths *Paths) bool {
	if len(c.paths) != len(c.decor) {
		c.paths = make([]pathPointer, len(c.decor))
	}
	p := &c.paths[i]
	kinds := pointer.Enter | pointer.Leave | pointer.Cancel | pointer.Move
	if paths.Held || p.pressed {
		// Taken only while it opens, so the span is no click target to
		// anything that reads them otherwise.
		kinds |= pointer.Press | pointer.Release
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: p, Kinds: kinds})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		p.mods = e.Modifiers
		switch e.Kind {
		case pointer.Enter, pointer.Move:
			p.over = true
		case pointer.Leave:
			p.over = false
		case pointer.Cancel:
			p.over, p.pressed = false, false
		case pointer.Press:
			p.pressed = e.Buttons.Contain(pointer.ButtonPrimary) && e.Modifiers.Contain(key.ModShortcut)
		case pointer.Release:
			if p.pressed && !c.dragged() {
				paths.Open(c.decor[i].path)
			}
			p.pressed = false
		}
	}
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, p)
	// Held alone can outlive the key: a release the frame never read, the
	// stage given to the file a click opened, leaves it set.
	link := p.over && paths.Held && p.mods.Contain(key.ModShortcut)
	if link {
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return link
}
