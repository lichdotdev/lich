package diffview

import (
	"fmt"
	"strings"

	"gioui.org/layout"

	"github.com/lichdotdev/godemirror"
)

// Block is one widget hung under a line of a diff; Key names
// it across frames, so its height and its place survive a redraw.
type Block struct {
	Key    string
	At     godemirror.LineRef
	Widget layout.Widget
}

// Blocks is what hangs under a diff's lines as its view holds them, and each
// one's height as last laid out: the diff is as tall as its rows plus these, so
// a scrolling parent scrolls and the editor never does. The zero value is ready
// to use, and it can outlive the View it was drawn with.
type Blocks struct {
	// placed is the signature of the blocks the view was last given: every
	// SetBlocks highlights the content again, so it is called only on a change.
	placed string
	// moved is a SetBlocks this frame: the view still lays the old blocks
	// out in it, so the frame after has the room to measure.
	moved   bool
	heights map[string]int
	seen    map[string]bool
	total   int
}

// place hands the view blocks when they differ from the ones it holds.
func (b *Blocks) place(view *godemirror.DiffView, blocks []Block) {
	keys := make([]string, len(blocks))
	for i, bl := range blocks {
		keys[i] = fmt.Sprint(bl.Key, "@", bl.At.Side, ":", bl.At.Line)
	}
	sig := strings.Join(keys, "\n")
	if sig == b.placed {
		return
	}
	b.placed, b.moved = sig, true
	out := make([]godemirror.DiffBlock, len(blocks))
	for i, bl := range blocks {
		out[i] = godemirror.DiffBlock{At: bl.At, Widget: b.measured(bl.Key, bl.Widget)}
	}
	view.SetBlocks(out)
}

// measured lays w out and records its height for the frame.
func (b *Blocks) measured(key string, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		dims := w(gtx)
		if b.heights == nil {
			b.heights, b.seen = map[string]int{}, map[string]bool{}
		}
		b.heights[key], b.seen[key] = dims.Size.Y, true
		return dims
	}
}

// height is the blocks' room as the last frame measured it.
func (b *Blocks) height() int { return b.total }

func (b *Blocks) startFrame() { clear(b.seen) }

// endFrame drops the heights of blocks the view no longer laid out (closed,
// or under a line folded into a gap) and reports whether the room changed,
// or the blocks did, which the next frame has to be drawn at.
func (b *Blocks) endFrame() bool {
	total := 0
	for key, h := range b.heights {
		if !b.seen[key] {
			delete(b.heights, key)
			continue
		}
		total += h
	}
	changed := total != b.total || b.moved
	b.total, b.moved = total, false
	return changed
}
