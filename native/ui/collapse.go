package ui

import (
	"image"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// collapseDuration is SessionGroup's `transition-[grid-template-rows,opacity]
// duration-200 ease-out`.
const collapseDuration = 200 * time.Millisecond

// Collapse is SessionGroup's fold: the `grid grid-rows-[0fr]↔[1fr]` box that
// clips its child to a height animating between nothing and all of it,
// fading with it. Folded, and while folding shut, the child is inert: it
// draws but takes no input, and offers nothing to focus.
type Collapse struct{ fold tween }

// Layout lays w out open or folded. The first call lands there without
// animating: a block drawn folded from the start is not a change.
func (c *Collapse) Layout(gtx C, open bool, w layout.Widget) D {
	target := float32(0)
	if open {
		target = 1
	}
	p := c.fold.at(gtx, target, collapseDuration, easeOut)
	if !open {
		gtx = gtx.Disabled()
	}
	return reveal(gtx, p, 1, p, w)
}

// Presence timings: 160ms ease-out in, 180ms ease-in out, from a 0.6 scale
// pose.
const (
	presenceEnter = 160 * time.Millisecond
	// PresenceExit is how long an item that left the list stays drawn, playing
	// its exit.
	PresenceExit  = 180 * time.Millisecond
	presenceScale = 0.6
)

// Presence animates a keyed list: an item that arrives after the list first
// drew grows in (height from 0, scale from .6 at the top edge, opacity from
// 0), and one that leaves is kept in the list, in its slot, while it plays the
// same motion backwards.
type Presence struct {
	items   map[string]*presenceItem
	drawn   []string
	settled bool
}

type presenceItem struct {
	motion  tween
	leaving bool
	leftAt  time.Time
	slot    int
}

// Update takes the live keys at now and returns the keys to draw: live, with
// every key that left less than PresenceExit ago put back where it was. Call
// it once a frame, before Layout, with gtx.Now.
func (p *Presence) Update(now time.Time, live []string) []string {
	if p.items == nil {
		p.items = map[string]*presenceItem{}
	}
	for _, k := range live {
		it, ok := p.items[k]
		if !ok {
			it = &presenceItem{}
			if p.settled {
				it.motion.seed(0)
			}
			p.items[k] = it
		}
		it.leaving = false
	}
	for i, k := range p.drawn {
		if it := p.items[k]; !it.leaving && !slices.Contains(live, k) {
			it.leaving, it.leftAt, it.slot = true, now, i
		}
	}
	for k, it := range p.items {
		if it.leaving && now.Sub(it.leftAt) >= PresenceExit {
			delete(p.items, k)
		}
	}
	p.settled = true
	p.drawn = p.withLeaving(live)
	return p.drawn
}

// withLeaving is live with each leaving key spliced back at the slot it left.
func (p *Presence) withLeaving(live []string) []string {
	var leaving []string
	for _, k := range p.drawn {
		if it, ok := p.items[k]; ok && it.leaving {
			leaving = append(leaving, k)
		}
	}
	slices.SortStableFunc(leaving, func(a, b string) int { return p.items[a].slot - p.items[b].slot })
	shown := slices.Clone(live)
	for _, k := range leaving {
		shown = slices.Insert(shown, min(p.items[k].slot, len(shown)), k)
	}
	return shown
}

// Leaving reports whether key is drawn only for its exit, so the caller
// keeps it out of anything it could be picked up by.
func (p *Presence) Leaving(key string) bool {
	it, ok := p.items[key]
	return ok && it.leaving
}

// Layout draws key's item w in its motion; a leaving item takes no input.
func (p *Presence) Layout(gtx C, key string, w layout.Widget) D {
	it, ok := p.items[key]
	if !ok {
		return w(gtx)
	}
	target, duration, curve := float32(1), presenceEnter, easeOut
	if it.leaving {
		target, duration, curve = 0, PresenceExit, easeIn
	}
	v := it.motion.at(gtx, target, duration, curve)
	if it.leaving {
		gtx = gtx.Disabled()
	}
	return reveal(gtx, v, presenceScale+(1-presenceScale)*v, v, w)
}

// reveal draws w clipped to rows of its height (a 0fr..1fr grid track),
// scaled by scale from its top edge's middle (origin-top) and at opacity.
// It takes rows of the height in the layout; the scale takes none.
func reveal(gtx C, rows, scale, opacity float32, w layout.Widget) D {
	gtx.Constraints.Min.Y = 0
	m := op.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	h := int(float32(d.Size.Y)*rows + 0.5)
	if rows == 1 {
		call.Add(gtx.Ops)
		return d
	}
	if h == 0 {
		return D{Size: image.Pt(d.Size.X, 0)}
	}
	origin := f32.Pt(float32(d.Size.X)/2, 0)
	defer op.Affine(f32.Affine2D{}.Scale(origin, f32.Pt(scale, scale))).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: image.Pt(d.Size.X, h)}.Push(gtx.Ops).Pop()
	defer paint.PushOpacity(gtx.Ops, opacity).Pop()
	call.Add(gtx.Ops)
	return D{Size: image.Pt(d.Size.X, h)}
}
