package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui/icons"
)

const pillPadY unit.Dp = 3

// PillStyle is a rounded-full box with an inset 1px border around Content.
type PillStyle struct {
	Content           layout.Widget
	PadLeft, PadRight unit.Dp
	theme             *Theme
}

func (th *Theme) Pill(content layout.Widget, padLeft, padRight unit.Dp) PillStyle {
	return PillStyle{Content: content, PadLeft: padLeft, PadRight: padRight, theme: th}
}

func (p PillStyle) Layout(gtx C) D {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	d := layout.Inset{Left: p.PadLeft, Right: p.PadRight, Top: pillPadY, Bottom: pillPadY}.Layout(gtx, p.Content)
	call := macro.Stop()
	InsetBorder(gtx, d.Size, d.Size.Y/2, p.theme.Over(p.theme.Border))
	call.Add(gtx.Ops)
	return d
}

const keyCapSize unit.Sp = 10 // text-[0.625rem]

// KeyCapStyle is Keys: rounded, a hairline border with a thicker bottom one,
// on bg-muted, 10px mono text.
type KeyCapStyle struct {
	Keys  string
	theme *Theme
}

func (th *Theme) KeyCap(keys string) KeyCapStyle { return KeyCapStyle{Keys: keys, theme: th} }

func (k KeyCapStyle) Layout(gtx C) D {
	th := k.theme
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	d := layout.Inset{Left: Space(2), Right: Space(2), Top: Space(1.5), Bottom: Space(1.5) + 1}.Layout(gtx,
		th.Text(keyCapSize, k.Keys).Mono().Muted().Leading(keyCapSize).Layout)
	call := macro.Stop()
	box := gtx
	box.Constraints = layout.Exact(d.Size)
	Fill(box, th.Over(th.Palette.Muted), RadiusSM)
	border := th.Over(th.Border)
	InsetBorder(gtx, d.Size, gtx.Dp(RadiusSM), border)
	r := gtx.Dp(RadiusSM)
	paint.FillShape(gtx.Ops, border, clip.Rect{Min: image.Pt(r, d.Size.Y-2*gtx.Dp(1)), Max: image.Pt(d.Size.X-r, d.Size.Y-gtx.Dp(1))}.Op())
	call.Add(gtx.Ops)
	return d
}

// MetaGlyphStyle is a size-3 glyph and its text-xs words, gap-1.
type MetaGlyphStyle struct {
	Icon  *icons.Icon
	Text  string
	Color color.NRGBA
	theme *Theme
}

func (th *Theme) MetaGlyph(ic *icons.Icon, text string, fg color.NRGBA) MetaGlyphStyle {
	return MetaGlyphStyle{Icon: ic, Text: text, Color: fg, theme: th}
}

func (m MetaGlyphStyle) Layout(gtx C) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return m.Icon.Layout(gtx, Space(3), m.Color) }),
		Gap(1),
		layout.Rigid(m.theme.Text(TextXS, m.Text).In(m.Color).Layout),
	)
}
