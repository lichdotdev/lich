package main

import (
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/ui"
)

// statusSprite is a status the corner slot draws: 11x11 frames like the
// spinner's, row 0 on top, each held for its own step.
type statusSprite struct {
	frames []string
	steps  []time.Duration
}

func (s statusSprite) length() time.Duration {
	var t time.Duration
	for _, d := range s.steps {
		t += d
	}
	return t
}

// at is the frame showing since into the sprite looped, and how long until
// the next one.
func (s statusSprite) at(since time.Duration) (frame int, left time.Duration) {
	since %= s.length()
	if since < 0 {
		since += s.length()
	}
	for i, d := range s.steps {
		if since < d {
			return i, d - since
		}
		since -= d
	}
	return 0, s.steps[0]
}

// hourglassSprite is a session waiting on you: the sand runs down in four steps,
// the glass stays empty a moment, then turns clockwise and starts again.
var hourglassSprite = statusSprite{
	frames: []string{
		".445555555. | ..5444443.. | ..5444443.. | ...54443... | ....543.... | .....3..... | ....513.... | ...51113... | ..5111113.. | ..5111113.. | .333333311.",
		".445555555. | ..5111113.. | ..5444443.. | ...54443... | ....543.... | .....3..... | ....553.... | ...51113... | ..5111113.. | ..5144413.. | .333333311.",
		".445555555. | ..5111113.. | ..5144413.. | ...54443... | ....543.... | .....3..... | ....553.... | ...51113... | ..5111113.. | ..5444453.. | .333333311.",
		".445555555. | ..5111113.. | ..5111113.. | ...51413... | ....543.... | .....3..... | ....553.... | ...51113... | ..5144413.. | ..5444453.. | .333333311.",
		".445555555. | ..5111113.. | ..5111113.. | ...51113... | ....513.... | .....3..... | ....513.... | ...51413... | ..5444453.. | ..5444453.. | .333333311.",
		"......4.... | .....555... | .....5115.. | .....51115. | .....511135 | .555533333. | 334443..... | .34443..... | ..3453..... | ...333..... | ....1......",
		"........... | 3.........4 | 355.....554 | 3445...5115 | 34415.51115 | 34441311115 | 34413.31115 | 3553...3115 | 133.....335 | 1.........5 | ...........",
		"....3...... | ...335..... | ..3445..... | .34445..... | 135445..... | .333335555. | .....311154 | .....31115. | .....3115.. | .....335... | ......5....",
	},
	steps: []time.Duration{500 * time.Millisecond, 500 * time.Millisecond, 500 * time.Millisecond, 500 * time.Millisecond, 600 * time.Millisecond, 90 * time.Millisecond, 90 * time.Millisecond, 90 * time.Millisecond},
}

// soulFusion is a turn ending: the three souls of the spinner, met in its
// center, fuse into one. It plays once, from the spinner's soulsMeet frame.
var soulFusion = statusSprite{
	frames: []string{
		"........... | ........... | ........... | ........... | ........... | ....351.... | ....351.... | ....111.... | ........... | ........... | ...........",
		"........... | ........... | ........... | ........... | .....1..... | ....553.... | ...35451... | ...35553... | ....333.... | ........... | ...........",
		"........... | ........... | ........... | .....1..... | ....331.... | ....351.... | ...55553... | ..5544453.. | ..3544453.. | ...35553... | ....333....",
		"........... | ........... | .....1..... | ....331.... | ....331.... | ...55553... | ..5555553.. | ..5544453.. | ..3544453.. | ...35553... | ....333....",
		".....1..... | .....1..... | ....31..... | ....553.... | ...5553.... | ...55553... | ..5555553.. | ..5544453.. | ..3544453.. | ...35553... | ....333....",
	},
	steps: []time.Duration{90 * time.Millisecond, 90 * time.Millisecond, 90 * time.Millisecond, 90 * time.Millisecond, 140 * time.Millisecond},
}

// soulRest is a finished turn nobody read: the one soul breathing, looped.
var soulRest = statusSprite{
	frames: []string{
		"........... | .....1..... | ....31..... | ....351.... | ...5553.... | ...55553... | ..5555553.. | ..5544453.. | ..3544453.. | ...35553... | ....333....",
		"......1.... | .....31.... | .....31.... | ....553.... | ...55553... | ...55553... | ..5555553.. | ..5544453.. | ..3544453.. | ...35553... | ....333....",
		"........... | ......1.... | .....31.... | ....351.... | ....5553... | ...55553... | ..5555553.. | ..5544453.. | ..3544453.. | ...35553... | ....333....",
		".....3..... | ........... | .....1..... | ....331.... | ...35351... | ...55553... | ..5555553.. | ..5544453.. | ..3544453.. | ...35553... | ....333....",
		"........... | ....1...... | ....31..... | ....351.... | ...5553.... | ...55553... | ..5555553.. | ..5544453.. | ..3544453.. | ...35553... | ....333....",
	},
	steps: []time.Duration{900 * time.Millisecond, 180 * time.Millisecond, 180 * time.Millisecond, 180 * time.Millisecond, 180 * time.Millisecond},
}

