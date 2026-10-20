package ui

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// sonner 2.0.7's toast classes (dist/styles.css), and the wrap a toast with a
// description gets.
const (
	toastItemGap       unit.Dp = 6  // gap: 6px
	toastWrapRowGap    unit.Dp = 12 // row-gap: 12px
	toastContentGap    unit.Dp = 2  // [data-content] gap: 2px
	toastProgressGap   unit.Dp = 10 // gap-2.5
	toastTitleSize     unit.Sp = 13 // font-size: 13px
	toastTitleLeading  unit.Sp = 13 * 1.5
	toastDescLeading   unit.Sp = 13 * 1.4
	toastButtonHeight  unit.Dp = 24
	toastButtonPadX    unit.Dp = 8
	toastButtonRadius  unit.Dp = 4
	toastIconBox       unit.Dp = 16
	toastIconMarginEnd unit.Dp = 4
	// The icon box sits 3px into the padding, and its 20px svg 1px further.
	toastIconMarginStart unit.Dp = 3
	toastIconSVG         unit.Dp = 20
	toastIconSVGStart    unit.Dp = 1
	// A line under the title: mt-0.5, and its glyph size-3 with gap-1.
	toastLineGap     unit.Dp = 2
	toastLineIcon    unit.Dp = 12
	toastLineIconGap unit.Dp = 4
	// box-shadow: 0 4px 12px rgba(0,0,0,.1).
	toastShadowY      unit.Dp = 4
	toastShadowBlur   unit.Dp = 12
	toastShadowAlpha          = 0.1
	toastShadowLayers         = 6
	// cancel's background: rgba(0,0,0,.08), or rgba(255,255,255,.3) dark.
	toastCancelLight = 0.08
	toastCancelDark  = 0.3
)

// toastShadowLayerAlpha is one of the stacked rings that stand in for the
// shadow's blur, so that all of them together are black at 10%, compensated
// for the linear-light blend as dialogScrimOpacity is.
var toastShadowLayerAlpha = float32(1 - math.Pow(math.Pow(1-toastShadowAlpha, 2.4), 1.0/toastShadowLayers))

