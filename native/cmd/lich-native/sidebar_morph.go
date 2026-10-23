package main

import (
	"image"
	"math"
	"strconv"
	"strings"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/lichdotdev/lich/native/ui"
)

// sidebarMorphDuration times the sidebar's collapse and expand.
const sidebarMorphDuration = 180 * time.Millisecond

var sidebarMorphCurve = ui.CubicBezier(0.2, 0, 0, 1)

// The glyphs both shapes name.
const (
	glyphToggle  = "toggle"
	glyphNew     = "new"
	glyphSession = "session:" // + the session id
)

// morphProbe prefixes the semantic description that marks a glyph's box in
// the capture frame; only sidebarMorph.capture reads it.
const morphProbe = "lich-morph:"

type morphPhase int

const (
	morphIdle morphPhase = iota
	// morphCapture is the one frame that draws both shapes, the new one
	// invisible, so the router can say where each named glyph sits in each.
	morphCapture
	morphFlying
)

// morphSide is which of the two shapes a glyph is drawn in.
type morphSide int

const (
	sideFrom morphSide = iota
	sideTo
)

// sidebarMorph is the morph between the two shapes: the sidebar's box
// eases between the two shapes' widths, the shapes crossfade inside it, and
// the named glyphs fly from their box in one shape to their box in the other.
type sidebarMorph struct {
	phase morphPhase
	// capturing is whether this whole frame is the capture frame; the frame
	// whose click began the morph drew part of itself before the click.
	capturing bool
	start     time.Time
	p         float32 // this frame's eased progress
	side      morphSide
	boxes     [2]map[string]image.Rectangle // by side, in window coordinates

	// landed is whether this frame ended the morph.
	landed bool
}

// begin starts a morph to the shape the sidebar was just switched to.
func (m *sidebarMorph) begin(gtx C) {
	m.phase, m.start = morphCapture, time.Time{}
	gtx.Execute(op.InvalidateCmd{})
}

func (m *sidebarMorph) active() bool { return m.phase != morphIdle }

// holdsStage is whether the stage's size this frame holds until the morph
// lands, or is the size it landed on.
func (m *sidebarMorph) holdsStage() bool { return m.active() || m.landed }

// advance sets the frame's progress. It runs once, before the frame is laid
// out, so every part of the frame reads the same one.
func (m *sidebarMorph) advance(gtx C) {
	m.capturing = m.phase == morphCapture
	m.landed = false
	switch m.phase {
	case morphIdle:
		return
	case morphCapture:
		m.p = 0
	case morphFlying:
		if m.start.IsZero() {
			m.start = gtx.Now
		}
		if m.p = morphProgress(gtx.Now.Sub(m.start)); m.p >= 1 {
			m.phase, m.landed = morphIdle, true
			return
		}
	}
	gtx.Execute(op.InvalidateCmd{})
}

func morphProgress(elapsed time.Duration) float32 {
	t := float32(elapsed) / float32(sidebarMorphDuration)
	if t >= 1 {
		return 1
	}
	return sidebarMorphCurve(max(t, 0))
}

// glyph draws a named glyph of the shape being drawn. In the capture frame it
// also marks the glyph's box for capture; in flight it only takes its room,
// since flight draws it.
func (m *sidebarMorph) glyph(gtx C, name string, w layout.Widget) D {
	if m.phase == morphIdle {
		return w(gtx)
	}
	rec := op.Record(gtx.Ops)
	d := w(gtx)
	call := rec.Stop()
	if m.phase == morphCapture {
		call.Add(gtx.Ops)
		probe := clip.Rect{Max: d.Size}.Push(gtx.Ops)
		semantic.DescriptionOp(morphProbe + strconv.Itoa(int(m.side)) + ":" + name).Add(gtx.Ops)
		probe.Pop()
	}
	return d
}

// capture reads, from the finished capture frame, where each shape drew its
// named glyphs, and starts the flight. Gio keeps no transform a widget could
// read back, so the boxes come from a router of its own, the semantic tree
// carrying them in window coordinates.
func (m *sidebarMorph) capture(frame *op.Ops) {
	if !m.capturing || m.phase != morphCapture {
		return
	}
	var r input.Router
	r.Frame(frame)
	m.boxes = [2]map[string]image.Rectangle{{}, {}}
	for _, n := range r.AppendSemantics(nil) {
		rest, ok := strings.CutPrefix(n.Desc.Description, morphProbe)
		if !ok {
			continue
		}
		side, name, _ := strings.Cut(rest, ":")
		if i, err := strconv.Atoi(side); err == nil && i >= 0 && i < len(m.boxes) {
			m.boxes[i][name] = n.Desc.Bounds
		}
	}
	m.phase = morphFlying
}

// flyBox is a glyph's box at p: its corner eased from one box to the other,
// at the size of the box it is headed for.
func flyBox(from, to image.Rectangle, p float32) image.Rectangle {
	corner := image.Pt(lerpInt(from.Min.X, to.Min.X, p), lerpInt(from.Min.Y, to.Min.Y, p))
	return image.Rectangle{Min: corner, Max: corner.Add(to.Size())}
}

