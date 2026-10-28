package main

import (
	"image"
	"image/color"
	"math"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The agent's pointer over its page: lich's own arrow, in the session's card
// color or the theme's primary, outlined in primary-foreground so it reads
// over any page, with a badge naming the provider. It glides to each click,
// dips, rings, and fades once the agent goes quiet. Timings from the approved
// mockup (2026-10-26).
const (
	cursorGlide = 280 * time.Millisecond
	cursorDip   = 170 * time.Millisecond
	cursorRing  = 520 * time.Millisecond
	cursorFade  = 300 * time.Millisecond
	// cursorScroll is how long a scroll's chevrons take to slide past.
	cursorScroll = 560 * time.Millisecond
	// cursorDipScale is how far the arrow shrinks at the bottom of its dip.
	cursorDipScale = 0.14
	// cursorCatchUp is the share of its time an action plays in while more
	// wait behind it: an agent filling a form fires a field every half second,
	// and played whole the cursor fell seconds behind the page, pointing at
	// where fields had been before the page scrolled.
	cursorCatchUp = 0.4
)

// cursorShadow is the drop shadow under the arrow, the mockup's 35% black.
var cursorShadow = color.NRGBA{A: 90}

// cursorEase is the glide's curve, CSS's cubic-bezier(.2,.8,.2,1).
var cursorEase = ui.CubicBezier(0.2, 0.8, 0.2, 1)

// agentCursor is where the agent's pointer is, in pixels of its pane: over
// the page for what the agent does in it, over the toolbar for back, forward,
// reload and an address.
type agentCursor struct {
	shown    bool
	from, to f32.Point
	// action is the one playing; pace is the share of its time it plays in,
	// cursorCatchUp while more actions wait behind it.
	action string
	pace   float64
	// scroll is the direction of the scroll the cursor glided for, 1 down
	// and -1 up; 0 for a click, which dips and rings instead.
	scroll int
	// moved is when the glide to to began; last is the agent's last action,
	// which the fade counts from.
	moved, last time.Time
	// seen is the last mark taken; pending are the actions at a point still
	// to play, each played whole before the next.
	seen    uint64
	pending []browserMark
	// busy is whether the agent's turn ran at the last frame.
	busy bool
	// rings are the rings of clicks the cursor already moved on from, left
	// to fade where they were: the cursor goes on once a click has dipped,
	// rather than waiting out its ring.
	rings []cursorRingAt
}

// cursorRingAt is one click's ring: where, from when, for how long.
type cursorRingAt struct {
	at    f32.Point
	start time.Time
	span  time.Duration
}

// cursorPending is how many actions the cursor keeps waiting to play: past it
// the oldest go, so a fast agent leaves the cursor a beat behind, not a replay.
const cursorPending = 3

// toolbarActions are the agent's actions the cursor plays on the pane's
// toolbar rather than at a point of the page.
var toolbarActions = map[string]bool{"back": true, "forward": true, "reload": true, "navigate": true}

// take queues the marks not seen yet: an action at a point of the page (a
// click, typing into or pressing on an element, a scroll), on the toolbar or
// on the page's dialog is played by advance, any other only keeps the cursor up.
func (c *agentCursor) take(marks []browserMark) {
	for _, m := range marks {
		if m.seq <= c.seen {
			continue
		}
		c.seen = m.seq
		if m.at.After(c.last) {
			c.last = m.at
		}
		if m.x != nil && m.y != nil || toolbarActions[m.action] || dialogActions[m.action] {
			c.pending = append(c.pending, m)
		}
	}
	c.pending = trimPending(c.pending)
}

// trimPending keeps the latest cursorPending marks, and every answer to the
// page's dialog older than those: the card stays on the pane until the
// cursor has pressed its button.
func trimPending(pending []browserMark) []browserMark {
	cut := max(0, len(pending)-cursorPending)
	var kept []browserMark
	for _, m := range pending[:cut] {
		if dialogActions[m.action] {
			kept = append(kept, m)
		}
	}
	return append(kept, pending[cut:]...)
}

// advance starts the next queued action once the one playing has finished,
// at the point place puts it in the pane, and returns it; false when none
// started.
func (c *agentCursor) advance(now time.Time, place func(browserMark) f32.Point) (browserMark, bool) {
	if len(c.pending) == 0 || c.shown && now.Before(c.played()) {
		return browserMark{}, false
	}
	m := c.pending[0]
	c.pending = c.pending[1:]
	if c.shown && c.scroll == 0 {
		c.rings = append(c.rings, cursorRingAt{at: c.to, start: c.landing(), span: c.span(cursorRing)})
	}
	c.rings = slices.DeleteFunc(c.rings, func(r cursorRingAt) bool { return !now.Before(r.start.Add(r.span)) })
	to := place(m)
	from := to
	if c.shown {
		from = c.to
	}
	c.from, c.to, c.moved, c.shown, c.action = from, to, now, true, m.action
	c.pace = 1
	if len(c.pending) > 0 {
		c.pace = cursorCatchUp
	}
	c.scroll = 0
	if m.action == "scroll" && m.dy != 0 {
		c.scroll = 1
		if m.dy < 0 {
			c.scroll = -1
		}
	}
	return m, true
}

func (c *agentCursor) position(now time.Time) f32.Point {
	t := cursorEase(progress(now.Sub(c.moved), c.span(cursorGlide)))
	return c.from.Add(c.to.Sub(c.from).Mul(t))
}

// observe notes whether the agent's turn runs at now: its end counts as the
// agent's last activity, which the hold counts from.
func (c *agentCursor) observe(now time.Time, busy bool) {
	if c.busy && !busy && now.After(c.last) {
		c.last = now
	}
	c.busy = busy
}

// alpha is the cursor's opacity at now: whole while the agent's turn runs and
// for agentHold after its last activity, then fading out over cursorFade.
func (c *agentCursor) alpha(now time.Time) float32 {
	switch {
	case !c.shown:
		return 0
	case c.busy:
		return 1
	}
	return 1 - progress(now.Sub(c.last)-agentHold, cursorFade)
}

// landed is how long ago the glide reached the point, negative while it
// still moves.
func (c *agentCursor) landed(now time.Time) time.Duration {
	return now.Sub(c.moved) - c.span(cursorGlide)
}

// span is d at the pace the action playing goes at.
func (c *agentCursor) span(d time.Duration) time.Duration {
	if c.pace <= 0 {
		return d
	}
	return time.Duration(float64(d) * c.pace)
}

// landing is when the glide of the action playing reaches its point.
func (c *agentCursor) landing() time.Time {
	return c.moved.Add(c.span(cursorGlide))
}

// played is when the cursor may go on to the next action: a click once it
// has dipped, its ring fading on behind it; a scroll once its chevrons, which
// ride the cursor, have slid past.
func (c *agentCursor) played() time.Time {
	if c.scroll != 0 {
		return c.landing().Add(c.span(cursorScroll))
	}
	return c.landing().Add(c.span(cursorDip))
}

// settled is when nothing about the cursor moves any more until it fades.
func (c *agentCursor) settled() time.Time {
	end := c.played()
	if c.scroll == 0 {
		end = c.landing().Add(c.span(cursorRing))
	}
	for _, r := range c.rings {
		if r.start.Add(r.span).After(end) {
			end = r.start.Add(r.span)
		}
	}
	return end
}

func progress(d, span time.Duration) float32 {
	return float32(min(max(float64(d)/float64(span), 0), 1))
}

// cursorArrow is lich's arrow in a 24-unit box, its tip at cursorTip: rounder
// and heavier than the system's, so it is not taken for the person's own.
var cursorArrow = []f32.Point{
	f32.Pt(4.2, 2.6), f32.Pt(19.4, 10.4), f32.Pt(20.6, 11), f32.Pt(19.3, 11.6),
	f32.Pt(13.2, 13.6), f32.Pt(10.6, 19.6), f32.Pt(10, 20.9), f32.Pt(9.4, 19.6),
}

var cursorTip = f32.Pt(4, 3)

func arrowPath(ops *op.Ops, unit float32) clip.PathSpec {
	p := func(i int) f32.Point { return cursorArrow[i].Sub(cursorTip).Mul(unit) }
	var path clip.Path
	path.Begin(ops)
	path.MoveTo(p(0))
	path.LineTo(p(1))
	path.QuadTo(p(2), p(3))
	path.LineTo(p(4))
	path.LineTo(p(5))
	path.QuadTo(p(6), p(7))
	path.Close()
	return path.End()
}

// drawAgentCursor draws c over its pane, in tint with outline around it, the
// badge naming provider with its icon. It asks for the frames its motion
// needs.
func (u *window) drawAgentCursor(gtx C, c *agentCursor, tint, outline color.NRGBA, provider string, icon *icons.Icon) {
	now := gtx.Now
	a := c.alpha(now)
	if a <= 0 {
		c.shown = false
		return
	}
	at := c.position(now)
	landed := c.landed(now)
	defer paint.PushOpacity(gtx.Ops, a).Pop()
	if c.scroll != 0 {
		drawScrollChevrons(gtx, at, landed, c.span(cursorScroll), c.scroll, tint, outline)
	}
	for _, r := range c.rings {
		drawCursorRing(gtx, r.at, now.Sub(r.start), r.span, tint)
	}
	dipFor := c.span(cursorDip)
	if c.scroll == 0 {
		drawCursorRing(gtx, at, landed, c.span(cursorRing), tint)
	}
	s := float32(1)
	if c.scroll == 0 && landed >= 0 && landed < dipFor {
		s -= cursorDipScale * float32(math.Sin(math.Pi*float64(progress(landed, dipFor))))
	}
	unit := float32(gtx.Dp(1))
	place := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(s, s)).Offset(at)).Push(gtx.Ops)
	shadow := op.Offset(image.Pt(0, gtx.Dp(1))).Push(gtx.Ops)
	paint.FillShape(gtx.Ops, cursorShadow, clip.Outline{Path: arrowPath(gtx.Ops, unit)}.Op())
	shadow.Pop()
	paint.FillShape(gtx.Ops, tint, clip.Outline{Path: arrowPath(gtx.Ops, unit)}.Op())
	paint.FillShape(gtx.Ops, outline, clip.Stroke{Path: arrowPath(gtx.Ops, unit), Width: 1.6 * unit}.Op())
	place.Pop()
	badge := op.Offset(image.Pt(int(at.X)+gtx.Dp(12), int(at.Y)+gtx.Dp(15))).Push(gtx.Ops)
	u.cursorBadge(gtx, tint, outline, provider, icon)
	badge.Pop()
	// A busy cursor waits for the turn's end, whose status event redraws.
	switch {
	case now.Before(c.settled()) || len(c.pending) > 0 || !c.busy && c.last.Add(agentHold).Before(now):
		gtx.Execute(op.InvalidateCmd{})
	case !c.busy:
		gtx.Execute(op.InvalidateCmd{At: c.last.Add(agentHold)})
	}
}

