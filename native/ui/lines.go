package ui

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// HoverLink is a text-xs link that reads muted and takes the foreground under
// the pointer.
func (th *Theme) HoverLink(gtx C, click *widget.Clickable, text string) D {
	return click.Layout(gtx, func(gtx C) D {
		fg := th.MutedForeground
		if click.Hovered() {
			fg = th.Foreground
		}
		return th.Text(TextXS, text).In(fg).Layout(gtx)
	})
}

// TrailingIconLink is HoverLink with ic at the end of the text: a link that
// leaves the window.
func (th *Theme) TrailingIconLink(gtx C, click *widget.Clickable, label string, ic *icons.Icon) D {
	return click.Layout(gtx, func(gtx C) D {
		ink := th.MutedForeground
		if click.Hovered() {
			ink = th.Foreground
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(TextXS, label).In(ink).Layout),
			Gap(1),
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, Space(3), ink) }),
		)
	})
}

// TwoLinesStyle is a text-sm line over a muted text-xs one: a menu item's name
// and what places it.
type TwoLinesStyle struct {
	Top, Under string
	theme      *Theme
}

func (th *Theme) TwoLines(top, under string) TwoLinesStyle {
	return TwoLinesStyle{Top: top, Under: under, theme: th}
}

func (t TwoLinesStyle) Layout(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(t.theme.Text(TextSM, t.Top).Layout),
		layout.Rigid(t.theme.Text(TextXS, t.Under).Muted().Layout),
	)
}

// SpinnerLine is a muted size-4 spinner and Label, text-sm: what a dialog
// shows while it waits.
func (th *Theme) SpinnerLine(gtx C, label string) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return th.Spinner(gtx, Space(4), th.MutedForeground) }),
		Gap(2),
		layout.Rigid(th.Text(TextSM, label).Muted().Weight(font.Normal).Layout),
	)
}

// SpinnerBeside is w with a muted size-4 spinner before it.
func (th *Theme) SpinnerBeside(gtx C, w layout.Widget) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return th.Spinner(gtx, Space(4), th.MutedForeground) }),
		Gap(2),
		layout.Rigid(w),
	)
}

// CheckRow is a checkbox beside its text-sm label and, when Note is set, a
// muted text-xs line under it.
func (th *Theme) CheckRow(gtx C, box *widget.Bool, label, note string) D {
	text := []layout.FlexChild{layout.Rigid(th.Text(TextSM, label).Layout)}
	if note != "" {
		text = append(text, layout.Rigid(th.Text(TextXS, note).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: Space(0.5)}.Layout(gtx, th.Checkbox(box).Layout)
		}),
		Gap(2.5),
		layout.Flexed(1, func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, text...) }),
	)
}