// The description's color is sonner's own, not a token: #3f3f3f, and
// hsl(0, 0%, 91%) dark.
var (
	toastDescLight = color.NRGBA{R: 0x3f, G: 0x3f, B: 0x3f, A: 0xff}
	toastDescDark  = color.NRGBA{R: 0xe8, G: 0xe8, B: 0xe8, A: 0xff}
	white          = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

var toastIcons = map[ToastVariant]*icons.Icon{
	ToastSuccess: icons.Sonner("success"),
	ToastInfo:    icons.Sonner("info"),
	ToastWarning: icons.Sonner("warning"),
	ToastError:   icons.Sonner("error"),
}

// toastSlot is where a toast stands in the stack this frame, in pixels.
type toastSlot struct {
	index      int
	own, front float32 // its natural height and the front toast's
	gap        float32
	expanded   bool
}

// toastTarget is the state a toast's CSS transitions head for.
type toastTarget struct {
	y, scale, alpha, height, inside float32
	move, fade                      time.Duration
}

// targetOf is sonner's data-* rules for a toast in slot: collapsed, the
// ones behind the front peek out gap lower and a step smaller at the front
// one's height, their content hidden; expanded, each sits at its offset.
func targetOf(e *toastEntry, s toastSlot) toastTarget {
	behind := s.index > 0
	t := toastTarget{scale: 1, alpha: 1, height: s.own, inside: 1, move: toastTransition, fade: toastTransition}
	switch {
	case s.expanded:
		t.y = e.view.offset
	case behind:
		t.y = s.gap * float32(s.index)
		t.scale = 1 - toastScaleStep*float32(s.index)
		t.height, t.inside = s.front, 0
	}
	if !visible(s.index) {
		t.alpha = 0
	}
	if !e.removed() || e.view.swipedOut {
		return t
	}
	t.alpha = 0
	switch {
	case !behind:
		t.y = -t.height
	case s.expanded:
		t.y = e.view.offset - t.height
	default:
		t.y, t.scale = toastBehindExitLift*t.height, 1
		t.move, t.fade = toastBehindExit, toastBehindFade
	}
	return t
}

// place measures every toast and moves it toward its target.
func (s ToasterStyle) place(gtx C) {
	t := s.State
	inset := gtx.Dp(toastBorder) + gtx.Dp(toastPadding)
	width := gtx.Dp(toastWidth)
	front := -1
	for _, e := range t.stack.entries {
		s.measure(gtx, e, width-2*inset)
		if front < 0 && !e.removed() {
			front = e.view.bodySize.Y + 2*inset
		}
	}
	slot := toastSlot{front: float32(max(front, 0)), gap: float32(gtx.Dp(toastGap)), expanded: t.hovered || s.Expand}
	var before float32
	live := 0
	for i, e := range t.stack.entries {
		slot.index, slot.own = i, float32(e.view.bodySize.Y+2*inset)
		if !e.removed() {
			e.view.offset = float32(live)*slot.gap + before
			before += slot.own
			live++
		}
		center, scale, y, h := e.view.animate(gtx, targetOf(e, slot), width, inset, e.Description != "" || e.Progress != nil)
		e.view.place = s.Position.placeOf(center, scale, y, h)
		e.view.canHit = visible(i) && !e.removed()
	}
}

// animate runs the toast's transitions toward tg and works out its box, and
// returns what placing it takes: its center, scale, distance from the stack's
// edge and height. A toast drawn for the first time enters from
// translateY(-100%).
func (v *toastView) animate(gtx C, tg toastTarget, width, inset int, wrapped bool) (center f32.Point, scale, y, h float32) {
	if !v.seeded {
		v.seeded = true
		v.y.seed(-tg.height)
		v.opacity.seed(0)
		v.scale.seed(tg.scale)
		v.height.seed(tg.height)
		v.content.seed(tg.inside)
	}
	y = v.y.at(gtx, tg.y, tg.move, ease)
	scale = v.scale.at(gtx, tg.scale, tg.move, ease)
	h = v.height.at(gtx, tg.height, toastTransition, ease)
	v.alpha = v.opacity.at(gtx, tg.alpha, tg.fade, ease)
	v.inside = v.content.at(gtx, tg.inside, toastTransition, ease)
	if v.drag.active {
		v.swipeX.seed(v.drag.amount.X)
		v.swipeY.seed(v.drag.amount.Y)
	}
	v.swipe = f32.Pt(v.swipeX.at(gtx, 0, toastTransition, ease), v.swipeY.at(gtx, 0, toastTransition, ease))
	if v.drag.active {
		v.swipe = v.drag.amount
	}
	v.boxSize = image.Pt(width, int(h+0.5))
	v.bodyAt = image.Pt(inset, inset)
	if !wrapped {
		v.bodyAt.Y = (v.boxSize.Y - v.bodySize.Y) / 2
	}
	return f32.Pt(float32(width)/2, h/2), scale, y, h
}

// measure records the toast's content at the inner width.
func (s ToasterStyle) measure(gtx C, e *toastEntry, inner int) {
	gtx.Constraints = layout.Constraints{Max: image.Pt(inner, gtx.Constraints.Max.Y)}
	macro := op.Record(gtx.Ops)
	d := s.body(gtx, e)
	e.view.body, e.view.bodySize = macro.Stop(), d.Size
}

// drawBox paints the toast's box at the origin: shadow, popover fill,
// border and the content.
func (s ToasterStyle) drawBox(gtx C, v *toastView) {
	th := s.theme
	r := gtx.Dp(RadiusLG)
	toastShadow(gtx, v.boxSize, r)
	rect := clip.UniformRRect(image.Rectangle{Max: v.boxSize}, r)
	paint.FillShape(gtx.Ops, th.Popover, rect.Op(gtx.Ops))
	border := th.On(th.Popover).Over(th.Border)
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rect.Path(gtx.Ops), Width: float32(gtx.Dp(toastBorder))}.Op())
	if v.inside <= 0 {
		return
	}
	defer paint.PushOpacity(gtx.Ops, v.inside).Pop()
	defer op.Offset(v.bodyAt).Push(gtx.Ops).Pop()
	v.body.Add(gtx.Ops)
}

// toastShadow stands in for the blur with rings growing from half the blur
// inside the box to half outside it. The shadow lands on whatever the window
// holds, not a known surface, so it stays translucent.
func toastShadow(gtx C, size image.Point, radius int) {
	blur := float32(gtx.Dp(toastShadowBlur))
	down := gtx.Dp(toastShadowY)
	shade := Alpha(black, toastShadowLayerAlpha)
	for i := range toastShadowLayers {
		grow := int(blur*(float32(i)+0.5)/toastShadowLayers - blur/2)
		box := image.Rect(-grow, down-grow, size.X+grow, size.Y+down+grow)
		paint.FillShape(gtx.Ops, shade, clip.UniformRRect(box, max(radius+grow, 0)).Op(gtx.Ops))
	}
}

// recorded is a widget drawn once to be measured and placed after.
type recorded struct {
	call op.CallOp
	size image.Point
}

func record(gtx C, w layout.Widget) recorded {
	macro := op.Record(gtx.Ops)
	d := w(gtx)
	return recorded{call: macro.Stop(), size: d.Size}
}

