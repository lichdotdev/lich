package ui

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// sonner 2.0.7's geometry (dist/index.mjs, dist/styles.css) and the toast
// offset.
const (
	toastWidth          unit.Dp = 356 // TOAST_WIDTH
	toastGap            unit.Dp = 14  // GAP
	toastBorder         unit.Dp = 1
	toastPadding        unit.Dp = 16
	toastSwipeThreshold unit.Dp = 45 // SWIPE_THRESHOLD
	// toastSwipeVelocity is the px/ms past which a short swipe still
	// dismisses.
	toastSwipeVelocity = 0.11
	// toastScaleStep is how much smaller each toast behind the front is.
	toastScaleStep = 0.05
	// toastBehindExitLift is translateY(40%), where a toast behind the front
	// sinks to as it leaves a collapsed stack.
	toastBehindExitLift = 0.4
	// toastSwipeDampen is getDampening's divisor: a pull the wrong way
	// moves 1/(1.5 + |delta|/20) of the pointer.
	toastSwipeDampen = 20
)

var (
	toastOffsetTop  = Space(12) // top 3rem, below the topbar
	toastOffsetSide = Space(4)  // right 1rem, and left by the same
	// toastOffsetBottom mirrors the top: clear of the footer bar as the top
	// one is of the topbar.
	toastOffsetBottom = Space(12)
)

// ToastPosition is sonner's position prop: the corner a Toaster stacks in.
// The zero value is top-right, sonner's own default.
type ToastPosition int

const (
	ToastTopRight ToastPosition = iota
	ToastTopLeft
	ToastBottomRight
	ToastBottomLeft
)

func (p ToastPosition) bottom() bool { return p == ToastBottomRight || p == ToastBottomLeft }
func (p ToastPosition) left() bool   { return p == ToastTopLeft || p == ToastBottomLeft }

// anchor is the top-left of the column the stack is laid out in, in a window
// of size max: a bottom stack's column starts at its bottom edge and grows up
// (placeOf).
func (p ToastPosition) anchor(gtx C, max image.Point) image.Point {
	at := image.Pt(max.X-gtx.Dp(toastOffsetSide)-gtx.Dp(toastWidth), gtx.Dp(toastOffsetTop))
	if p.left() {
		at.X = gtx.Dp(toastOffsetSide)
	}
	if p.bottom() {
		at.Y = max.Y - gtx.Dp(toastOffsetBottom)
	}
	return at
}

// placeOf is where a toast of height h sits y away from the stack's edge,
// scaled about its center: below the edge for a top stack, above it for a
// bottom one, so entering from y = -h comes in from outside the window on
// either side.
func (p ToastPosition) placeOf(center f32.Point, scale, y, h float32) f32.Affine2D {
	if p.bottom() {
		y = -y - h
	}
	return f32.Affine2D{}.Scale(center, f32.Pt(scale, scale)).Offset(f32.Pt(0, y))
}

const (
	toastTransition = 400 * time.Millisecond // transform, opacity, height .4s
	toastSwipeOut   = 200 * time.Millisecond // swipe-out-* animation
	// A toast leaving from behind a collapsed stack: transform .5s,
	// opacity .2s.
	toastBehindExit = 500 * time.Millisecond
	toastBehindFade = 200 * time.Millisecond
)

// cssEaseOut is CSS's "ease-out" keyword, the swipe-out animation's timing;
// motion.go's easeOut is Tailwind's.
var cssEaseOut = CubicBezier(0, 0, 0.58, 1)

// toastView is a toast's drawing state between frames.
type toastView struct {
	seeded                             bool
	y, scale, opacity, height, content tween
	swipeX, swipeY                     tween
	// offset is the expanded stack's distance from the top, frozen once
	// the toast is removed (sonner's offsetBeforeRemove).
	offset                          float32
	action, cancel, secondary       widget.Clickable
	actionAt, cancelAt, secondaryAt image.Rectangle // body-local
	drag                            toastDrag
	swipedOut                       bool
	outDir, outFrom                 f32.Point
	// What place worked out for this frame.
	body          op.CallOp
	bodySize      image.Point
	bodyAt        image.Point // the body's top-left in the box
	boxSize       image.Point
	place         f32.Affine2D
	swipe         f32.Point
	alpha, inside float32 // the toast's opacity and its content's
	canHit        bool
}

// toastDrag is a swipe in progress, its amounts in pixels.
type toastDrag struct {
	active bool
	start  f32.Point
	at     time.Time
	axis   byte // 0 until the pointer moves, then 'x' or 'y'
	amount f32.Point
}

