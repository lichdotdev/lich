package main

import (
	"image"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/ui"
)

// A pane coming into the stage grows out of the edge it was split on, and
// one leaving shrinks back into it, its neighbours giving and taking the
// room over the same time. Only the panes' frames move: every terminal is
// drawn at its final size from the first frame, clipped by its frame, so a
// PTY is resized once and its agent redraws once (the dock does the same).

// The timings a block of the sidebar opens and a card leaves with.
const (
	paneEnter = 200 * time.Millisecond
	paneLeave = 180 * time.Millisecond
)

var (
	paneEaseOut = ui.CubicBezier(0, 0, 0.2, 1)
	paneEaseIn  = ui.CubicBezier(0.4, 0, 1, 1)
)

// paneMotion moves the stage's panes from where they were to where they are
// when the set of them changes. A zoom, a divider dragged and another tab
// move nothing: they change no pane's company, or all of it.
type paneMotion struct {
	// last is where each pane was drawn the frame before, now this frame's,
	// and from where they stood when the run began, nil at rest.
	last, now, from map[string]image.Rectangle
	zoomed          bool
	start, at       time.Time
	leaving         bool
}

// begin opens a frame that draws the panes ids, and starts a run when they
// are not the ones drawn before but share one with them.
func (m *paneMotion) begin(now time.Time, zoomed bool, ids []string) {
	m.at, m.now = now, make(map[string]image.Rectangle, len(ids))
	entered, shared := false, false
	for _, id := range ids {
		if _, ok := m.last[id]; ok {
			shared = true
		} else {
			entered = true
		}
	}
	if (entered || len(ids) != len(m.last)) && shared && !zoomed && !m.zoomed {
		m.from, m.start, m.leaving = m.last, now, !entered
	}
	m.zoomed = zoomed
}

// progress is how far the run has come, 1 at rest.
func (m *paneMotion) progress() float32 {
	if m.from == nil {
		return 1
	}
	d, curve := paneEnter, paneEaseOut
	if m.leaving {
		d, curve = paneLeave, paneEaseIn
	}
	p := float32(m.at.Sub(m.start)) / float32(d)
	if p >= 1 {
		m.from = nil
		return 1
	}
	return curve(max(p, 0))
}

// frame is where pane id, whose place is final, is drawn this frame: on its
// way there from where it was, or, for a pane new to the stage, growing out
// of the edge of the pane it took its room from.
func (m *paneMotion) frame(id string, final image.Rectangle) image.Rectangle {
	m.now[id] = final
	p := m.progress()
	if p == 1 {
		return final
	}
	if old, ok := m.from[id]; ok {
		return lerpRect(old, final, p)
	}
	for _, donor := range m.from {
		if edge, ok := collapsed(final, donor); ok {
			return lerpRect(edge, final, p)
		}
	}
	return final
}

// end closes the frame and answers the panes that left, each shrinking into
// the edge of the pane that takes its room.
func (m *paneMotion) end() []image.Rectangle {
	var ghosts []image.Rectangle
	if p := m.progress(); p < 1 {
		for id, old := range m.from {
			if _, stays := m.now[id]; stays {
				continue
			}
			for _, heir := range m.now {
				if edge, ok := collapsed(old, heir); ok {
					ghosts = append(ghosts, lerpRect(old, edge, p))
					break
				}
			}
		}
	}
	m.last = m.now
	return ghosts
}

// collapsed is inner flattened onto the edge it shares with outer, which
// holds it: the edge a pane carved out of outer grows from. It reports false
// when outer does not hold inner or inner fills it.
func collapsed(inner, outer image.Rectangle) (image.Rectangle, bool) {
	r := inner
	switch {
	case !inner.In(outer):
		return r, false
	case inner.Max.X == outer.Max.X && inner.Min.X > outer.Min.X:
		r.Min.X = r.Max.X
	case inner.Min.X == outer.Min.X && inner.Max.X < outer.Max.X:
		r.Max.X = r.Min.X
	case inner.Max.Y == outer.Max.Y && inner.Min.Y > outer.Min.Y:
		r.Min.Y = r.Max.Y
	case inner.Min.Y == outer.Min.Y && inner.Max.Y < outer.Max.Y:
		r.Max.Y = r.Min.Y
	default:
		return r, false
	}
	return r, true
}

func lerpRect(a, b image.Rectangle, p float32) image.Rectangle {
	mix := func(x, y int) int { return x + int(float32(y-x)*p+0.5) }
	return image.Rect(mix(a.Min.X, b.Min.X), mix(a.Min.Y, b.Min.Y), mix(a.Max.X, b.Max.X), mix(a.Max.Y, b.Max.Y))
}

// paneLeaf draws a pane at its final place r under the frame it has this
// frame, which is r itself at rest.
func (u *window) paneLeaf(gtx C, id string, r image.Rectangle, draw func() error) error {
	frame := u.layoutStage.motion.frame(id, r)
	if frame == r {
		return draw()
	}
	shape := clip.UniformRRect(frame, gtx.Dp(paneRadius))
	bg := u.terms.backgroundOf(id)
	if bg.A == 0 {
		bg = u.theme.Background
	}
	paint.FillShape(gtx.Ops, bg, shape.Op(gtx.Ops))
	area := shape.Push(gtx.Ops)
	err := draw()
	area.Pop()
	paneBorder(gtx, u.theme, shape)
	return err
}

// endPaneMotion draws the panes that left this run and asks for the next
// frame while one runs.
func (u *window) endPaneMotion(gtx C) {
	m := &u.layoutStage.motion
	for _, r := range m.end() {
		shape := clip.UniformRRect(r, gtx.Dp(paneRadius))
		paint.FillShape(gtx.Ops, u.theme.Background, shape.Op(gtx.Ops))
		paneBorder(gtx, u.theme, shape)
	}
	if m.from != nil {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func paneBorder(gtx C, th *ui.Theme, shape clip.RRect) {
	on := th.On(th.Background)
	paint.FillShape(gtx.Ops, on.Over(on.Border), clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
}
