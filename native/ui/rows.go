package ui

import (
	"gioui.org/font"
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
)

// GhostHoverFill is the opacity of the fill under a hovered ghost row or
// button.
const GhostHoverFill = 0.5

// NavRow is a list row of the given height and side padding, the width it is
// given: Accent while on, a ghost fill under the pointer. content draws on the
// row's own surface.
func (th *Theme) NavRow(gtx C, click *Clickable, on bool, height, padX unit.Dp, content func(gtx C, th *Theme) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		switch {
		case on:
			bg = th.Accent
		case click.Hovered():
			bg = th.Over(Alpha(th.Accent, GhostHoverFill))
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return Fill(gtx, bg, RadiusMD) },
			func(gtx C) D {
				gtx.Constraints.Min.Y = gtx.Dp(height)
				return layout.Inset{Left: padX, Right: padX}.Layout(gtx, func(gtx C) D {
					return layout.W.Layout(gtx, func(gtx C) D { return content(gtx, th.On(bg)) })
				})
			},
		)
	})
}

// PanelRow is a list row of a panel, the width it is given, its content set
// in by edge and padX from the sides and Space(2) high. It takes the ghost
// fill under the pointer only while live, rounded as the sidebar's rows.
// content draws on the row's own surface.
func (th *Theme) PanelRow(gtx C, click *Clickable, live bool, edge, padX unit.Dp, content func(gtx C, th *Theme) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		if live && click.Hovered() {
			bg = th.Over(Alpha(th.Accent, GhostHoverFill))
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return Fill(gtx, bg, RadiusMD) },
			func(gtx C) D {
				return layout.Inset{Left: edge + padX, Right: edge + padX, Top: Space(2), Bottom: Space(2)}.Layout(gtx, func(gtx C) D {
					return content(gtx, th.On(bg))
				})
			})
	})
}

// FillRow fills size with the accent at alpha, rounded as a row.
func (th *Theme) FillRow(gtx C, size image.Point, alpha float32) {
	fill := gtx
	fill.Constraints = layout.Exact(size)
	Fill(fill, th.Over(Alpha(th.Accent, alpha)), RadiusMD)
}

// MenuHeader is a menu's title in text-xs semibold with its count, muted, at
// the end.
func (th *Theme) MenuHeader(gtx C, title, count string) D {
	return layout.Inset{Left: Space(2), Right: Space(2), Top: Space(1.5), Bottom: Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, th.Text(TextXS, title).Weight(font.SemiBold).Layout),
			layout.Rigid(th.Text(TextXS, count).Muted().Layout),
		)
	})
}

// MenuSeparator is the space between a menu's groups, where a rule stood:
// they are set apart by space.
func (th *Theme) MenuSeparator(gtx C) D {
	return layout.Spacer{Height: Space(2)}.Layout(gtx)
}

// RailHairline is the short rule that splits two blocks of the collapsed
// sidebar.
func (th *Theme) RailHairline(gtx C) D {
	return layout.Inset{Top: Space(2), Bottom: Space(1)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(Space(5)), gtx.Dp(1)))
		return Fill(gtx, th.Over(th.Border), 0)
	})
}

// CaptionDivider is the room between the groups of a title bar, inset on
// both sides of where a 1px rule stood: they are set apart by space.
func (th *Theme) CaptionDivider(gtx C, inset, height unit.Dp) D {
	return layout.Inset{Left: inset, Right: inset}.Layout(gtx, func(gtx C) D {
		return D{Size: image.Pt(gtx.Dp(1), gtx.Dp(height))}
	})
}

// OrRule is word in muted text-xs between two rules that split a form's two
// ways of doing one thing.
func (th *Theme) OrRule(gtx C, word string) D {
	line := func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.Y = gtx.Dp(1)
			return th.RuleAbove(gtx)
		})
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, line),
		Gap(3),
		layout.Rigid(th.Text(TextXS, word).Muted().Layout),
		Gap(3),
		layout.Flexed(1, line),
	)
}