// ToasterStyle draws a Toaster in the corner Position names, above
// everything.
type ToasterStyle struct {
	State    *Toaster
	Position ToastPosition
	// Light is the scheme sonner's theme prop follows: the description and
	// cancel colors are sonner's own, not the popover tokens.
	Light bool
	// Expand is sonner's expand prop: the stack drawn expanded without a
	// hover.
	Expand bool
	theme  *Theme
}

// Toaster is sonner's Toaster over state.
func (th *Theme) Toaster(state *Toaster, light bool) ToasterStyle {
	return ToasterStyle{State: state, Light: light, theme: th}
}

// Layout draws the stack deferred above the rest of the frame. Call it at
// the window's origin with the window's size as the maximum constraint.
func (s ToasterStyle) Layout(gtx C) D {
	t := s.State
	t.mu.Lock()
	clicked := s.frame(gtx)
	t.mu.Unlock()
	for _, f := range clicked {
		f()
	}
	return D{}
}

func (s ToasterStyle) frame(gtx C) []func() {
	t := s.State
	t.readHover(gtx)
	clicked := s.readEntries(gtx)
	t.stack.pause(t.hovered || t.pressed || t.hidden, gtx.Now)
	if wake := t.stack.advance(gtx.Now); !wake.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: wake})
	}
	if len(t.stack.entries) == 0 {
		t.hovered, t.pressed = false, false
		return clicked
	}
	s.place(gtx)
	macro := op.Record(gtx.Ops)
	off := op.Offset(s.Position.anchor(gtx, gtx.Constraints.Max)).Push(gtx.Ops)
	entries := t.stack.entries
	for i := len(entries) - 1; i >= 0; i-- {
		s.paint(gtx, entries[i])
	}
	s.areas(gtx)
	off.Pop()
	op.Defer(gtx.Ops, macro.Stop())
	return clicked
}

// readHover follows the pointer over the stack: hovered expands it, and
// either holds the timers.
func (t *Toaster) readHover(gtx C) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: t, Kinds: pointer.Enter | pointer.Leave | pointer.Move | pointer.Press | pointer.Release | pointer.Cancel})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Enter, pointer.Move:
			t.hovered = true
		case pointer.Leave:
			// A swipe dragged out of the stack keeps it open.
			t.hovered = t.pressed
		case pointer.Press:
			t.pressed = true
		case pointer.Release:
			t.pressed = false
		case pointer.Cancel:
			t.hovered, t.pressed = false, false
		}
	}
}

// readEntries takes each toast's swipes and button clicks, and returns the
// OnClick handlers to run once the lock is released.
func (s ToasterStyle) readEntries(gtx C) []func() {
	var clicked []func()
	stack := &s.State.stack
	for _, e := range stack.entries {
		s.readDrag(gtx, e)
		for _, b := range []struct {
			click  *widget.Clickable
			button *ToastButton
		}{{&e.view.action, e.Action}, {&e.view.cancel, e.Cancel}, {&e.view.secondary, e.Secondary}} {
			if b.click.Clicked(gtx) && b.button != nil && !e.removed() {
				stack.dismiss(e.ID, gtx.Now)
				if b.button.OnClick != nil {
					clicked = append(clicked, b.button.OnClick)
				}
			}
		}
	}
	return clicked
}

func (s ToasterStyle) readDrag(gtx C, e *toastEntry) {
	d := &e.view.drag
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: e, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			return
		}
		p, ok := ev.(pointer.Event)
		if !ok || e.removed() {
			continue
		}
		switch p.Kind {
		case pointer.Press:
			onButton := e.view.onButton(p.Position)
			if p.Buttons == pointer.ButtonPrimary && !onButton {
				*d = toastDrag{active: true, start: p.Position, at: gtx.Now}
			}
		case pointer.Drag:
			if d.active {
				d.follow(gtx, p.Position.Sub(d.start), s.Position)
			}
		case pointer.Release:
			if d.active {
				s.release(gtx, e)
			}
		case pointer.Cancel:
			if d.active {
				d.active, d.amount = false, f32.Point{}
			}
		}
	}
}

// onButton reports whether p, in the toast's own space, is on a button:
// a press there clicks rather than swipes.
func (v *toastView) onButton(p f32.Point) bool {
	at := image.Pt(int(p.X), int(p.Y)).Sub(v.bodyAt)
	for _, r := range []image.Rectangle{v.actionAt, v.cancelAt, v.secondaryAt} {
		if !r.Empty() && at.In(r) {
			return true
		}
	}
	return false
}

