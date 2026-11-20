package markdown

import (
	"image"

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
}

// Paths makes the paths written in code that Link takes open with Open on a
// click with the shortcut modifier (Ctrl, Cmd on macOS), as a terminal's do.
// Held is that modifier held down: only then does the hovered one underline
// and take a hand, so prose reads as prose until it is asked to link.
type Paths struct {
	Link func(path string) bool
	Open func(path string)
	Held bool
}

// pathPointer is whether the pointer is over a path's span, and the
// modifiers its last event there held.
type pathPointer struct {
	over bool
	mods key.Modifiers
}

// followPath gives span i, a path size big, the pointer: it opens on a
// shortcut click, and reports whether it shows as a link this frame.
func (c *markdownCell) followPath(gtx C, i int, size image.Point, paths *Paths) bool {
	if len(c.paths) != len(c.decor) {
		c.paths = make([]pathPointer, len(c.decor))
	}
	p := &c.paths[i]
	kinds := pointer.Enter | pointer.Leave | pointer.Cancel | pointer.Move
	if paths.Held {
		// Taken only while it opens, so the span is no click target to
		// anything that reads them otherwise.
		kinds |= pointer.Press
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
		case pointer.Leave, pointer.Cancel:
			p.over = false
		case pointer.Press:
			if e.Buttons.Contain(pointer.ButtonPrimary) && e.Modifiers.Contain(key.ModShortcut) {
				paths.Open(c.decor[i].path)
			}
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
