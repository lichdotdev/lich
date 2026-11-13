package ui

import (
	"image"
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

const (
	countBadgeSize  = unit.Dp(18)
	countBadgePadX  = unit.Dp(5)
	countBadgeFill  = float32(0.2)
	abbrBadgeSize   = unit.Dp(20)
	abbrBadgeText   = unit.Sp(9) // text-[0.5625rem]
	outlineBadgeRim = 0.5
)

// CountBadge is a count in the wait tone on its own 20% tint, a pill that is a
// circle up to two digits. It draws nothing for none.
func (th *Theme) CountBadge(gtx C, n int) D {
	if n == 0 {
		return D{}
	}
	label := th.Text(Text2XS, strconv.Itoa(n)).In(th.ToneWait).Weight(font.SemiBold)
	gtx.Constraints.Min = image.Point{}
	ink := inkBounds(gtx, th, label)
	size := image.Pt(max(ink.Dx()+gtx.Dp(countBadgePadX)*2, gtx.Dp(countBadgeSize)), gtx.Dp(countBadgeSize))
	box := gtx
	box.Constraints = layout.Exact(size)
	Fill(box, th.Over(Alpha(th.ToneWait, countBadgeFill)), countBadgeSize/2)
	center := ink.Min.Add(ink.Max).Div(2)
	defer op.Offset(size.Div(2).Sub(center)).Push(gtx.Ops).Pop()
	label.Layout(gtx)
	return D{Size: size}
}

// inkBounds is the box label's glyphs paint, where its Layout puts them: a
// digit's advance carries side bearings that sit it off center in a pill.
func inkBounds(gtx C, th *Theme, label LabelStyle) image.Rectangle {
	th.Shaper.LayoutString(text.Parameters{Font: label.Font, PxPerEm: fixed.I(gtx.Sp(label.Size)), MaxLines: 1, MaxWidth: gtx.Constraints.Max.X, Locale: gtx.Locale}, label.Text)
	var ink image.Rectangle
	for g, ok := th.Shaper.NextGlyph(); ok; g, ok = th.Shaper.NextGlyph() {
		x, y := g.X, fixed.I(int(g.Y))
		ink = ink.Union(image.Rect((x + g.Bounds.Min.X).Floor(), (y + g.Bounds.Min.Y).Floor(), (x + g.Bounds.Max.X).Ceil(), (y + g.Bounds.Max.Y).Ceil()))
	}
	return ink
}

// OutlineBadgeStyle is Text, muted, in a rounded outline: a state set apart
// from the amber a verdict beside it can take.
type OutlineBadgeStyle struct {
	Text  string
	theme *Theme
}

func (th *Theme) OutlineBadge(text string) OutlineBadgeStyle {
	return OutlineBadgeStyle{Text: text, theme: th}
}

// The outline is half the muted token so it stays visible on a selected row,
// where the Border token sits on the Accent fill at nearly its own shade.
func (b OutlineBadgeStyle) Layout(gtx C) D {
	th := b.theme
	gtx.Constraints.Min = image.Point{}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			rr := clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(RadiusSM))
			border := th.Over(Alpha(th.MutedForeground, outlineBadgeRim))
			paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
			return D{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx C) D {
			return layout.Inset{Left: Space(1), Right: Space(1)}.Layout(gtx,
				th.Text(TextXS, b.Text).Muted().Layout)
		}),
	)
}

// AbbrBadgeStyle is a short bold abbreviation in fg on a 20dp square of bg.
type AbbrBadgeStyle struct {
	Abbr   string
	Bg, Fg color.NRGBA
	theme  *Theme
}

func (th *Theme) AbbrBadge(abbr string, bg, fg color.NRGBA) AbbrBadgeStyle {
	return AbbrBadgeStyle{Abbr: abbr, Bg: bg, Fg: fg, theme: th}
}

func (b AbbrBadgeStyle) Layout(gtx C) D {
	size := gtx.Dp(abbrBadgeSize)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	Fill(gtx, b.Bg, RadiusSM)
	return layout.Center.Layout(gtx, b.theme.On(b.Bg).Text(abbrBadgeText, b.Abbr).In(b.Fg).Weight(font.Bold).Layout)
}

// Tag is text in fg inside an outlined pill.
func (th *Theme) Tag(text string, fg color.NRGBA) PillStyle {
	return th.Pill(th.Text(TextXS, text).In(fg).Layout, Space(2), Space(2))
}
