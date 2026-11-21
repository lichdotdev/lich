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

// glassEnter is a session that stops to wait on you: the spinner's three
// souls, met in its center, become the hourglass. It plays once, from the
// spinner's soulsMeet frame, into the hourglass's first frame.
var glassEnter = statusSprite{
	frames: []string{
		"........... | ........... | ........... | ........... | ........... | .....4..... | ........... | ........... | ........... | ........... | ...........",
		"........... | ........... | ........... | ........... | ....444.... | .....3..... | ....414.... | ........... | ........... | ........... | ...........",
		"........... | ........... | ........... | ...44444... | ....543.... | .....3..... | ....513.... | ...41114... | ........... | ........... | ...........",
		"........... | ........... | ..4444444.. | ...54443... | ....543.... | .....3..... | ....513.... | ...51113... | ..4111114.. | ........... | ...........",
		"........... | ..4444444.. | ..5444443.. | ...54443... | ....543.... | .....3..... | ....513.... | ...51113... | ..5111113.. | ..4111114.. | ...........",
	},
	steps: []time.Duration{80 * time.Millisecond, 60 * time.Millisecond, 60 * time.Millisecond, 60 * time.Millisecond, 60 * time.Millisecond},
}

// checkFusion is a turn ending: the ember the souls meet in turns green and
// stretches into the check, white at its ends, then shines. It plays once,
// from the spinner's soulsMeet frame.
var checkFusion = statusSprite{
	frames: []string{
		"........... | ........... | ........... | ........... | ........... | ....676.... | ....777.... | ....676.... | ........... | ........... | ...........",
		"........... | ........... | ........... | ........... | ........... | ........... | .....44.... | .....4..... | ........... | ........... | ...........",
		"........... | ........... | ........... | ........... | .......4... | ......84... | .....87.... | ...487..... | ...44...... | ........... | ...........",
		"........... | ........... | ........... | ........44. | .......87.. | ......87... | .44..87.... | ..7887..... | ...77...... | ........... | ...........",
		"........... | ........... | .........8. | ........87. | .......87.. | .8....87... | .78..87.... | ..7887..... | ...77...... | ........... | ...........",
		"........... | ........... | .........4. | ........44. | .......44.. | .4....44... | .44..44.... | ..4444..... | ...44...... | ........... | ...........",
	},
	steps: []time.Duration{80 * time.Millisecond, 60 * time.Millisecond, 60 * time.Millisecond, 60 * time.Millisecond, 60 * time.Millisecond, 90 * time.Millisecond},
}

// checkDone is a finished turn nobody read, at rest: the check, still, so a
// finished session asks the sidebar for no frame.
const checkDone = "........... | ........... | .........8. | ........87. | .......87.. | .8....87... | .78..87.... | ..7887..... | ...77...... | ........... | ..........."

// soulsMeet is the spinner's frame with the three souls together in the
// center, where the fusion starts so the corner never jumps.
const soulsMeet = 10

// fusionStart is when a turn that ended at end starts the motion out of the
// spinner: the next time the spinner reaches soulsMeet.
func fusionStart(end time.Time) time.Time {
	tick := end.UnixNano() / int64(soulsStep)
	n := int64(len(soulsFrames))
	wait := ((soulsMeet-tick)%n + n) % n
	return time.Unix(0, (tick+wait)*int64(soulsStep))
}

// spriteInk is a status symbol's color: 4 Foreground, 5 halfway from
// MutedForeground to it, 2 the chat flame's body, 3 MutedForeground, 1 Muted;
// the check's 7 TonePass, 8 its lit side, toward the lighter of the theme's
// ink and ground, and 6 its dim side, toward the surface under it.
func spriteInk(th *ui.Theme, symbol byte) (color.NRGBA, bool) {
	switch symbol {
	case '5':
		return ui.MixOKLab(th.MutedForeground, th.Foreground, 0.5), true
	case '1':
		return th.Muted, true
	case '7':
		return th.TonePass, true
	case '8':
		return ui.MixOKLab(th.TonePass, lighter(th.Foreground, th.Background), checkLit), true
	case '6':
		return ui.MixOKLab(th.TonePass, th.Surface, checkDim), true
	}
	return chatFlameColor(th, symbol)
}