// follow is sonner's onPointerMove: the first move past a pixel locks the
// axis, and a pull the way toasts at pos do not leave is dampened. They leave
// toward their own side and their own edge (sonner's default swipeDirections).
func (d *toastDrag) follow(gtx C, delta f32.Point, pos ToastPosition) {
	if d.axis == 0 && (abs32(delta.X) > 1 || abs32(delta.Y) > 1) {
		d.axis = 'y'
		if abs32(delta.X) > abs32(delta.Y) {
			d.axis = 'x'
		}
	}
	perDp := gtx.Metric.PxPerDp
	d.amount = f32.Point{}
	switch d.axis {
	case 'x':
		d.amount.X = swipeAmount(delta.X/perDp, (delta.X > 0) != pos.left()) * perDp
	case 'y':
		d.amount.Y = swipeAmount(delta.Y/perDp, (delta.Y < 0) != pos.bottom()) * perDp
	}
}

// release is sonner's onPointerUp: far or fast enough sends the toast out
// the way it went, anything less springs back.
func (s ToasterStyle) release(gtx C, e *toastEntry) {
	v := &e.view
	d := v.drag
	v.drag = toastDrag{}
	amount := d.amount.X
	if d.axis != 'x' {
		amount = d.amount.Y
	}
	if !swipeDismisses(amount/gtx.Metric.PxPerDp, gtx.Now.Sub(d.at)) {
		return
	}
	v.swipedOut, v.outFrom = true, d.amount
	v.outDir = f32.Pt(0, sign(amount))
	if d.axis == 'x' {
		v.outDir = f32.Pt(sign(amount), 0)
	}
	s.State.stack.dismiss(e.ID, gtx.Now)
}

// swipeDismisses is sonner's onPointerUp test on a swipe of amount dp that
// took elapsed: past the threshold, or faster than the velocity.
func swipeDismisses(amount float32, elapsed time.Duration) bool {
	a := abs32(amount)
	if a >= float32(toastSwipeThreshold) {
		return true
	}
	ms := float32(elapsed) / float32(time.Millisecond)
	return a > 0 && (ms <= 0 || a/ms > toastSwipeVelocity)
}

// swipeAmount is how far the toast follows a pull of delta dp: all the way
// toward a side it leaves by (allowed), dampened toward any other.
func swipeAmount(delta float32, allowed bool) float32 {
	if allowed {
		return delta
	}
	return delta / (1.5 + abs32(delta)/toastSwipeDampen)
}

// areas lays out the pointer areas over the painted stack: one over all of
// it for hover, and each toast's, untouched by a swipe so drag positions
// stay put, with its buttons'.
func (s ToasterStyle) areas(gtx C) {
	t := s.State
	var zone image.Rectangle
	for _, e := range t.stack.entries {
		if e.view.canHit {
			zone = zone.Union(boundsOf(e.view.place, e.view.boxSize))
		}
	}
	defer clip.Rect(zone).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, t)
	entries := t.stack.entries
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if !e.view.canHit {
			continue
		}
		v := &e.view
		tr := op.Affine(v.place).Push(gtx.Ops)
		area := clip.Rect{Max: v.boxSize}.Push(gtx.Ops)
		event.Op(gtx.Ops, e)
		buttonArea(gtx, &v.action, v.actionAt.Add(v.bodyAt))
		buttonArea(gtx, &v.cancel, v.cancelAt.Add(v.bodyAt))
		buttonArea(gtx, &v.secondary, v.secondaryAt.Add(v.bodyAt))
		area.Pop()
		tr.Pop()
	}
}

func buttonArea(gtx C, click *widget.Clickable, r image.Rectangle) {
	if r.Empty() {
		return
	}
	defer op.Offset(r.Min).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(r.Size())
	click.Layout(gtx, func(gtx C) D {
		pointer.CursorPointer.Add(gtx.Ops)
		return D{Size: gtx.Constraints.Min}
	})
}

// boundsOf is the window-aligned box around size under a.
func boundsOf(a f32.Affine2D, size image.Point) image.Rectangle {
	lo := a.Transform(f32.Point{})
	hi := a.Transform(f32.Pt(float32(size.X), float32(size.Y)))
	return image.Rect(int(lo.X), int(lo.Y), int(hi.X+0.5), int(hi.Y+0.5))
}

// paint draws one toast where place put it, plus its swipe.
func (s ToasterStyle) paint(gtx C, e *toastEntry) {
	v := &e.view
	swipe, opacity := v.swipe, v.alpha
	if v.swipedOut {
		p := min(float32(gtx.Now.Sub(e.removedAt))/float32(toastSwipeOut), 1)
		eased := cssEaseOut(p)
		extent := f32.Pt(float32(v.boxSize.X), float32(v.boxSize.Y))
		swipe = v.outFrom.Add(f32.Pt(v.outDir.X*extent.X, v.outDir.Y*extent.Y).Mul(eased))
		opacity *= 1 - eased
		if p < 1 {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if opacity <= 0 {
		return
	}
	defer op.Affine(v.place.Offset(swipe)).Push(gtx.Ops).Pop()
	defer paint.PushOpacity(gtx.Ops, opacity).Pop()
	s.drawBox(gtx, v)
}
