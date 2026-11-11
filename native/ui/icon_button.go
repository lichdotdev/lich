package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// IconActionState is an IconAction's clickable and tooltip.
type IconActionState struct {
	Click widget.Clickable
	Tip   Tooltip
}

// IconActionStyle is a bare icon button for a panel's header, its meaning in
// a tooltip, filled with the accent while hovered or pressed.
type IconActionStyle struct {
	Root    *Root
	State   *IconActionState
	Icon    *icons.Icon
	Label   string
	Pressed bool
	theme   *Theme
}

func (th *Theme) IconAction(root *Root, s *IconActionState, ic *icons.Icon, label string, pressed bool) IconActionStyle {
	return IconActionStyle{Root: root, State: s, Icon: ic, Label: label, Pressed: pressed, theme: th}
}

func (a IconActionStyle) Layout(gtx C) D {
	th, s := a.theme, a.State
	tip := th.Tooltip(a.Root, &s.Tip, a.Label)
	return tip.Layout(gtx, func(gtx C) D {
		return s.Click.Layout(gtx, func(gtx C) D {
			side := gtx.Dp(Space(6))
			gtx.Constraints = layout.Exact(image.Pt(side, side))
			fg := th.MutedForeground
			if a.Pressed || s.Click.Hovered() {
				Fill(gtx, th.Accent, RadiusMD)
				fg = th.AccentForeground
			}
			return layout.Center.Layout(gtx, func(gtx C) D { return a.Icon.Layout(gtx, Space(3.5), fg) })
		})
	})
}

// CornerButtonStyle is the shape of both controls in a card's corner. On is
// the set pin: the glyph in the foreground and filled.
type CornerButtonStyle struct {
	Click *widget.Clickable
	Icon  *icons.Icon
	On    bool
	theme *Theme
}

func (th *Theme) CornerButton(click *widget.Clickable, ic *icons.Icon, on bool) CornerButtonStyle {
	return CornerButtonStyle{Click: click, Icon: ic, On: on, theme: th}
}

func (b CornerButtonStyle) Layout(gtx C) D {
	th, click := b.theme, b.Click
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(16), gtx.Dp(16)))
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				if !click.Hovered() {
					return D{Size: gtx.Constraints.Min}
				}
				return Fill(gtx, th.Over(Alpha(th.Foreground, 0.15)), unit.Dp(4))
			},
			func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					if b.On {
						return b.Icon.LayoutFilled(gtx, 12, th.Foreground)
					}
					return b.Icon.Layout(gtx, 12, th.MutedForeground)
				})
			},
		)
	})
}

// BarButtonSize is the side of BarIconButton, the size-7 icon buttons of the
// top strip.
var BarButtonSize = Space(7)

// barDisabledOpacity is the opacity of a disabled bar button.
const barDisabledOpacity = 0.5

// BarIconButtonStyle is a size-7 ghost icon button of the strip: muted,
// accent/50 hovered, accent while its popover is open, faded when disabled.
// A nil Click draws it disabled.
type BarIconButtonStyle struct {
	Click *widget.Clickable
	Icon  *icons.Icon
	On    bool
	theme *Theme
}

func (th *Theme) BarIconButton(click *widget.Clickable, ic *icons.Icon, on bool) BarIconButtonStyle {
	return BarIconButtonStyle{Click: click, Icon: ic, On: on, theme: th}
}

func (b BarIconButtonStyle) Layout(gtx C) D {
	th, click := b.theme, b.Click
	size := gtx.Dp(BarButtonSize)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	face := func(gtx C) D {
		hovered := click != nil && click.Hovered()
		fg := th.MutedForeground
		switch {
		case click == nil:
			fg = th.Over(Alpha(fg, barDisabledOpacity))
		case b.On:
			Fill(gtx, th.Accent, RadiusMD)
			fg = th.Foreground
		case hovered:
			Fill(gtx, th.Over(Alpha(th.Accent, barHoverFill)), RadiusMD)
			fg = th.Foreground
		}
		return layout.Center.Layout(gtx, func(gtx C) D { return b.Icon.Layout(gtx, Space(4), fg) })
	}
	if click == nil {
		return face(gtx)
	}
	return click.Layout(gtx, face)
}

// barHoverFill is hover:bg-accent/50.
const barHoverFill = 0.5