// How far the check's lit and dim sides go from TonePass.
const (
	checkLit = 0.4
	checkDim = 0.5
)

// lighter is whichever of a and b is lighter, the side light comes from in
// a dark theme and in a light one alike.
func lighter(a, b color.NRGBA) color.NRGBA {
	if int(a.R)+int(a.G)+int(a.B) >= int(b.R)+int(b.G)+int(b.B) {
		return a
	}
	return b
}

// drawSprite paints rows, an 11x11 frame, centered in gtx's minimum size at
// soulsSize. Each cell's edges land on whole device pixels, so the sprite
// never blurs; at a size 11 does not divide, some cells are a pixel wider.
func drawSprite(gtx C, th *ui.Theme, rows string) D {
	side := max(soulsCells, gtx.Dp(soulsSize))
	edge := func(i int) int { return (i*side + soulsCells/2) / soulsCells }
	box := gtx.Constraints.Min
	defer op.Offset(image.Pt((box.X-side)/2, (box.Y-side)/2)).Push(gtx.Ops).Pop()
	for y, row := range strings.Split(rows, " | ") {
		for x := 0; x < len(row); x++ {
			if c, ok := spriteInk(th, row[x]); ok {
				paint.FillShape(gtx.Ops, c, clip.Rect(image.Rect(edge(x), edge(y), edge(x+1), edge(y+1))).Op())
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
// it waits on you, and the check for a finished turn nobody read. A status
// the session reached on screen, from working, plays out of the spinner when
// its souls meet in the center; one read at startup is drawn at once.
func statusCorner(gtx C, th *ui.Theme, sv *sessionView) (D, bool) {
	switch {
	case spins(sv.status):
		return soulsSpinner(gtx, th), true
	case sv.status == statusWaiting:
		return waitingCorner(gtx, th, sv.turn), true
	case sv.status == statusDone && sv.unread:
		return doneCorner(gtx, th, sv.turn), true
	}
	return D{}, false
}

// sinceSpinner is how far past the motion out of the spinner gtx.Now is for
// turn, false for a status that did not come from working on screen.
func sinceSpinner(gtx C, turn statusTurn) (time.Duration, bool) {
	if !spins(turn.from) || turn.at.IsZero() {
		return 0, false
	}
	return gtx.Now.Sub(fusionStart(turn.at)), true
}

// waitingCorner is the spinner until its souls meet, the souls turning into
// the hourglass once, then the hourglass looped.
func waitingCorner(gtx C, th *ui.Theme, turn statusTurn) D {
	since, played := sinceSpinner(gtx, turn)
	switch {
	case !played:
		return loop(gtx, th, hourglassSprite, gtx.Now.Sub(time.Unix(0, 0)))
	case since < 0:
		return soulsSpinner(gtx, th)
	case since < glassEnter.length():
		return once(gtx, th, glassEnter, since)
	}
	return loop(gtx, th, hourglassSprite, since-glassEnter.length())
}

// doneCorner is the spinner until its souls meet, the check forming once,
// then the check at rest, which asks for no frame.
func doneCorner(gtx C, th *ui.Theme, turn statusTurn) D {
	since, played := sinceSpinner(gtx, turn)
	switch {
	case played && since < 0:
		return soulsSpinner(gtx, th)
	case played && since < checkFusion.length():
		return once(gtx, th, checkFusion, since)
	}
	return drawSprite(gtx, th, checkDone)
}

// once draws s since into its single play and asks for its next frame, or
// for the frame that ends it.
func once(gtx C, th *ui.Theme, s statusSprite, since time.Duration) D {
	f, left := s.at(since)
	nextFrame(gtx, th, left)
	return drawSprite(gtx, th, s.frames[f])
}
