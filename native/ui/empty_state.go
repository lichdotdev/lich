package ui

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui/icons"
)

const (
	emptyStateTitleSize    unit.Sp = 18
	emptyStateTitleLeading unit.Sp = 28
)

// EmptyStateStyle is what a screen or panel with nothing to show draws
// instead: a glyph, a title, a line of description and the actions out.
type EmptyStateStyle struct {
	// Icon is nil for no glyph, its gap gone with it.
	Icon *icons.Icon
	// Description is "" for no line.
	Title, Description string
	// Actions is nil for no action row.
	Actions layout.Widget

	IconSize unit.Dp
	// IconGap, DescriptionGap and ActionsGap are Space units.
	IconGap, DescriptionGap, ActionsGap float32
	TitleSize, DescriptionSize          unit.Sp
	// TitleLeading is 0 to keep the size's Tailwind line height.
	TitleLeading unit.Sp
	TitleWeight  font.Weight
	// MaxLines holds the title and description; 0 wraps them.
	MaxLines int
	// Anchor and Align place the block and its lines in the area; Pad insets
	// the block on every side.
	Anchor layout.Direction
	Align  layout.Alignment
	Pad    unit.Dp
	theme  *Theme
}

// EmptyState is a full surface's empty screen, centred.
func (th *Theme) EmptyState(icon *icons.Icon, title, description string, actions layout.Widget) EmptyStateStyle {
	return EmptyStateStyle{
		Icon: icon, Title: title, Description: description, Actions: actions,
		IconSize: Space(12), IconGap: 4, ActionsGap: 4,
		TitleSize: emptyStateTitleSize, TitleLeading: emptyStateTitleLeading, TitleWeight: font.SemiBold,
		DescriptionSize: TextSM, MaxLines: 1,
		Anchor: layout.Center, Align: layout.Middle, theme: th,
	}
}

// InlineEmptyState is the compact form, at the top of a card: text that wraps
// and no glyph.
func (th *Theme) InlineEmptyState(title, description string, actions layout.Widget) EmptyStateStyle {
	return EmptyStateStyle{
		Title: title, Description: description, Actions: actions,
		DescriptionGap: 1, ActionsGap: 3,
		TitleSize: TextSM, TitleWeight: font.Medium,
		DescriptionSize: TextXS,
		Anchor:          layout.NW, Pad: Space(4), theme: th,
	}
}

func (e EmptyStateStyle) Layout(gtx C) D {
	return e.Anchor.Layout(gtx, func(gtx C) D {
		return layout.UniformInset(e.Pad).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: e.Align}.Layout(gtx, e.children()...)
		})
	})
}

func (e EmptyStateStyle) children() []layout.FlexChild {
	th := e.theme
	var kids []layout.FlexChild
	if e.Icon != nil {
		kids = append(kids,
			layout.Rigid(func(gtx C) D { return e.Icon.Layout(gtx, e.IconSize, th.MutedForeground) }),
			VGap(e.IconGap))
	}
	title := th.Text(e.TitleSize, e.Title).Weight(e.TitleWeight)
	title.MaxLines = e.MaxLines
	if e.TitleLeading != 0 {
		title = title.Leading(e.TitleLeading)
	}
	kids = append(kids, layout.Rigid(title.Layout))
	if e.Description != "" {
		desc := th.Text(e.DescriptionSize, e.Description).Muted()
		desc.MaxLines = e.MaxLines
		kids = append(kids, layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: Space(e.DescriptionGap)}.Layout(gtx, desc.Layout)
		}))
	}
	if e.Actions != nil {
		kids = append(kids, VGap(e.ActionsGap), layout.Rigid(e.Actions))
	}
	return kids
}