func lerpInt(a, b int, p float32) int {
	return a + int(math.Round(float64(float32(b-a)*p)))
}

// boxWidth is a shape's width: the panel's stored width, or the rail's w-8.
func (s *sidebar) boxWidth(gtx C, open bool) int {
	if open {
		return gtx.Dp(unit.Dp(s.widthRem * remDp))
	}
	return gtx.Dp(ui.Space(8))
}

// column is the sidebar's width this frame and how open it is, 0 for the
// rail to 1 for the panel, mid-morph in between: what the topbar's brand
// column follows.
func (s *sidebar) column(gtx C) (int, float32) {
	return brandColumn(s.boxWidth(gtx, false), s.boxWidth(gtx, true), s.open, s.morph.active(), s.morph.p)
}

// stageReserve is the sidebar width the stage makes room for. Mid-morph it is
// the narrower shape's for the whole flight: the stage keeps one size and
// slides with the sidebar's edge, clipped by the window. A terminal resize
// reaches the PTY, so the stage resizes once, as a collapse starts or once an
// expand lands, and never to a size in between.
func (s *sidebar) stageReserve(gtx C) int {
	if !s.morph.active() {
		return s.boxWidth(gtx, s.open)
	}
	return min(s.boxWidth(gtx, true), s.boxWidth(gtx, false))
}

// stageOverhang is how much wider the stage is this frame than where the
// morph leaves it: the shapes' difference through an expand, else 0.
func (s *sidebar) stageOverhang(gtx C) int {
	return s.boxWidth(gtx, s.open) - s.stageReserve(gtx)
}

// morphing is the sidebar mid-morph: its box eases between the two shapes'
// widths, each shape drawn at its own size from the box's leading corner and
// clipped by it, the old one fading out as the new one fades in.
func (s *sidebar) morphing(gtx C, th *ui.Theme, m *model, active string) (D, sidebarAction) {
	mo := &s.morph
	width := lerpInt(s.boxWidth(gtx, !s.open), s.boxWidth(gtx, s.open), mo.p)
	size := image.Pt(width, gtx.Constraints.Max.Y)
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	mo.side = sideFrom
	s.faded(gtx.Disabled(), th, m, active, !s.open, 1-mo.p)
	mo.side = sideTo
	to := gtx
	if mo.phase == morphCapture {
		to = gtx.Disabled()
	}
	return D{Size: size}, s.faded(to, th, m, active, s.open, mo.p)
}

func (s *sidebar) faded(gtx C, th *ui.Theme, m *model, active string, open bool, alpha float32) sidebarAction {
	defer paint.PushOpacity(gtx.Ops, alpha).Pop()
	_, act := s.shape(gtx, th, m, active, open)
	return act
}

// flight draws the flying glyphs over everything, as the browser draws its
// view-transition groups over the page. A glyph both shapes name flies
// between its two boxes; one only a shape names fades in place.
func (s *sidebar) flight(gtx C, th *ui.Theme, m *model) {
	mo := &s.morph
	if mo.phase != morphFlying {
		return
	}
	sessions := map[string]*sessionView{}
	view := m.sidebarBlocks(sidebarFilter{})
	for bi := range view.blocks {
		for si := range view.blocks[bi].sessions {
			sv := &view.blocks[bi].sessions[si]
			sessions[sv.ID] = sv
		}
	}
	for name, from := range mo.boxes[sideFrom] {
		to, ok := mo.boxes[sideTo][name]
		if !ok {
			s.flyingGlyph(gtx, th, name, sideFrom, from, 1-mo.p, sessions)
			continue
		}
		box := flyBox(from, to, mo.p)
		if name != glyphToggle {
			s.flyingGlyph(gtx, th, name, sideTo, box, 1, sessions)
			continue
		}
		// The toggle is the one glyph the shapes draw differently.
		s.flyingGlyph(gtx, th, name, sideFrom, box, 1-mo.p, sessions)
		s.flyingGlyph(gtx, th, name, sideTo, box, mo.p, sessions)
	}
	for name, to := range mo.boxes[sideTo] {
		if _, ok := mo.boxes[sideFrom][name]; !ok {
			s.flyingGlyph(gtx, th, name, sideTo, to, mo.p, sessions)
		}
	}
}

func (s *sidebar) flyingGlyph(gtx C, th *ui.Theme, name string, side morphSide, box image.Rectangle, alpha float32, sessions map[string]*sessionView) {
	defer op.Offset(box.Min).Push(gtx.Ops).Pop()
	defer paint.PushOpacity(gtx.Ops, alpha).Pop()
	gtx.Constraints = layout.Exact(box.Size())
	panel := (side == sideTo) == s.open
	switch {
	case name == glyphToggle && panel:
		icCollapse.Layout(gtx, ui.Space(4), th.MutedForeground)
	case name == glyphToggle:
		icExpand.Layout(gtx, ui.Space(4), th.MutedForeground)
	case name == glyphNew:
		icPlus.Layout(gtx, ui.Space(4), th.MutedForeground)
	default:
		if sv := sessions[strings.TrimPrefix(name, glyphSession)]; sv != nil {
			s.statusGlyph(gtx, th, sv)
		}
	}
}