func (r recorded) at(gtx C, p image.Point) {
	defer op.Offset(p).Push(gtx.Ops).Pop()
	r.call.Add(gtx.Ops)
}

// body lays the toast's content out as sonner's flex row: icon, title and
// description, then cancel and action, each button pushed right by its
// margin-left: auto. With a description, the buttons wrap onto a row of their
// own under the text, grouped at the right edge; three buttons take that row
// too.
func (s ToasterStyle) body(gtx C, e *toastEntry) D {
	th := s.theme.On(s.theme.Popover)
	gtx.Constraints.Min = image.Point{}
	buttons, rects := s.buttons(gtx, th, e)
	ic := toastGlyphOf(th, e)
	lead := 0
	if ic != nil {
		lead = gtx.Dp(toastIconBox) + gtx.Dp(toastIconMarginEnd) - gtx.Dp(toastIconMarginStart) + gtx.Dp(toastItemGap)
	}
	var d D
	if e.Description != "" || e.Secondary != nil || e.Progress != nil {
		d = s.wrappedBody(gtx, th, e, ic, lead, buttons)
	} else {
		d = s.rowBody(gtx, th, e, ic, lead, buttons)
	}
	e.view.cancelAt, e.view.secondaryAt, e.view.actionAt = rects(e.Cancel), rects(e.Secondary), rects(e.Action)
	return d
}

// placedButton is a recorded button and where the body put it.
type placedButton struct {
	recorded
	of *ToastButton
	at image.Point
}

// buttons records cancel, secondary and action, in that order, and returns a lookup
// of where each landed once the body placed them.
func (s ToasterStyle) buttons(gtx C, th *Theme, e *toastEntry) ([]*placedButton, func(*ToastButton) image.Rectangle) {
	var out []*placedButton
	cancelBg := th.Over(Alpha(black, toastCancelLight))
	if !s.Light {
		cancelBg = th.Over(Alpha(white, toastCancelDark))
	}
	for _, b := range []*ToastButton{e.Cancel, e.Secondary} {
		if b != nil {
			r := record(gtx, func(gtx C) D { return toastButton(gtx, th, b.Label, cancelBg, th.PopoverForeground) })
			out = append(out, &placedButton{recorded: r, of: b})
		}
	}
	if e.Action != nil {
		r := record(gtx, func(gtx C) D { return toastButton(gtx, th, e.Action.Label, th.PopoverForeground, th.Popover) })
		out = append(out, &placedButton{recorded: r, of: e.Action})
	}
	return out, func(b *ToastButton) image.Rectangle {
		for _, p := range out {
			if b != nil && p.of == b {
				return image.Rectangle{Min: p.at, Max: p.at.Add(p.size)}
			}
		}
		return image.Rectangle{}
	}
}

func (s ToasterStyle) rowBody(gtx C, th *Theme, e *toastEntry, ic *toastGlyph, lead int, buttons []*placedButton) D {
	gap := gtx.Dp(toastItemGap)
	inner := gtx.Constraints.Max.X
	used := lead
	height := 0
	for _, b := range buttons {
		used += gap + b.size.X
		height = max(height, b.size.Y)
	}
	cgtx := gtx
	cgtx.Constraints.Max.X = max(inner-used, 0)
	content := record(cgtx, func(gtx C) D { return s.content(gtx, th, e) })
	height = max(height, content.size.Y)
	if ic != nil {
		height = max(height, gtx.Dp(toastIconBox))
		ic.layout(gtx, (height-gtx.Dp(toastIconBox))/2)
	}
	content.at(gtx, image.Pt(lead, (height-content.size.Y)/2))
	x := lead + content.size.X
	if len(buttons) > 0 {
		share := max(inner-used-content.size.X, 0) / len(buttons)
		for _, b := range buttons {
			x += gap + share
			b.at = image.Pt(x, (height-b.size.Y)/2)
			b.recorded.at(gtx, b.at)
			x += b.size.X
		}
	}
	return D{Size: image.Pt(inner, height)}
}

func (s ToasterStyle) wrappedBody(gtx C, th *Theme, e *toastEntry, ic *toastGlyph, lead int, buttons []*placedButton) D {
	inner := gtx.Constraints.Max.X
	cgtx := gtx
	cgtx.Constraints.Max.X = max(inner-lead, 0)
	content := record(cgtx, func(gtx C) D { return s.content(gtx, th, e) })
	height := content.size.Y
	if ic != nil {
		height = max(height, gtx.Dp(toastIconBox))
		ic.layout(gtx, 0)
	}
	content.at(gtx, image.Pt(lead, 0))
	if len(buttons) == 0 {
		return D{Size: image.Pt(inner, height)}
	}
	gap := gtx.Dp(toastItemGap)
	row := height + gtx.Dp(toastWrapRowGap)
	x := inner + gap
	for _, b := range buttons {
		x -= gap + b.size.X
	}
	for _, b := range buttons {
		b.at = image.Pt(x, row)
		b.recorded.at(gtx, b.at)
		x += b.size.X + gap
		height = max(height, row+b.size.Y)
	}
	return D{Size: image.Pt(inner, height)}
}

