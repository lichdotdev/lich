package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// PopoverRowStyle is a DropdownMenuItem drawn by hand: px-2 py-1.5,
// rounded-sm, bg-accent while the pointer is on it. Content draws on the
// row's own fill.
type PopoverRowStyle struct {
	Click   *widget.Clickable
	Content func(gtx C, th *Theme) D
	theme   *Theme
}

func (th *Theme) PopoverRow(click *widget.Clickable, content func(gtx C, th *Theme) D) PopoverRowStyle {
	return PopoverRowStyle{Click: click, Content: content, theme: th}
}

func (r PopoverRowStyle) Layout(gtx C) D {
	th, click := r.theme, r.Click
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		if click.Hovered() {
			bg = th.Accent
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return Fill(gtx, bg, RadiusSM) },
			func(gtx C) D {
				return layout.Inset{Left: Space(2), Right: Space(2), Top: Space(1.5), Bottom: Space(1.5)}.Layout(gtx, func(gtx C) D {
					return r.Content(gtx, th.On(bg))
				})
			},
		)
	})
}

// SelectTriggerStyle is the outline SelectTrigger, Width wide, naming the
// value, with the mark Lead draws before it when set.
type SelectTriggerStyle struct {
	Value string
	Width unit.Dp
	Lead  layout.Widget
	theme *Theme
}

func (th *Theme) SelectTrigger(value string, width unit.Dp) SelectTriggerStyle {
	return SelectTriggerStyle{Value: value, Width: width, theme: th}
}

func (s SelectTriggerStyle) Layout(gtx C) D {
	th := s.theme
	size := image.Pt(gtx.Dp(s.Width), gtx.Dp(Space(9)))
	gtx.Constraints = layout.Exact(size)
	bg := th.Over(Alpha(th.Palette.Input, 0.3))
	Fill(gtx, bg, RadiusMD)
	InsetBorder(gtx, size, gtx.Dp(RadiusMD), th.Over(th.Palette.Input))
	on := th.On(bg)
	return layout.Inset{Left: Space(2.5), Right: Space(2.5)}.Layout(gtx, func(gtx C) D {
		var children []layout.FlexChild
		if s.Lead != nil {
			children = append(children, layout.Rigid(s.Lead), Gap(2))
		}
		children = append(children,
			layout.Flexed(1, on.Text(TextSM, s.Value).Layout),
			Gap(2),
			layout.Rigid(func(gtx C) D { return selectChevron.Layout(gtx, Space(4), on.MutedForeground) }),
		)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

var selectChevron = icons.Lucide("chevron-down")
