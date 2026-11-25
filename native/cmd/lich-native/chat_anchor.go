package main

import (
	"slices"
	"time"

	"gioui.org/unit"
)

// chatSendPeek is how much of what came before stays in view over a message
// that starts a turn, so the person does not lose where they were.
const chatSendPeek = unit.Dp(100)

// chatAnchor holds the message that started a turn near the top of the view,
// from at, with scroll room past the last row for the answer to fill; key is
// the message's row.
type chatAnchor struct {
	at  time.Time
	key string
}

// sent runs the list back to its end for a message sent from the box, and
// holds one that starts a turn near the top of the view: the message only
// held, not run as a command or in the shell.
func (u *window) sent(gtx C, p *chatPane, id string, working bool) {
	p.toEnd(gtx.Now)
	if key := u.model.chat.heldID(id); !working && key != "" {
		p.anchor(gtx.Now, key)
	}
}

// anchor holds the message just sent near the top of the view. A
// conversation that still fits the view rests on the composer instead, its
// turn growing it from the foot as any other row does.
func (v *chatView) anchor(now time.Time, key string) {
	if v.list.ScrollToEnd {
		v.anchored = chatAnchor{at: now, key: key}
	}
}

// room is the scroll room past shown's last row that keeps the anchored
// message chatSendPeek under the top of a view h high, the list pinned to its
// end, eased in over chatToEndRun. The rows under the message take it as they
// grow, and the anchor is given up once they fill it: from there the list
// follows its end, and a message taller than the view shows from its end. The
// message comes into the rows the frame after it was sent, and a message
// taken back gives the anchor up.
func (v *chatView) room(gtx C, shown []string, h int) int {
	a := &v.anchored
	if a.at.IsZero() {
		return 0
	}
	i := slices.Index(shown, a.key)
	if _, drawn := v.seen[a.key]; i < 0 && !drawn {
		return 0
	}
	room := h - gtx.Dp(chatSendPeek)
	for _, k := range shown[max(i, 0):] {
		room -= v.heights[k]
	}
	if i < 0 || room <= 0 {
		*a = chatAnchor{}
		return 0
	}
	return int(float32(room)*chatPhase(gtx, a.at, 0, chatToEndRun, paneEaseOut) + 0.5)
}