// content is [data-content]: the title with its lines, and the description
// under it.
func (s ToasterStyle) content(gtx C, th *Theme, e *toastEntry) D {
	title := th.Text(toastTitleSize, e.Title).Weight(font.Medium).Leading(toastTitleLeading).In(th.PopoverForeground)
	title.MaxLines = 0
	head := title.Layout
	if len(e.Lines) > 0 {
		head = func(gtx C) D { return toastLines(gtx, th, title.Layout, e.Lines) }
	}
	if e.Description == "" && e.Progress == nil {
		return head(gtx)
	}
	rows := []layout.FlexChild{layout.Rigid(head)}
	if e.Progress != nil {
		rows = append(rows, layout.Rigid(layout.Spacer{Height: toastProgressGap}.Layout),
			layout.Rigid(th.Progress(e.Progress.Percent).Layout))
	}
	if e.Description != "" {
		descColor := toastDescDark
		if s.Light {
			descColor = toastDescLight
		}
		desc := th.Text(toastTitleSize, e.Description).Leading(toastDescLeading).In(descColor)
		desc.MaxLines = 0
		rows = append(rows, layout.Rigid(layout.Spacer{Height: toastContentGap}.Layout), layout.Rigid(desc.Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// toastLines stacks the title over its muted lines, each truncated to one.
func toastLines(gtx C, th *Theme, title layout.Widget, lines []ToastLine) D {
	children := []layout.FlexChild{layout.Rigid(title)}
	for _, l := range lines {
		children = append(children, layout.Rigid(layout.Spacer{Height: toastLineGap}.Layout), layout.Rigid(func(gtx C) D {
			text := th.Text(TextXS, l.Text).In(th.MutedForeground)
			if l.Icon == nil {
				return text.Layout(gtx)
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return l.Icon.Layout(gtx, toastLineIcon, th.MutedForeground) }),
				layout.Rigid(layout.Spacer{Width: toastLineIconGap}.Layout),
				layout.Rigid(text.Layout),
			)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// toastGlyph is the toast's leading glyph: the variant's 20px svg, or the
// icon option's, drawn at the box's own 16px.
type toastGlyph struct {
	icon  *icons.Icon
	size  unit.Dp
	color color.NRGBA
	spin  bool
}

// toastGlyphOf is the icon option over the variant's, nil for neither.
func toastGlyphOf(th *Theme, e *toastEntry) *toastGlyph {
	if e.Icon != nil || e.Spin {
		g := &toastGlyph{icon: e.Icon, size: toastIconBox, color: th.PopoverForeground, spin: e.Spin}
		if e.IconColor != nil {
			g.color = e.IconColor(th)
		}
		return g
	}
	if ic := toastIcons[e.Variant]; ic != nil {
		return &toastGlyph{icon: ic, size: toastIconSVG, color: th.PopoverForeground}
	}
	return nil
}

// layout draws the glyph in its 16px box, top pixels down, as [data-icon]
// and its svg's negative margins place it: a 20px one overflows the box.
func (g *toastGlyph) layout(gtx C, top int) {
	x := -gtx.Dp(toastIconMarginStart) - gtx.Dp(toastIconSVGStart)
	y := top + (gtx.Dp(toastIconBox)-gtx.Dp(g.size))/2
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	if g.spin {
		Spinner(gtx, g.size, g.color)
		return
	}
	g.icon.Layout(gtx, g.size, g.color)
}

// toastButton is [data-button]: h-24px px-8px rounded-4px, 12px medium.
func toastButton(gtx C, th *Theme, label string, bg, fg color.NRGBA) D {
	text := record(gtx, th.Text(TextXS, label).Weight(font.Medium).In(fg).Layout)
	size := image.Pt(text.size.X+2*gtx.Dp(toastButtonPadX), gtx.Dp(toastButtonHeight))
	paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(toastButtonRadius)).Op(gtx.Ops))
	text.at(gtx, image.Pt(gtx.Dp(toastButtonPadX), (size.Y-text.size.Y)/2))
	return D{Size: size}
}

func abs32(v float32) float32 { return float32(math.Abs(float64(v))) }

func sign(v float32) float32 {
	if v < 0 {
		return -1
	}
	return 1
}
