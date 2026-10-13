package main

import (
	"fmt"
	"strings"

	"gioui.org/layout"

	"github.com/lichdotdev/godemirror"
)

// cardBlock is one widget hung under a line of a file card's diff; key names
// it across frames, so its height and its place survive a redraw.
type cardBlock struct {
	key    string
	at     godemirror.LineRef
	widget layout.Widget
}

// cardBlocks is a card's blocks as the view holds them, and each one's height
// as last laid out: the card is as tall as its rows plus these, so the panel
// scrolls and the editor never does.
type cardBlocks struct {
	// placed is the signature of the blocks the view was last given: every
	// SetBlocks highlights the content again, so it is called only on a change.
	placed  string
	heights map[string]int
	seen    map[string]bool
	total   int
}

// place hands the view blocks when they differ from the ones it holds.
func (b *cardBlocks) place(view *godemirror.DiffView, blocks []cardBlock) {
	keys := make([]string, len(blocks))
	for i, bl := range blocks {
		keys[i] = fmt.Sprint(bl.key, "@", bl.at.Side, ":", bl.at.Line)
	}
	sig := strings.Join(keys, "\n")
	if sig == b.placed {
		return
	}
	b.placed = sig
	out := make([]godemirror.DiffBlock, len(blocks))
	for i, bl := range blocks {
		out[i] = godemirror.DiffBlock{At: bl.at, Widget: b.measured(bl.key, bl.widget)}
	}
	view.SetBlocks(out)
}

// measured lays w out and records its height for the frame.
func (b *cardBlocks) measured(key string, w layout.Widget) layout.Widget {
	return func(gtx C) D {
		dims := w(gtx)
		if b.heights == nil {
			b.heights, b.seen = map[string]int{}, map[string]bool{}
		}
		b.heights[key], b.seen[key] = dims.Size.Y, true
		return dims
	}
}

// height is the blocks' room as the last frame measured it.
func (b *cardBlocks) height() int { return b.total }

func (b *cardBlocks) startFrame() { clear(b.seen) }

// endFrame drops the heights of blocks the view no longer laid out (closed,
// or under a line folded into a gap) and reports whether the room changed,
// which the next frame has to be drawn at.
func (b *cardBlocks) endFrame() bool {
	total := 0
	for key, h := range b.heights {
		if !b.seen[key] {
			delete(b.heights, key)
			continue
		}
		total += h
	}
	changed := total != b.total
	b.total = total
	return changed
}
