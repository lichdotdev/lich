package main

import (
	"image"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"
)

// A collapsed panel comes out once the pointer rests on its rail or its edge
// this long, so crossing on the way to something else brings nothing out,
// and goes back once the pointer has been away this long.
const (
	peekOpenDelay  = 150 * time.Millisecond
	peekCloseDelay = 300 * time.Millisecond
)

// panelPeek is whether a collapsed panel is out. The pointer
// over the trigger or the panel itself is one fact, over; since is when it
// last changed.
type panelPeek struct {
	shown bool
	over  bool
	since time.Time
	// held is a panel brought out by a click elsewhere, the footer's: it
	// stays out until the pointer has been in it.
	held bool
	// muted is a panel just collapsed under the pointer: it stays in until
	// the pointer has left, or collapsing would bring it straight back.
	muted bool
}

// point records where the pointer is at now.
func (p *panelPeek) point(now time.Time, over bool) {
	if over == p.over {
		return
	}
	switch {
	case !over:
		p.muted = false
	case p.muted && now.Sub(p.since) >= peekCloseDelay:
		// Away since the collapse for longer than a leave takes: the
		// pointer is coming back, not still there.
		p.muted = false
	}
	if over {
		p.held = false
	}
	p.over, p.since = over, now
}

// update decides whether the panel is out at now. staying keeps it out while
// something in it is in use: a menu, a field, a drag. It answers when the
// decision can next change, zero for not on its own.
func (p *panelPeek) update(now time.Time, staying bool) time.Time {
	if staying && !p.over {
		p.since = now
	}
	switch {
	case p.over && !p.shown && !p.muted:
		at := p.since.Add(peekOpenDelay)
		if now.Before(at) {
			return at
		}
		p.shown = true
	case !p.over && p.shown && !p.held && !staying:
		at := p.since.Add(peekCloseDelay)
		if now.Before(at) {
			return at
		}
		p.shown = false
	}
	return time.Time{}
}

// openNow brings the panel out at now without waiting for the pointer to
// rest. A pointer already gone on past the trigger gets the full delay to
// reach the panel.
func (p *panelPeek) openNow(now time.Time) {
	if p.muted || p.shown {
		return
	}
	p.shown = true
	if !p.over {
		p.since = now
	}
}

// hold brings the panel out until the pointer has been in it: at once if it
// is in it already.
func (p *panelPeek) hold() { p.shown, p.held, p.muted = true, !p.over, false }

// dismiss sends the panel back at once, and keeps it in until the pointer
// leaves: what was clicked in it is done.
func (p *panelPeek) dismiss() { p.shown, p.held, p.muted = false, false, p.over }

// hoverArea is an area that reads whether the pointer is over it, and only
// that: it takes no motion, so a move inside it draws no frame.
type hoverArea struct{ over bool }

func (h *hoverArea) read(gtx C) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: h, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel})
		if !ok {
			return
		}
		if e, ok := ev.(pointer.Event); ok {
			h.over = e.Kind == pointer.Enter
		}
	}
}

// watch lays the area over r, letting every event through to what is under
// it.
func (h *hoverArea) watch(gtx C, r image.Rectangle) {
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect(r).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, h)
}
