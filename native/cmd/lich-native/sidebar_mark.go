package main

import (
	"image"
	"image/color"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/grimoire"
)

// markPixel is one cell of a provider's mark: a whole device pixel at every
// scale, the mark's own size beside the label.
var markPixel = unit.Dp(1)

// markColors are the providers' brand colors, read off each one's own logo.
// A provider whose logo is black and white has none and paints its mark in
// Foreground. They are literals on purpose: a brand's color is the same in
// every theme.
var markColors = map[string]color.NRGBA{
	"claude": {R: 0xd9, G: 0x77, B: 0x57, A: 0xff},
	// Codex's logo is a gradient; its middle.
	"codex":       {R: 0x6e, G: 0x7b, B: 0xff, A: 0xff},
	"antigravity": {R: 0x42, G: 0x85, B: 0xf4, A: 0xff},
	"kiro":        {R: 0x90, G: 0x46, B: 0xff, A: 0xff},
	"crush":       {R: 0xff, G: 0x5f, B: 0xd2, A: 0xff},
}

// markHalf is how strongly a mark's half-lit cells (2) take the ink: opencode's
// logo is a frame half lit inside.
const markHalf = 0.35

// marks memoizes grimoire.Mark by provider: a crop of the book, read on every
// row of every frame.
var marks = map[string]grimoire.Grid{}

// providerMark is kind's pixel mark, nil for a provider with no book.
func providerMark(kind string) grimoire.Grid {
	g, ok := marks[kind]
	if ok {
		return g
	}
	g = grimoire.Mark(kind)
	marks[kind] = g
	return g
}

// pixelGrid paints g's mark cells in c (its half-lit ones fainter), centered
// in gtx's minimum size.
func pixelGrid(gtx C, g grimoire.Grid, c color.NRGBA) D {
	px := max(1, gtx.Dp(markPixel))
	box := gtx.Constraints.Min
	w, h := len(g[0])*px, len(g)*px
	defer op.Offset(image.Pt((box.X-w)/2, (box.Y-h)/2)).Push(gtx.Ops).Pop()
	for y, row := range g {
		for x, cell := range row {
			if cell == '.' {
				continue
			}
			ink := c
			if cell == '2' {
				ink.A = uint8(float32(c.A) * markHalf)
			}
			paint.FillShape(gtx.Ops, ink, clip.Rect(image.Rect(x*px, y*px, (x+1)*px, (y+1)*px)).Op())
		}
	}
	return D{Size: box}
}
