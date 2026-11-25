package main

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/ui"
)

// The motion of what opens over the composer (the panels, the / and @
// lists): it grows out of the corner it was called from, scaling up as its
// edges open from there, and a change of what it shows fades the new content
// in. It shuts at once, so the frame that shut it shows it gone. Timings are
// the house's (DESIGN.md, Motion).
const (
	panelEnter = 160 * time.Millisecond
	panelFade  = 120 * time.Millisecond
	// panelZoomFrom is the scale it grows from.
	panelZoomFrom = 0.92
	// panelOpenFromX and panelOpenFromY are the share of its width and height
	// open at the start, around the corner it grows from.
	panelOpenFromX = 0.22
	panelOpenFromY = 0.15
	// panelFadeShare is the share of the entry its opacity takes to reach 1.
	panelFadeShare = 0.6
	// panelShadowRoom is how far past its edge the clip lets the shadow show.
	panelShadowRoom = 48
)

var panelEaseOut = ui.CubicBezier(0, 0, 0.2, 1)

// panelMotion is one surface's motion over the composer. key is what it
// shows, so a change of it fades the new content in.
type panelMotion struct {
	open  bool
	at    time.Time
	key   string
	faded time.Time
}

// track starts the entry when the surface opens and the fade when key
// changes while it is open.
func (m *panelMotion) track(now time.Time, shown bool, key string) {
	switch {
	case !shown:
		m.open = false
	case !m.open:
		m.open, m.at, m.key, m.faded = true, now, key, time.Time{}
	case key != m.key:
		m.key, m.faded = key, now
	}
}

// pose is how far the surface is open, 0 to 1, asking for frames while it
// moves.
func (m *panelMotion) pose(gtx C) float32 {
	elapsed := gtx.Now.Sub(m.at)
	if elapsed >= panelEnter || elapsed < 0 {
		return 1
	}
	gtx.Execute(op.InvalidateCmd{})
	return panelEaseOut(float32(elapsed) / float32(panelEnter))
}

// push draws what follows in the surface's motion, a surface of size
// growing out of originX on its bottom edge. The returned func pops what it
// pushed.
func (m *panelMotion) push(gtx C, size image.Point, originX int) func() {
	p := m.pose(gtx)
	if p >= 1 {
		return func() {}
	}
	w, h := float32(size.X), float32(size.Y)
	ox := float32(originX)
	room := float32(gtx.Dp(panelShadowRoom))
	lerp := func(a, b float32) int { return int(a + (b-a)*p + 0.5) }
	// from the corner around the origin to the whole surface and its shadow
	r := image.Rect(
		lerp(ox-ox*panelOpenFromX, -room), lerp(h-h*panelOpenFromY, -room),
		lerp(ox+(w-ox)*panelOpenFromX, w+room), lerp(h, h+room))
	zoom := panelZoomFrom + (1-panelZoomFrom)*p
	transform := op.Affine(f32.Affine2D{}.Scale(f32.Pt(ox, h), f32.Pt(zoom, zoom))).Push(gtx.Ops)
	cl := clip.Rect(r).Push(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, min(p/panelFadeShare, 1))
	return func() {
		opacity.Pop()
		cl.Pop()
		transform.Pop()
	}
}

// fade draws what follows fading in after the content changed. The returned
// func pops what it pushed.
func (m *panelMotion) fade(gtx C) func() {
	elapsed := gtx.Now.Sub(m.faded)
	if m.faded.IsZero() || elapsed >= panelFade || elapsed < 0 {
		return func() {}
	}
	gtx.Execute(op.InvalidateCmd{})
	o := paint.PushOpacity(gtx.Ops, panelEaseOut(float32(elapsed)/float32(panelFade)))
	return o.Pop
}

// surfaceOverBox draws content on the popover surface over the composer's
// box, Space(2) above it and no taller than maxH, deferred so it stands over
// the conversation, in m's motion out of the box's left corner, where a /
// or an @ is typed.
func surfaceOverBox(gtx C, th *ui.Theme, m *panelMotion, maxH int, content func(gtx C, th *ui.Theme) D) {
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X, maxH)}
	macro := op.Record(gtx.Ops)
	d := th.PopoverPanel(g, content)
	call := macro.Stop()
	rec := op.Record(gtx.Ops)
	op.Offset(image.Pt(0, -d.Size.Y-gtx.Dp(ui.Space(2)))).Add(gtx.Ops)
	pop := m.push(gtx, d.Size, 0)
	call.Add(gtx.Ops)
	pop()
	op.Defer(gtx.Ops, rec.Stop())
}
