package ui

import (
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
)

// Everything in the window that answers a click is a Clickable, a Bool or an
// Enum from this file, never gioui's own: each lays its area out under the
// hand cursor while it is enabled, so a control cannot forget it.
// TestNoGioClickableOutsideTheKit holds the window to that.

// HandCursor puts the hand over the clip in force, for a control that lays
// its own area out: a disabled gtx keeps the arrow.
func HandCursor(gtx C) {
	if gtx.Enabled() {
		pointer.CursorPointer.Add(gtx.Ops)
	}
}

// hand lays w out and puts the hand cursor over the box it took, under
// whatever cursor w itself declares inside.
func hand(gtx C, w layout.Widget) D {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	content := m.Stop()
	area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	HandCursor(gtx)
	content.Add(gtx.Ops)
	area.Pop()
	return dims
}

// Clickable is widget.Clickable under the hand cursor.
type Clickable struct{ widget.Clickable }

func (c *Clickable) Layout(gtx C, w layout.Widget) D {
	return c.Clickable.Layout(gtx, func(gtx C) D { return hand(gtx, w) })
}

// Focused is whether the keyboard is on c; gio keys its focus by the
// widget.Clickable inside, not by c.
func (c *Clickable) Focused(gtx C) bool { return gtx.Focused(&c.Clickable) }

// Bool is widget.Bool under the hand cursor.
type Bool struct{ widget.Bool }

func (b *Bool) Layout(gtx C, w layout.Widget) D {
	return b.Bool.Layout(gtx, func(gtx C) D { return hand(gtx, w) })
}

// Focused is whether the keyboard is on b; see Clickable.Focused.
func (b *Bool) Focused(gtx C) bool { return gtx.Focused(&b.Bool) }

// Enum is widget.Enum under the hand cursor, over each of its keys' areas.
type Enum struct{ widget.Enum }

func (e *Enum) Layout(gtx C, key string, w layout.Widget) D {
	return e.Enum.Layout(gtx, key, func(gtx C) D { return hand(gtx, w) })
}

// BoolOf is a Bool that starts as value.
func BoolOf(value bool) *Bool {
	b := new(Bool)
	b.Value = value
	return b
}

// EnumOf is an Enum that starts on value.
func EnumOf(value string) *Enum {
	e := new(Enum)
	e.Value = value
	return e
}
