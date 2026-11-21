package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// stepperValueMinWidth is the value box's min-w-16: as wide as its longest
// reading, so stepping never shifts the buttons around it.
var stepperValueMinWidth = ui.Space(16)

// stepperTrackPad is the track's padding, the segmented control's p-[3px]
// and its border, so a stepper stands as tall as a ToggleGroup beside it.
var stepperTrackPad = ui.Space(1)

// stepperState is one Stepper's widget state.
type stepperState struct {
	dec, value, inc          ui.Clickable
	decTip, valueTip, incTip ui.Tooltip
}

// stepperSpec is what a Stepper steps: value within min and max by step,
// the box reading display, a click on it returning to fallback.
type stepperSpec struct {
	value, min, max, step, fallback float64
	display, reset                  string
	dec, inc                        *icons.Icon
	decLabel, incLabel              string
	onChange                        func(float64)
}

// layout is common/Stepper: two ghost icon buttons around the reading, which
// is itself the reset, never disabled, in one bordered track as the segmented
// control's.
func (st *stepperState) layout(gtx C, th *ui.Theme, root *ui.Root, s stepperSpec) D {
	custom := s.value != s.fallback
	if st.dec.Clicked(gtx) && s.value > s.min {
		s.onChange(s.value - s.step)
	}
	if st.inc.Clicked(gtx) && s.value < s.max {
		s.onChange(s.value + s.step)
	}
	if st.value.Clicked(gtx) && custom {
		s.onChange(s.fallback)
	}
	icon := func(click *ui.Clickable, tip *ui.Tooltip, ic *icons.Icon, label string, disabled bool) layout.Widget {
		b := th.Button(click, "")
		b.Icon, b.Variant, b.Size, b.Disabled = ic, ui.VariantGhost, ui.SizeIconSM, disabled
		return func(gtx C) D { return th.Tooltip(root, tip, label).Layout(gtx, b.Layout) }
	}
	valueHint := i18n.T("common.stepper.default")
	if custom {
		valueHint = s.reset
	}
	value := th.Button(&st.value, s.display)
	value.Variant, value.Size = ui.VariantGhost, ui.SizeSM
	macro := op.Record(gtx.Ops)
	d := layout.UniformInset(stepperTrackPad).Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(icon(&st.dec, &st.decTip, s.dec, s.decLabel, s.value <= s.min)),
			layout.Rigid(func(gtx C) D {
				return th.Tooltip(root, &st.valueTip, valueHint).Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min = image.Pt(gtx.Dp(stepperValueMinWidth), 0)
					return value.Layout(gtx)
				})
			}),
			layout.Rigid(icon(&st.inc, &st.incTip, s.inc, s.incLabel, s.value >= s.max)),
		)
	})
	call := macro.Stop()
	ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusLG), th.Over(th.Border))
	call.Add(gtx.Ops)
	return d
}
