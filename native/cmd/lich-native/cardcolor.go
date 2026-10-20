package main

import (
	"image/color"

	"github.com/lichdotdev/lich/native/ui"
)

var cardColors = map[string]color.NRGBA{
	"red":    ui.OKLCH(0.637, 0.237, 25.331, 1),
	"orange": ui.OKLCH(0.705, 0.213, 47.604, 1),
	"amber":  ui.OKLCH(0.769, 0.188, 70.08, 1),
	"green":  ui.OKLCH(0.723, 0.219, 149.579, 1),
	"teal":   ui.OKLCH(0.704, 0.14, 182.503, 1),
	"blue":   ui.OKLCH(0.623, 0.214, 259.815, 1),
	"violet": ui.OKLCH(0.606, 0.25, 292.717, 1),
	"pink":   ui.OKLCH(0.656, 0.241, 354.308, 1),
}

// cardColorNames is the swatch order.
var cardColorNames = []string{"red", "orange", "amber", "green", "teal", "blue", "violet", "pink"}

// The tint's share of each fill.
const (
	tintRest   = 0.10
	tintHover  = 0.22
	tintActive = 0.38
)

// hoverFill is the accent's alpha on an untinted card under the pointer.
const hoverFill = 0.6

// cardFill is the card's background for its state, composited onto the
// surface it sits on.
func cardFill(th *ui.Theme, tint string, active, hovered bool) color.NRGBA {
	return th.Over(cardLayer(th, tint, active, hovered))
}

// cardLayer is the card's fill, translucent where it is tinted or hovered;
// transparent for an untinted card at rest.
func cardLayer(th *ui.Theme, tint string, active, hovered bool) color.NRGBA {
	c, tinted := cardColors[tint]
	switch {
	case active && tinted:
		return ui.MixOKLab(th.Accent, c, tintActive)
	case active:
		return th.Accent
	case hovered && tinted:
		return ui.MixOKLab(th.Accent, c, tintHover)
	case hovered:
		return ui.Alpha(th.Accent, hoverFill)
	case tinted:
		return ui.MixOKLab(color.NRGBA{}, c, tintRest)
	}
	return color.NRGBA{}
}