// soulsMeet is the spinner's frame with the three souls together in the
// center, where the fusion starts so the corner never jumps.
const soulsMeet = 10

// fusionStart is when a turn that ended at end starts its fusion: the next
// time the spinner reaches soulsMeet.
func fusionStart(end time.Time) time.Time {
	tick := end.UnixNano() / int64(soulsStep)
	n := int64(len(soulsFrames))
	wait := ((soulsMeet-tick)%n + n) % n
	return time.Unix(0, (tick+wait)*int64(soulsStep))
}

// spriteInk is a status symbol's color: 4 Foreground, 5 halfway from
// MutedForeground to it, 2 the chat flame's body, 3 MutedForeground, 1 Muted.
func spriteInk(th *ui.Theme, symbol byte) (color.NRGBA, bool) {
	switch symbol {
	case '5':
		return ui.MixOKLab(th.MutedForeground, th.Foreground, 0.5), true
	case '1':
		return th.Muted, true
	}
	return chatFlameColor(th, symbol)
}

// drawSprite paints rows, an 11x11 frame, centered in gtx's minimum size, each
// cell the whole device pixel nearest to filling soulsSize.
func drawSprite(gtx C, th *ui.Theme, rows string) D {
	px := max(1, (gtx.Dp(soulsSize)+soulsCells/2)/soulsCells)
	side := soulsCells * px
	box := gtx.Constraints.Min
	defer op.Offset(image.Pt((box.X-side)/2, (box.Y-side)/2)).Push(gtx.Ops).Pop()
	for y, row := range strings.Split(rows, " | ") {
		for x := 0; x < len(row); x++ {
			if c, ok := spriteInk(th, row[x]); ok {
				paint.FillShape(gtx.Ops, c, clip.Rect(image.Rect(x*px, y*px, (x+1)*px, (y+1)*px)).Op())
			}
		}
	}
	return D{Size: box}
}

// nextFrame asks for the frame after left, or a still window's slower look.
func nextFrame(gtx C, th *ui.Theme, left time.Duration) {
	at := gtx.Now.Add(left)
	if th.Still {
		at = gtx.Now.Add(ui.StillFrame)
	}
	gtx.Execute(op.InvalidateCmd{At: at})
}

// loop draws s at gtx.Now from the clock's own origin and asks for its next
// frame.
func loop(gtx C, th *ui.Theme, s statusSprite, since time.Duration) D {
	f, left := s.at(since)
	nextFrame(gtx, th, left)
	return drawSprite(gtx, th, s.frames[f])
}

// statusCorner draws what a session's corner shows at rest, false for a
// status it has nothing for: the spinner while it works, the hourglass while
// it waits on you, and a finished turn nobody read as the soul the spinner's
// souls fuse into.
func statusCorner(gtx C, th *ui.Theme, sv *sessionView) (D, bool) {
	sinceEpoch := gtx.Now.Sub(time.Unix(0, 0))
	switch {
	case spins(sv.status):
		return soulsSpinner(gtx, th), true
	case sv.status == statusWaiting:
		return loop(gtx, th, hourglassSprite, sinceEpoch), true
	case sv.status == statusDone && sv.unread:
		return turnEnding(gtx, th, sv.turn), true
	}
	return D{}, false
}

// turnEnding is the corner of a finished turn: the spinner until it reaches
// soulsMeet, the fusion once, then the soul at rest. A turn that did not end
// on screen, read at startup, goes straight to rest.
func turnEnding(gtx C, th *ui.Theme, turn statusTurn) D {
	if !spins(turn.from) || turn.at.IsZero() {
		return loop(gtx, th, soulRest, gtx.Now.Sub(time.Unix(0, 0)))
	}
	since := gtx.Now.Sub(fusionStart(turn.at))
	switch {
	case since < 0:
		return soulsSpinner(gtx, th)
	case since < soulFusion.length():
		f, left := soulFusion.at(since)
		nextFrame(gtx, th, left)
		return drawSprite(gtx, th, soulFusion.frames[f])
	}
	return loop(gtx, th, soulRest, since-soulFusion.length())
}