// drawCursorRing draws a click's ring at at, since into its span.
func drawCursorRing(gtx C, at f32.Point, since, span time.Duration, tint color.NRGBA) {
	if since < 0 || since >= span {
		return
	}
	p := progress(since, span)
	r := float32(gtx.Dp(17)) * cursorEase(p)
	ring := clip.Ellipse{Min: image.Pt(int(at.X-r), int(at.Y-r)), Max: image.Pt(int(at.X+r), int(at.Y+r))}
	paint.FillShape(gtx.Ops, ui.Alpha(tint, 0.95*(1-p)), clip.Stroke{Path: ring.Path(gtx.Ops), Width: float32(gtx.Dp(2))}.Op())
}

// drawScrollChevrons slides two chevrons under the arrow at at in the scroll's
// direction, fading in and out, for span after the glide landed.
func drawScrollChevrons(gtx C, at f32.Point, landed, span time.Duration, dir int, tint, outline color.NRGBA) {
	if landed < 0 || landed >= span {
		return
	}
	p := progress(landed, span)
	// in until a third of the way, then out, as the mockup's keyframes
	alpha := min(p/0.35, (1-p)/0.65)
	unit := float32(gtx.Dp(1))
	d := float32(dir)
	shift := (-6 + 16*p) * d * unit
	center := at.Add(f32.Pt(0, 30*unit+shift))
	chevron := func(dy float32) clip.PathSpec {
		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(center.Add(f32.Pt(-6*unit, (dy-3)*d*unit)))
		path.LineTo(center.Add(f32.Pt(0, (dy+3)*d*unit)))
		path.LineTo(center.Add(f32.Pt(6*unit, (dy-3)*d*unit)))
		return path.End()
	}
	defer paint.PushOpacity(gtx.Ops, alpha).Pop()
	for _, dy := range []float32{-5, 5} {
		paint.FillShape(gtx.Ops, outline, clip.Stroke{Path: chevron(dy), Width: 4.4 * unit}.Op())
		paint.FillShape(gtx.Ops, tint, clip.Stroke{Path: chevron(dy), Width: 2.6 * unit}.Op())
	}
}

