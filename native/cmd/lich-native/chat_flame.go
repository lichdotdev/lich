package main

import (
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// chatFlameFrames is the live turn's flame, a 9x11 sprite in 8 frames, row 0
// on top. 4 is the core, 2 the body, 3 the edge, 1 the base and the sparks
// that leave it, . nothing. The core's column, 4 on rows 7 to 9, is the same
// in every frame, so the flame sways round a fixed spine.
var chatFlameFrames = [...]string{
	"......... | ...3..... | ......... | ....3.... | ...33.... | ...333... | ..332331. | ..32423.. | 33224223. | .3224223. | ..11111..",
	"...3..... | ......... | .....3... | ....3.... | ....33... | ...333.1. | ..33233.. | 3332423.. | .3224223. | .3224223. | ..11111..",
	"......... | .....3... | ......... | .....3... | ...3.3... | ...333... | 1.33233.. | ..32423.. | .3224223. | .32242233 | ..11111..",
	".....3... | ......... | ....3.... | ...3..... | ...3.3... | 1..333... | ..33233.. | ..324233. | .32242233 | .3224223. | ..11111..",
	"......... | ...3..... | ......... | ....3.... | ...33.... | ...333... | ..33233.. | .332423.1 | .3224223. | .3224223. | ..11111..",
	"...3..... | ......... | .....3... | ....3.... | ....33... | ...333... | .333233.1 | .332423.. | .3224223. | .3224223. | ..11111..",
	"......... | .....3... | ......... | .....3... | ...3.3... | .1.333... | ..33233.. | ..32423.. | .3224223. | .3224223. | ..11111..",
	".....3... | ......... | ....3.... | ...3..... | .1.3.3... | ...333... | ..33233.. | ..324233. | .3224223. | .3224223. | ..11111..",
}

const (
	chatFlameCols = 9
	chatFlameRows = 11
	// chatFlameStep is how long a frame holds.
	chatFlameStep = 120 * time.Millisecond
	// chatFlameBody is how far the body goes from the muted foreground toward
	// the foreground.
	chatFlameBody = 0.55
)

// chatFlamePixel is one sprite pixel's side: 1.5dp, which is two device pixels
// at 1x and three at 2x, a whole number at the scales a window runs at.
const chatFlamePixel unit.Dp = 1.5

// chatFlameFrame is the frame showing at now, and when the next one starts.
func chatFlameFrame(now time.Time) (frame int, next time.Time) {
	tick := now.UnixNano() / int64(chatFlameStep)
	return int(tick % int64(len(chatFlameFrames))), time.Unix(0, (tick+1)*int64(chatFlameStep))
}

// chatFlameColor is the color of a sprite symbol, from the theme's own tokens
// so a gray theme draws a gray flame and a colored one colors it; false for
// the symbol that is nothing.
func chatFlameColor(th *ui.Theme, symbol byte) (color.NRGBA, bool) {
	switch symbol {
	case '4':
		return th.Foreground, true
	case '2':
		return ui.MixOKLab(th.MutedForeground, th.Foreground, chatFlameBody), true
	case '3':
		return th.MutedForeground, true
	case '1':
		return th.Accent, true
	}
	return color.NRGBA{}, false
}

// chatFlame draws the flame of the frame at gtx.Now and asks for the next one.
// Only a live turn draws it, so nothing is asked for once the turn is over.
func chatFlame(gtx C, th *ui.Theme) D {
	frame, next := chatFlameFrame(gtx.Now)
	// A window without focus keeps the frame it has and looks in less often.
	if th.Still {
		next = gtx.Now.Add(ui.StillFrame)
	}
	gtx.Execute(op.InvalidateCmd{At: next})
	px := max(gtx.Dp(chatFlamePixel), 1)
	for y, row := range strings.Split(chatFlameFrames[frame], " | ") {
		for x := 0; x < len(row); {
			c, ok := chatFlameColor(th, row[x])
			run := 1
			for x+run < len(row) && row[x+run] == row[x] {
				run++
			}
			if ok {
				rect := image.Rect(x*px, y*px, (x+run)*px, (y+1)*px)
				paint.FillShape(gtx.Ops, c, clip.Rect(rect).Op())
			}
			x += run
		}
	}
	return D{Size: image.Pt(chatFlameCols*px, chatFlameRows*px)}
}
