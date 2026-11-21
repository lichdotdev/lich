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

// chatFlameFrames is the live turn's flame, an 11x14 sprite in 16 frames, row 0
// on top: a bonfire of three tongues over a round foot. 4 is the core, 2 the
// body, 3 the edge and the sparks that leave a tongue, 1 the ember bed and a
// spark going out, . nothing. The core, 4 on column 5 of rows 10 to 12, is
// the same in every frame, so the tongues move over a fixed foot.
var chatFlameFrames = [...]string{
	"........... | ........... | ......1.... | ....3...... | ....33..... | ....33..... | ....333.... | ...33233... | ...32223... | ..3322233.. | ..3224223.. | .332242233. | .332242233. | ..1111111..",
	"........... | ........... | ....3...... | ........... | ........... | .....3..... | .....33.... | ..33.333... | ..3333233.. | .33222223.. | .32224223.. | .322242233. | .332242233. | ..1111111..",
	"........... | ........... | ...3....... | ........... | ......3.... | ......3.... | .....3333.. | .3...3223.. | .33.33223.. | .333322233. | .322242223. | .322242223. | .332242233. | ..1111111..",
	"........... | ........... | ...1....... | ........... | ......3.... | .....33.... | .....33..3. | .3..333.33. | .33.323333. | .333322223. | .322242223. | .322242223. | .332242233. | ..1111111..",
	"........... | ...1....... | ........... | ......3.... | .....33.... | .....33.... | ....333..3. | ..3.323.33. | ..33323333. | ..32222223. | ..32242223. | .332242223. | .332242233. | ..1111111..",
	"........... | ...1....... | ......3.... | ........... | .....3..... | ....33..... | ....333.... | ...3323.3.. | ..3322333.. | ..3222223.. | ..3224223.. | .332242233. | .332242233. | ..1111111..",
	"........... | ........... | .....3..... | ....3...... | ....33..... | ....33..... | ...333..... | ..332333... | ..3222233.. | .33222223.. | .32224223.. | .322242233. | .332242233. | ..1111111..",
	"........... | ........... | .....1..... | ....3...... | ....3...... | ....33..... | ....333.... | ....32333.. | .33.32223.. | .33332223.. | .322242233. | .322242223. | .332242233. | ..1111111..",
	"........... | ........... | ....31..... | ........... | ........... | ....3...... | ....33..... | .3..333.3.. | .33.323.33. | .333323333. | .322242223. | .322242223. | .332242233. | ..1111111..",
	"........... | .....1..... | ....3...... | ........... | ........... | .....3..... | .....33.... | .....33.... | ..3.3333.3. | ..33322333. | ..32242223. | .332242223. | .332242233. | ..1111111..",
	"........... | ...3....... | ........... | ........... | ......3.... | ......3.... | .....33.... | .....33.... | ..3333333.. | ..3222223.. | ..3224223.. | .332242233. | .332242233. | ..1111111..",
	"........... | ...1....... | ........... | ......3.... | ........... | ......3.... | .....33.... | .....33.... | ...333333.. | ..3322223.. | ..3224223.. | .332242233. | .332242233. | ..1111111..",
	"........... | ....1...... | ........... | ......3.... | .....33.... | .....33.... | ....333.... | ....32333.. | ..3332223.. | .33222223.. | .322242233. | .322242223. | .332242233. | ..1111111..",
	"........... | ....1...... | .......3... | .....3..... | ....33..... | ....33..... | ....33..3.. | ...3333.33. | .3.3223.33. | .333223333. | .322242223. | .322242223. | .332242233. | ..1111111..",
	"........... | ........... | ......1.... | ....3...... | ....3...... | ....33..... | ...333..... | ...3233..3. | ..33223.33. | ..32223333. | ..32242223. | .332242223. | .332242233. | ..1111111..",
	"........... | ........... | ......1.... | ....3...... | ....33..... | ....33..... | ....333.... | ...3323.... | ..3322333.. | ..3222223.. | ..3224223.. | .332242233. | .332242233. | ..1111111..",
}

const (
	chatFlameCols = 11
	chatFlameRows = 14
	// chatFlameLineRows is how many rows, from the foot up, the flame takes of
	// its line: the tongues' tips and the sparks burn over the line above, so
	// the foot centers on the verb and the line is no taller for them.
	chatFlameLineRows = 11
	// chatFlameStep is how long a frame holds.
	chatFlameStep = 100 * time.Millisecond
	// chatFlameEdge and chatFlameEmber are how much of Primary the edge and the
	// ember bed keep over the background.
	chatFlameEdge  = 0.65
	chatFlameEmber = 0.35
)

// chatFlamePixel is one sprite pixel's side: 1.5dp, which is two device pixels
// at 1x and three at 2x, a whole number at the scales a window runs at.
const chatFlamePixel unit.Dp = 1.5

// chatFlameFrame is the frame showing at now, and when the next one starts.
func chatFlameFrame(now time.Time) (frame int, next time.Time) {
	tick := now.UnixNano() / int64(chatFlameStep)
	return int(tick % int64(len(chatFlameFrames))), time.Unix(0, (tick+1)*int64(chatFlameStep))
}

// chatFlameColor is the color of a sprite symbol: the flame burns in the
// theme's Primary, as the sidebar's souls do, around a core in the foreground;
// false for the symbol that is nothing.
func chatFlameColor(th *ui.Theme, symbol byte) (color.NRGBA, bool) {
	switch symbol {
	case '4':
		return th.Foreground, true
	case '2':
		return th.Primary, true
	case '3':
		return ui.MixOKLab(th.Background, th.Primary, chatFlameEdge), true
	case '1':
		return ui.MixOKLab(th.Background, th.Primary, chatFlameEmber), true
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
	defer op.Offset(image.Pt(0, (chatFlameLineRows-chatFlameRows)*px)).Push(gtx.Ops).Pop()
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
	return D{Size: image.Pt(chatFlameCols*px, chatFlameLineRows*px)}
}
