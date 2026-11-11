package ui

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui/icons"
)

const (
	// dropHintFill is the accent's opacity over what is dropped on.
	dropHintFill = 0.45
	// dropHintDetailMin is the height below which the detail line goes.
	dropHintDetailMin = unit.Dp(120)
)

// DropHintStyle covers what a file drag is over: Icon, then the headline
// Before Label After with Label in its own weight, and under it Detail, which
// is dropped when the box is too short for it. Its theme draws on the
// covered surface.
type DropHintStyle struct {
	Icon                         *icons.Icon
	Before, Label, After, Detail string
	theme                        *Theme
}

func (th *Theme) DropHint(ic *icons.Icon, before, label, after, detail string) DropHintStyle {
	return DropHintStyle{Icon: ic, Before: before, Label: label, After: after, Detail: detail, theme: th}
}

func (h DropHintStyle) Layout(gtx C) D {
	// The fill stays translucent so what is under it reads through; th.Over
	// is only the color the text measures its contrast against.
	tint := Alpha(h.theme.Accent, dropHintFill)
	paint.Fill(gtx.Ops, tint)
	on := h.theme.On(h.theme.Over(tint))
	showDetail := gtx.Constraints.Max.Y >= gtx.Dp(dropHintDetailMin)
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{Left: Space(4), Right: Space(4)}.Layout(gtx, func(gtx C) D {
			rows := []layout.FlexChild{
				layout.Rigid(func(gtx C) D { return h.Icon.Layout(gtx, Space(5), on.Foreground) }),
				VGap(1.5),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(on.Text(TextSM, h.Before).Layout),
						layout.Rigid(on.Text(TextSM, h.Label).Weight(font.SemiBold).Layout),
						layout.Rigid(on.Text(TextSM, h.After).Layout),
					)
				}),
			}
			if showDetail {
				rows = append(rows, VGap(1.5), layout.Rigid(on.Text(TextXS, h.Detail).Muted().Layout))
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, rows...)
		})
	})
}

// KeyHintStyle is a key cap and the word beside it.
type KeyHintStyle struct {
	Keys, Label string
	theme       *Theme
}

func (th *Theme) KeyHint(keys, label string) KeyHintStyle {
	return KeyHintStyle{Keys: keys, Label: label, theme: th}
}

func (k KeyHintStyle) Layout(gtx C) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return k.theme.KeyCap(k.Keys).Layout(gtx) }),
		Gap(1.5),
		layout.Rigid(k.theme.Text(TextXS, k.Label).Muted().Layout),
	)
}
