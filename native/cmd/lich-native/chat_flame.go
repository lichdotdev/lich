package main

import (
	"image"
	"image/color"

	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

const (
	// chatFlameEdge and chatFlameEmber are how much of Primary the edge and the
	// ember bed keep over the background.
	chatFlameEdge  = 0.65
	chatFlameEmber = 0.35
	// chatFlameCore is how far the core goes from Primary toward the lighter
	// of the theme's ink and ground.
	chatFlameCore = 0.5
)

// chatSpinnerSize is the spinner's side. It scales as a whole: rounding each
// of its eleven sprite pixels first halved it at a 90% zoom.
const chatSpinnerSize unit.Dp = 22

// chatFlameColor is the color of a sprite symbol: the flame burns in the
// theme's Primary, as the spinner does, around a core of the same hue,
// lighter; false for the symbol that is nothing.
func chatFlameColor(th *ui.Theme, symbol byte) (color.NRGBA, bool) {
	switch symbol {
	case '4':
		return ui.MixOKLab(th.Primary, lighter(th.Foreground, th.Background), chatFlameCore), true
	case '2':
		return th.Primary, true
	case '3':
		return ui.MixOKLab(th.Background, th.Primary, chatFlameEdge), true
	case '1':
		return ui.MixOKLab(th.Background, th.Primary, chatFlameEmber), true
	}
	return color.NRGBA{}, false
}

// chatSpinner draws the sidebar's working snake, over the chat's background,
// at gtx.Now and asks for its next frame. Only a live turn draws it, so
// nothing is asked for once the turn is over.
func chatSpinner(gtx C, th *ui.Theme) D {
	a, next := spinnerAt(gtx.Now)
	nextFrame(gtx, th, next.Sub(gtx.Now))
	side := max(spriteCells, gtx.Dp(chatSpinnerSize))
	drawCells(gtx, side, th.Background, th.Primary, a)
	return D{Size: image.Pt(side, side)}
}

// lighter is whichever of a and b is lighter, the side light comes from in
// a dark theme and in a light one alike.
func lighter(a, b color.NRGBA) color.NRGBA {
	if int(a.R)+int(a.G)+int(a.B) >= int(b.R)+int(b.G)+int(b.B) {
		return a
	}
	return b
}
