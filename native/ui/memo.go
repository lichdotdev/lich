package ui

import (
	"hash/maphash"
	"image"
	"image/color"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
)

// Memo replays the operations a subtree recorded while its key holds,
// instead of laying the subtree out again: a frame drawn for an animation
// elsewhere in the window costs the subtree nothing. It lays the subtree out
// again on a frame with input, or once a frame the subtree asked for falls
// due, so its widgets take the input and their animations run (Gio's
// input.KeepOp).
type Memo struct {
	ops  op.Ops
	call op.CallOp
	dims layout.Dimensions
	key  MemoKey
	ok   bool
}

// MemoKey is what a memoized subtree draws from besides its operations:
// Version must change with everything the subtree shows that no input
// brings, its content and what arrives for it asynchronously.
type MemoKey struct {
	Constraints layout.Constraints
	// Theme is the theme's Generation.
	Theme   uint64
	Version uint64
	// Rect is where the subtree sits, for one that draws or hit-tests in
	// window coordinates.
	Rect image.Rectangle
}

// Layout draws w, or what it drew for the same key.
func (m *Memo) Layout(gtx layout.Context, key MemoKey, w layout.Widget) layout.Dimensions {
	if !m.ok || m.key != key || gtx.Source.Queued(gtx.Now) {
		m.ops.Reset()
		rec := gtx
		rec.Ops = &m.ops
		macro := op.Record(rec.Ops)
		keep := input.KeepOp{}.Push(rec.Ops)
		m.dims = w(rec)
		keep.Pop()
		m.call = macro.Stop()
		m.key, m.ok = key, true
	}
	m.call.Add(gtx.Ops)
	return m.dims
}

// Drop makes the next Layout lay the subtree out again, for a subtree that
// failed to draw.
func (m *Memo) Drop() { m.ok = false }

var themeSeed = maphash.MakeSeed()

// themeLook is what of a theme changes how a subtree draws.
type themeLook struct {
	Palette
	surface color.NRGBA
	still   bool
	shaper  *text.Shaper
}

// Generation identifies how th draws: two themes with the same Generation
// paint the same, whichever On they came from.
func (th *Theme) Generation() uint64 {
	return maphash.Comparable(themeSeed, themeLook{th.Palette, th.Surface, th.Still, th.Shaper})
}
