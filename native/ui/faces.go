package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// GhostTextButton is a text-xs medium label in ink, Space(6) high, with a
// muted fill under the pointer.
func (th *Theme) GhostTextButton(gtx C, click *widget.Clickable, label string, ink color.NRGBA) D {
	return click.Layout(gtx, func(gtx C) D {
		macro := op.Record(gtx.Ops)
		gtx.Constraints.Min = image.Pt(0, gtx.Dp(Space(6)))
		d := layout.Inset{Left: Space(2), Right: Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Center.Layout(gtx, th.Text(TextXS, label).In(ink).Weight(font.Medium).Layout)
		})
		call := macro.Stop()
		if click.Hovered() {
			box := gtx
			box.Constraints = layout.Exact(d.Size)
			Fill(box, th.Over(Alpha(th.Muted, 0.5)), RadiusMD)
		}
		call.Add(gtx.Ops)
		return d
	})
}

// CardIconButton is a size-3 glyph turned by turn radians, in a square of
// side with a muted fill under the pointer.
func (th *Theme) CardIconButton(gtx C, click *widget.Clickable, ic *icons.Icon, side unit.Dp, turn float32) D {
	return click.Layout(gtx, func(gtx C) D { return th.cardIcon(gtx, click, ic, side, turn) })
}

// CardIconDisabled is a CardIconButton that takes no input and reads dimmed.
func (th *Theme) CardIconDisabled(gtx C, ic *icons.Icon, side unit.Dp, turn float32) D {
	return th.cardIcon(gtx, nil, ic, side, turn)
}

// cardIcon draws CardIconButton, dimmed and without a hover when click is nil.
func (th *Theme) cardIcon(gtx C, click *widget.Clickable, ic *icons.Icon, side unit.Dp, turn float32) D {
	size := gtx.Dp(side)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	ink := th.Foreground
	if click == nil {
		ink = th.Over(Alpha(ink, 0.5))
	} else if click.Hovered() {
		Fill(gtx, th.Over(Alpha(th.Muted, 0.5)), RadiusMD)
	}
	icon := gtx.Dp(Space(3))
	off := (size - icon) / 2
	defer op.Offset(image.Pt(off, off)).Push(gtx.Ops).Pop()
	c := f32.Pt(float32(icon)/2, float32(icon)/2)
	defer op.Affine(f32.AffineId().Rotate(c, turn)).Push(gtx.Ops).Pop()
	ic.Layout(gtx, Space(3), ink)
	return D{Size: image.Pt(size, size)}
}

// GhostIconButton is a ghost icon button of SizeIcon with hint in a tooltip.
func (th *Theme) GhostIconButton(gtx C, root *Root, click *widget.Clickable, tip *Tooltip, ic *icons.Icon, hint string, disabled bool) D {
	b := th.Button(click, "")
	b.Icon, b.Variant, b.Size, b.Disabled = ic, VariantGhost, SizeIcon, disabled
	return th.Tooltip(root, tip, hint).Layout(gtx, b.Layout)
}

// SquareFace is icon centered on a Space(8) square of bg.
func SquareFace(gtx C, bg color.NRGBA, icon layout.Widget) D {
	size := gtx.Dp(Space(8))
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return Fill(gtx, bg, RadiusMD) },
		func(gtx C) D { return layout.Center.Layout(gtx, icon) },
	)
}

// LabelFace is icon and a text-sm medium label on a Space(9) high row of bg.
func (th *Theme) LabelFace(gtx C, bg color.NRGBA, icon layout.Widget, label string) D {
	h := gtx.Dp(Space(9))
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return Fill(gtx, bg, RadiusMD) },
		func(gtx C) D {
			return layout.Inset{Left: Space(2.5), Right: Space(2.5)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.Y = h
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(icon),
					Gap(2),
					layout.Flexed(1, th.Text(TextSM, label).Weight(font.Medium).Layout),
				)
			})
		},
	)
}
