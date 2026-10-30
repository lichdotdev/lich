package main

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// The line heights the skeleton's bars sit in, so each bar takes the room of
// the text it stands for: text-lg, text-sm and text-xs, and the line variant's
// tab list.
var (
	skeletonTitleLine = ui.Space(7)
	skeletonSMLine    = ui.Space(5)
	skeletonXSLine    = ui.Space(4)
	skeletonTabsLine  = ui.Space(9)
)

// skeletonTabWidths are the tab strip's triggers, Overview to Files.
var skeletonTabWidths = []unit.Dp{ui.Space(18), ui.Space(24), ui.Space(18), ui.Space(14)}

// detailSkeleton mirrors body while the pull request loads, skeletonHeading
// standing in for its title row: the tab strip and the open tab's content,
// each bar where its text will land.
func (v *pullView) detailSkeleton(th *ui.Theme) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: pullsToolbarPad, Right: pullsToolbarPad, Top: ui.Space(4)}.Layout(gtx, func(gtx C) D {
					return skeletonTabs(gtx, th)
				})
			}),
			layout.Rigid(th.Separator),
			layout.Flexed(1, func(gtx C) D {
				if v.tabs.Selected > 0 {
					return skeletonRows(gtx, th)
				}
				return skeletonOverview(gtx, th)
			}),
		)
	}
}

// skeletonBar is a bar of width by height centred in a line of height line.
func skeletonBar(th *ui.Theme, width, line, height unit.Dp) layout.Widget {
	pad := (line - height) / 2
	return func(gtx C) D {
		return layout.Inset{Top: pad, Bottom: pad}.Layout(gtx, th.Skeleton(width, height).Layout)
	}
}

// skeletonHeading is heading's title over the stats line: state, branches
// and author.
func skeletonHeading(gtx C, th *ui.Theme) D {
	return layout.Inset{Top: pullsHeadingTop}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(skeletonBar(th, ui.Space(120), skeletonTitleLine, ui.Space(5))),
			ui.VGap(3),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(skeletonBar(th, ui.Space(14), skeletonXSLine, ui.Space(3))),
					ui.Gap(4),
					layout.Rigid(skeletonBar(th, ui.Space(48), skeletonXSLine, ui.Space(3))),
					ui.Gap(4),
					layout.Rigid(skeletonBar(th, ui.Space(24), skeletonXSLine, ui.Space(3))),
				)
			}),
		)
	})
}

// skeletonTabs is the line tab strip, a bar where each trigger's label sits.
func skeletonTabs(gtx C, th *ui.Theme) D {
	var children []layout.FlexChild
	for _, w := range skeletonTabWidths {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(3)}.Layout(gtx, skeletonBar(th, w, skeletonTabsLine, ui.Space(3.5)))
		}))
	}
	return layout.Flex{}.Layout(gtx, children...)
}

// skeletonOverview is overview's description beside its facts, stacked the
// same way on an island too narrow for both.
func skeletonOverview(gtx C, th *ui.Theme) D {
	prose := func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(overviewProseWidth+2*pullsPadX))
		return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(5)}.Layout(gtx, func(gtx C) D {
			full := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
			children := []layout.FlexChild{layout.Rigid(skeletonBar(th, ui.Space(24), skeletonXSLine, ui.Space(3))), ui.VGap(2)}
			for _, w := range []float32{1, 0.94, 0.97, 0.6} {
				children = append(children, layout.Rigid(skeletonBar(th, full*unit.Dp(w), skeletonSMLine, ui.Space(3.5))))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	}
	if gtx.Constraints.Max.X < gtx.Dp(overviewSplitWidth) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(5)}.Layout(gtx, func(gtx C) D { return skeletonFacts(gtx, th) })
			}),
			layout.Rigid(prose),
		)
	}
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, prose),
		layout.Rigid(func(gtx C) D { return verticalRule(gtx, th) }),
		layout.Rigid(func(gtx C) D {
			w := gtx.Dp(overviewFactsWidth)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
			return layout.Inset{Left: ui.Space(5), Right: ui.Space(5), Top: ui.Space(5)}.Layout(gtx, func(gtx C) D { return skeletonFacts(gtx, th) })
		}),
	)
}

// skeletonFacts is facts' sections, a heading over its value each.
func skeletonFacts(gtx C, th *ui.Theme) D {
	var children []layout.FlexChild
	for i, w := range []unit.Dp{ui.Space(32), ui.Space(40), ui.Space(28)} {
		if i > 0 {
			children = append(children, ui.VGap(5))
		}
		children = append(children,
			layout.Rigid(skeletonBar(th, ui.Space(18), skeletonXSLine, ui.Space(3))),
			ui.VGap(2),
			layout.Rigid(skeletonBar(th, w, skeletonSMLine, ui.Space(3.5))),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// skeletonRows is panelList's rows as Conversation, Commits and Checks draw
// them: a glyph beside a line over a dimmer one.
func skeletonRows(gtx C, th *ui.Theme) D {
	widths := []float32{0.3, 0.42, 0.25, 0.36, 0.3}
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		var rows []layout.FlexChild
		for _, w := range widths {
			rows = append(rows, layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
					full := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(th.Skeleton(ui.Space(4), ui.Space(4)).Layout),
						ui.Gap(3),
						layout.Rigid(func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(skeletonBar(th, full*unit.Dp(w), skeletonSMLine, ui.Space(3.5))),
								layout.Rigid(skeletonBar(th, full*unit.Dp(w)*0.8, skeletonXSLine, ui.Space(3))),
							)
						}),
					)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}
