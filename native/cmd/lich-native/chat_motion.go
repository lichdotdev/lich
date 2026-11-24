package main

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/ui"
)

// The chat's own motions, past its rows coming and going (native/DESIGN.md ›
// The chat view): a row that becomes another plays the change in its slot.
const (
	// The live line turning into the closing line: the flame, verb and clock
	// go by chatSwapOut, the avatar, word and count come in up to chatSwapIn,
	// the hairline runs out to the edge up to chatSwapRule, and the slot takes
	// the closing line's height over chatSwapHeight.
	chatSwapOut    = 120 * time.Millisecond
	chatSwapIn     = 280 * time.Millisecond
	chatSwapRule   = 360 * time.Millisecond
	chatSwapHeight = 160 * time.Millisecond
	// chatToEndRun is the list's run back to its end when a message is sent
	// from above it.
	chatToEndRun = 220 * time.Millisecond
	// chatViewFade is how long a session's chat or terminal takes to come in
	// from the background after its view changed.
	chatViewFade = 150 * time.Millisecond
)

// chatPhase is how far a motion played from `from` to `to` after start has
// come at gtx.Now, on curve: 0 before it, 1 after it, asking for the next
// frame until then.
func chatPhase(gtx C, start time.Time, from, to time.Duration, curve func(float32) float32) float32 {
	e := gtx.Now.Sub(start)
	if e >= to {
		return 1
	}
	gtx.Execute(op.InvalidateCmd{})
	if e <= from {
		return 0
	}
	return curve(float32(e-from) / float32(to-from))
}

// chatSwap is the live line turning into the closing line key in its slot,
// from at; live is the line as it last stood.
type chatSwap struct {
	key  string
	at   time.Time
	live chatRow
}

// playing is whether the swap plays at now.
func (s chatSwap) playing(now time.Time) bool { return s.key != "" && now.Sub(s.at) < chatSwapRule }

// chatSwapLine is the closing line r while the live line turns into it.
func (u *window) chatSwapLine(gtx C, th *ui.Theme, p *chatPane, sv sessionView, r chatRow) D {
	s := p.swap
	out := 1 - chatPhase(gtx, s.at, 0, chatSwapOut, paneEaseIn)
	in := chatPhase(gtx, s.at, chatSwapOut, chatSwapIn, paneEaseOut)
	rule := chatPhase(gtx, s.at, chatSwapOut, chatSwapRule, paneEaseOut)
	height := chatPhase(gtx, s.at, 0, chatSwapHeight, paneEaseOut)
	return chatCrossfade(gtx, layout.W, height, out, 1,
		func(gtx C) D { return chatLive(gtx, th, sv.ID, s.live) },
		func(gtx C) D { return u.chatDone(gtx, th, p, sv, r, in, rule) })
}

// chatCrossfade draws out leaving and in arriving in one slot, each placed in
// it by at: out at opacity outA and taking no input, in at inA, and the slot's
// height run from out's to in's by height. Out is no longer drawn once it is
// gone and its height no longer counts.
func chatCrossfade(gtx C, at layout.Direction, height, outA, inA float32, out, in layout.Widget) D {
	gtx.Constraints.Min.Y = 0
	m := op.Record(gtx.Ops)
	di := in(gtx)
	ci := m.Stop()
	var do D
	var co op.CallOp
	if outA > 0 || height < 1 {
		m = op.Record(gtx.Ops)
		do = out(gtx.Disabled())
		co = m.Stop()
	}
	h := di.Size.Y
	if height < 1 {
		h = do.Size.Y + int(float32(di.Size.Y-do.Size.Y)*height+0.5)
	}
	size := image.Pt(max(do.Size.X, di.Size.X), h)
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	for _, l := range []struct {
		d    D
		call op.CallOp
		a    float32
	}{{do, co, outA}, {di, ci, inA}} {
		if l.a <= 0 {
			continue
		}
		off := op.Offset(at.Position(l.d.Size, size)).Push(gtx.Ops)
		fade := paint.PushOpacity(gtx.Ops, l.a)
		l.call.Add(gtx.Ops)
		fade.Pop()
		off.Pop()
	}
	return D{Size: size}
}

// chatToEnd is the list's run back to its end after a message was sent from
// above it, from at; done is how much of the run the list has made.
type chatToEnd struct {
	at   time.Time
	done float32
}

// toEnd starts the list's run back to its end at now, when it is scrolled
// up from an end it would otherwise follow.
func (v *chatView) toEnd(now time.Time) {
	if v.list.ScrollToEnd && v.list.Position.BeforeEnd {
		v.run = chatToEnd{at: now}
	}
}

// runToEnd moves the list of n rows its frame's share of the run to its end,
// and pins it there once the run is over. What is left is measured each frame
// and not once, since the list only estimates the rows it has not laid out.
func (v *chatView) runToEnd(gtx C, n int) {
	r := &v.run
	if r.at.IsZero() || n == 0 {
		return
	}
	pos := &v.list.Position
	e := chatPhase(gtx, r.at, 0, chatToEndRun, paneEaseOut)
	if e == 1 {
		pos.BeforeEnd, *r = false, chatToEnd{}
		return
	}
	left := float32(max(n-pos.First-pos.Count, 0))*float32(pos.Length)/float32(n) + float32(max(-pos.OffsetLast, 0))
	pos.Offset += int(left*(e-r.done)/(1-r.done) + 0.5)
	r.done = e
}

// chatShown is the view a session was last seen in: changed when it is not
// the one it was first seen in, and then from when it was first drawn.
type chatShown struct {
	view    string
	changed bool
	at      time.Time
}

// look takes the view session s is in, each frame that shows it. A chat
// seen again starts its rows afresh, so what came while it was a terminal
// plays no entries.
func (c *chatScreens) look(s sessionView) {
	view := chatViewOf(s)
	seen, ok := c.views[s.ID]
	if ok && seen.view == view {
		return
	}
	if c.views == nil {
		c.views = map[string]chatShown{}
	}
	c.views[s.ID] = chatShown{view: view, changed: ok}
	if ok && view == api.ViewChat {
		c.pane(s.ID).reopen()
	}
}

// viewFade is the opacity session id's chat or terminal is drawn at: rising
// from nothing over chatViewFade from the first frame that draws it after its
// view changed, whole otherwise. A terminal still starting draws nothing, so
// its fade waits for it.
func (c *chatScreens) viewFade(gtx C, id string) float32 {
	seen := c.views[id]
	if !seen.changed {
		return 1
	}
	if seen.at.IsZero() {
		seen.at = gtx.Now
		c.views[id] = seen
	}
	return chatPhase(gtx, seen.at, 0, chatViewFade, paneEaseOut)
}

// reopen forgets the rows the view brought in, and the calls of each fold, so
// the conversation drawn again lands as it is.
func (v *chatView) reopen() {
	v.rows = ui.Presence{}
	for _, f := range v.folds {
		f.enter = ui.Presence{}
	}
}
