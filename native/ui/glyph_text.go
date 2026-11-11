package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// IconLine is a size-3 glyph in fg and, after a gap-1.5, the rest of a line:
// a tooltip's one fact.
func IconLine(gtx C, ic *icons.Icon, fg color.NRGBA, rest ...layout.FlexChild) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 12, fg) }),
		Gap(1.5),
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, append(children, rest...)...)
}

// IconLabelStyle is a size-3 glyph in Glyph and, after a gap-1, Label filling
// the rest of the line.
type IconLabelStyle struct {
	Icon  *icons.Icon
	Glyph color.NRGBA
	Label LabelStyle
}

func IconLabel(ic *icons.Icon, glyph color.NRGBA, label LabelStyle) IconLabelStyle {
	return IconLabelStyle{Icon: ic, Glyph: glyph, Label: label}
}

func (l IconLabelStyle) Layout(gtx C) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return l.Icon.Layout(gtx, 12, l.Glyph) }),
		Gap(1),
		layout.Flexed(1, l.Label.Layout),
	)
}

// MonoGlyphStyle is a size-3 glyph and its text-xs monospace words, gap-1:
// MetaGlyph set in the mono face.
type MonoGlyphStyle struct {
	Icon  *icons.Icon
	Text  string
	Color color.NRGBA
	theme *Theme
}

func (th *Theme) MonoGlyph(ic *icons.Icon, text string, fg color.NRGBA) MonoGlyphStyle {
	return MonoGlyphStyle{Icon: ic, Text: text, Color: fg, theme: th}
}

func (m MonoGlyphStyle) Layout(gtx C) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return m.Icon.Layout(gtx, Space(3), m.Color) }),
		Gap(1),
		layout.Rigid(m.theme.Text(TextXS, m.Text).Mono().In(m.Color).Layout),
	)
}

// MutedGlyphStyle is a glyph of Size in the muted foreground: the lead of a
// list row.
type MutedGlyphStyle struct {
	Icon  *icons.Icon
	Size  unit.Dp
	theme *Theme
}

func (th *Theme) MutedGlyph(ic *icons.Icon, size unit.Dp) MutedGlyphStyle {
	return MutedGlyphStyle{Icon: ic, Size: size, theme: th}
}

func (g MutedGlyphStyle) Layout(gtx C) D { return g.Icon.Layout(gtx, g.Size, g.theme.MutedForeground) }

// CenteredIcon is ic of size in c, centered in the box it is given at a
// fractional offset: a box of odd leftover would round the glyph half a pixel
// off.
func CenteredIcon(gtx C, ic *icons.Icon, size unit.Dp, c color.NRGBA) D {
	box := gtx.Constraints.Min
	off := f32.Pt(float32(box.X-gtx.Dp(size))/2, float32(box.Y-gtx.Dp(size))/2)
	defer op.Affine(f32.Affine2D{}.Offset(off)).Push(gtx.Ops).Pop()
	ic.Layout(gtx, size, c)
	return D{Size: box}
}

// BoxIcon is ic centered in the box it is given, as large as the box allows.
func BoxIcon(gtx C, ic *icons.Icon, c color.NRGBA) D {
	box := gtx.Constraints.Min
	return CenteredIcon(gtx, ic, unit.Dp(float32(min(box.X, box.Y))/gtx.Metric.PxPerDp), c)
}

// TriggerChevron is a size-4 chevron in the foreground, pointing down when
// closed and up when open: the end of a trigger that opens a popover.
func (th *Theme) TriggerChevron(gtx C, open bool) D {
	if open {
		size := float32(gtx.Dp(Space(4)))
		defer op.Affine(f32.AffineId().Rotate(f32.Pt(size/2, size/2), math.Pi)).Push(gtx.Ops).Pop()
	}
	return selectChevron.Layout(gtx, Space(4), th.Foreground)
}

const (
	TinyTextSize    unit.Sp = 10
	TinyTextLeading unit.Sp = 14.3 // 20/14 of the 10px size
	tinyChipBorder          = 0.4  // the chip's border, its tone at 40%
)

// TinyChipStyle is Word in the tone, 10px mono, inside a rounded-full border
// of the same tone.
type TinyChipStyle struct {
	Word  string
	Tone  color.NRGBA
	theme *Theme
}

func (th *Theme) TinyChip(word string, tone color.NRGBA) TinyChipStyle {
	return TinyChipStyle{Word: word, Tone: tone, theme: th}
}

func (c TinyChipStyle) Layout(gtx C) D {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	d := layout.Inset{Left: Space(2), Right: Space(2), Top: 1, Bottom: 1}.Layout(gtx,
		c.theme.Text(TinyTextSize, c.Word).Leading(TinyTextLeading).Mono().In(c.Tone).Layout)
	call := macro.Stop()
	InsetBorder(gtx, d.Size, d.Size.Y/2, c.theme.Over(Alpha(c.Tone, tinyChipBorder)))
	call.Add(gtx.Ops)
	return d
}
