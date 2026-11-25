package main

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// labeledGlyph draws glyph, a state the eye reads alone, with word as its
// label for whoever reads the window without seeing it.
func labeledGlyph(gtx C, word string, glyph layout.Widget) D {
	d := glyph(gtx)
	defer clip.Rect{Max: d.Size}.Push(gtx.Ops).Pop()
	semantic.LabelOp(word).Add(gtx.Ops)
	return d
}
