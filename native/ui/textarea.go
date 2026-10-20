package ui

import (
	"image"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// textareaPadY is shadcn's textarea py-2; its px is the Input's.
var (
	textareaPadY      = Space(2)
	textareaMinHeight = Space(16) // min-h-16
)

// TextareaStyle draws shadcn's Textarea around a caller-owned Editor. It is
// field-sizing-content: as tall as its text, at least min-h-16, and at most
// the maximum height constraint, which is where a max-h-* class goes; past it
// the editor scrolls.
type TextareaStyle struct {
	Editor *widget.Editor
	// Resize is resize-y: a grip in the corner that drags the height. nil is
	// resize-none.
	Resize      *TextareaResize
	Placeholder string
	Width       unit.Dp // 0 is w-full
	Disabled    bool
	Invalid     bool
	theme       *Theme
}

// Textarea is a default textarea over editor.
func (th *Theme) Textarea(editor *widget.Editor, placeholder string) TextareaStyle {
	return TextareaStyle{Editor: editor, Placeholder: placeholder, theme: th}
}

func (s TextareaStyle) Layout(gtx C) D {
	if s.Disabled {
		gtx = gtx.Disabled()
	}
	s.Editor.SingleLine = false
	gtx.Constraints.Max.X = inputWidth(gtx, s.Width)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	field := inputText{theme: s.theme, editor: s.Editor, placeholder: s.Placeholder, size: TextSM,
		font: font.Font{Typeface: s.theme.Sans}, padLeft: inputPadX, padRight: inputPadX, padY: textareaPadY,
		minHeight: textareaMinHeight}
	r := s.Resize
	if r != nil {
		floor := max(gtx.Dp(textareaMinHeight), gtx.Constraints.Min.Y)
		if h := r.update(gtx, floor, gtx.Constraints.Max.Y); h > 0 {
			gtx.Constraints.Max.Y = h
			field.minHeight = unit.Dp(float32(h) / gtx.Metric.PxPerDp)
		}
	}
	st := inputState{invalid: s.Invalid, disabled: s.Disabled}
	d := inputFrame(gtx, s.theme, s.Editor, st, false, func(gtx C) D { return field.layout(gtx, 0) })
	if r != nil {
		r.layout(gtx, s.theme, d.Size)
	}
	return d
}

// textareaGrip is the side of the square in the bottom-right corner that
// takes the resize drag, about Chromium's own resizer.
var textareaGrip = Space(3)

// TextareaResize is a resize-y textarea's grip and the height it was dragged
// to. The caller keeps it beside the Editor; until the first drag the field
// grows with its text, and after it the height holds and the text scrolls,
// as the inline height a drag writes overrides field-sizing-content.
type TextareaResize struct {
	height unit.Dp // 0 until dragged

	dragging bool
	pressY   float32
	// laidOut is the px height the last Layout drew, the box the grip's
	// pointer positions were taken in; pressLaidOut is it at the press.
	laidOut, pressLaidOut int
}

// update reads the frame's drag and returns the px height to lay out, 0
// while the field still sizes to its text, clamped to [floor, ceiling].
func (r *TextareaResize) update(gtx C, floor, ceiling int) int {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: r, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			r.dragging, r.pressY, r.pressLaidOut = true, e.Position.Y, r.laidOut
		case pointer.Drag, pointer.Release:
			if !r.dragging {
				continue
			}
			// The grip travels with the bottom edge, so the positions it
			// reports lag the pointer by how far the edge has moved.
			travel := e.Position.Y - r.pressY + float32(r.laidOut-r.pressLaidOut)
			r.height = unit.Dp(float32(r.pressLaidOut)+travel) / unit.Dp(gtx.Metric.PxPerDp)
			r.dragging = e.Kind != pointer.Release
		case pointer.Cancel:
			r.dragging = false
		}
	}
	if r.height == 0 {
		return 0
	}
	return min(max(gtx.Dp(r.height), floor), ceiling)
}

// layout draws the grip's two diagonal strokes in the corner of a field of
// size and takes the pointer over them, above the field's own handlers.
func (r *TextareaResize) layout(gtx C, th *Theme, size image.Point) {
	r.laidOut = size.Y
	g := gtx.Dp(textareaGrip)
	defer op.Offset(size.Sub(image.Pt(g, g))).Push(gtx.Ops).Pop()
	stroke := float32(gtx.Dp(inputBorderWidth))
	edge := float32(g) - 2*stroke
	var path clip.Path
	path.Begin(gtx.Ops)
	for _, from := range []float32{stroke * 2, float32(g) / 2} {
		path.MoveTo(f32.Pt(edge, from))
		path.LineTo(f32.Pt(from, edge))
	}
	paint.FillShape(gtx.Ops, Alpha(th.Palette.MutedForeground, textareaGripAlpha), clip.Stroke{Path: path.End(), Width: stroke}.Op())
	defer clip.Rect{Max: image.Pt(g, g)}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, r)
	pointer.CursorRowResize.Add(gtx.Ops)
}

// textareaGripAlpha keeps the grip a hint, under the placeholder's weight.
const textareaGripAlpha = 0.6