// cursorBadge is the pill beside the arrow: the provider's icon and name in
// ink on tint, outlined like the arrow.
func (u *window) cursorBadge(gtx C, tint, outline color.NRGBA, provider string, icon *icons.Icon) {
	th := u.theme.On(tint)
	fg := inkOn(tint, outline)
	gtx.Constraints.Min = image.Point{}
	rec := op.Record(gtx.Ops)
	d := layout.Inset{Left: ui.Space(1.25), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = gtx.Dp(20)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icon.Layout(gtx, ui.Space(3), fg) }),
			ui.Gap(1),
			layout.Rigid(th.Text(ui.TextXS, provider).In(fg).Weight(font.Medium).Layout),
		)
	})
	call := rec.Stop()
	pill := clip.UniformRRect(image.Rectangle{Max: d.Size}, d.Size.Y/2)
	paint.FillShape(gtx.Ops, tint, pill.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, outline, clip.Stroke{Path: pill.Path(gtx.Ops), Width: float32(gtx.Dp(1.5))}.Op())
	call.Add(gtx.Ops)
}

// inkOn is the text color on tint: the theme's primary-foreground, which is
// made for the theme's primary, unless tint is a card color it does not read
// on, where black or white is, by luminance.
func inkOn(tint, themed color.NRGBA) color.NRGBA {
	if contrast(tint, themed) >= 3 {
		return themed
	}
	if luminance(tint) > 0.4 {
		return color.NRGBA{A: 255}
	}
	return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
}

// contrast is WCAG's contrast ratio of two opaque colors.
func contrast(a, b color.NRGBA) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// luminance is WCAG's relative luminance of an opaque sRGB color.
func luminance(c color.NRGBA) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}
