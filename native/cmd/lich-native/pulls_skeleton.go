package main

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

// The line heights the list's and rows' bars sit in, so each bar takes the
// room of the text it stands for: text-sm and text-xs.
var (
	skeletonSMLine = ui.Space(5)
	skeletonXSLine = ui.Space(4)
)

// detailSkeleton mirrors body while the pull request loads, skeletonHeading
// standing in for its title row: the tab strip, its labels known before the
// pull request is, and the open tab's content, each bar where its text will
// land.
func (v *pullView) detailSkeleton(th *ui.Theme) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: pullsToolbarPad, Right: pullsToolbarPad}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						ui.VGap(4),
						layout.Rigid(func(gtx C) D { return v.skeletonTabs(gtx, th) }),
					)
				})
			}),
			layout.Rigid(th.Separator),
			layout.Flexed(1, func(gtx C) D {
				return v.skeletonTab(gtx, th)
			}),
		)
	}
}

// skeletonHeading is titleRow: heading's title over the stats line, beside
// the actions Open in Session, Merge and ⋯ as blocks of their size.
func skeletonHeading(gtx C, th *ui.Theme) D {
	var session, merge, more widget.Clickable
	actions := func(gtx C) D {
		sessionB := th.Button(&session, i18n.T("pulls.pulls.openInSession"))
		sessionB.Icon, sessionB.Size = icPullSession, ui.SizeSM
		mergeB := th.Button(&merge, i18n.T("pulls.pullRequestView.merge"))
		mergeB.Icon, mergeB.Size = icPullMerged, ui.SizeSM
		moreB := th.Button(&more, "")
		moreB.Icon, moreB.Size = icPullMore, ui.SizeIconSM
		return ui.Wrap{GapX: ui.Space(2), Justify: ui.JustifyEnd}.Layout(gtx,
			ui.WrapItem(th.SkeletonShape(sessionB.Layout).Layout),
			ui.WrapItem(th.SkeletonShape(mergeB.Layout).Layout),
			ui.WrapItem(th.SkeletonShape(moreB.Layout).Layout),
		)
	}
	heading := func(gtx C) D {
		return layout.Inset{Right: ui.Space(4), Top: pullsHeadingTop}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					// The spacer is the title's pencil, which sets the line when taller.
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(th.SkeletonText(pullsTitleSize, ui.Space(120)).Layout),
						ui.Gap(1),
						layout.Rigid(layout.Spacer{Width: ui.Space(6), Height: ui.Space(6)}.Layout),
					)
				}),
				ui.VGap(3),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(th.SkeletonText(ui.TextXS, ui.Space(14)).Layout),
						ui.Gap(4),
						layout.Rigid(th.SkeletonText(ui.TextXS, ui.Space(48)).Layout),
						ui.Gap(4),
						layout.Rigid(th.SkeletonText(ui.TextXS, ui.Space(24)).Layout),
					)
				}),
			)
		})
	}
	return layout.Flex{}.Layout(gtx, layout.Flexed(1, ui.Spanning(heading)), layout.Rigid(actions))
}

// skeletonTabOrder is every tab a pull request can show, as tabsOf lists them.
var skeletonTabOrder = []pullsTab{tabOverview, tabConversation, tabCommits, tabFiles, tabChecks}

// skeletonTabs is tabStrip with every tab's label, a bar where a count
// lands, the open tab marked, and a bar where the diff's size lands.
func (v *pullView) skeletonTabs(gtx C, th *ui.Theme) D {
	var triggers []ui.TabsTrigger
	for _, t := range skeletonTabOrder {
		trigger := ui.TabsTrigger{Text: t.label()}
		if t != tabOverview {
			trigger.Trailing = th.SkeletonText(ui.TextSM, ui.Space(2.5)).Layout
		}
		triggers = append(triggers, trigger)
	}
	tabs := th.Tabs(&v.tabs, triggers...)
	tabs.Variant = ui.TabsVariantLine
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			d := tabs.Layout(gtx)
			d.Size.X = gtx.Constraints.Max.X
			return d
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(4)}.Layout(gtx, th.SkeletonText(ui.TextXS, ui.Space(16)).Layout)
		}),
	)
}

// skeletonOverview is overview: the description's fixed heading over its
// prose filling the column, beside the facts, stacked the same way on an
// island too narrow for both.
func skeletonOverview(gtx C, th *ui.Theme) D {
	var more widget.Clickable
	heading := func(gtx C) D {
		return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(4)}.Layout(gtx, func(gtx C) D {
			moreB := th.Button(&more, "")
			moreB.Icon, moreB.Size = icPullMore, ui.SizeIconXS
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, ui.Spanning(th.SkeletonText(ui.TextXS, ui.Space(24)).Layout)),
				layout.Rigid(th.SkeletonShape(moreB.Layout).Layout),
			)
		})
	}
	prose := func(gtx C) D {
		size := gtx.Constraints.Max
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(overviewProseWidth+2*pullsPadX))
		layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(3)}.Layout(gtx, func(gtx C) D { return skeletonProse(gtx, th) })
		return D{Size: size}
	}
	if gtx.Constraints.Max.X < gtx.Dp(overviewSplitWidth) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(5)}.Layout(gtx, func(gtx C) D { return skeletonFacts(gtx, th) })
			}),
			layout.Rigid(heading),
			layout.Flexed(1, prose),
		)
	}
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: ui.Space(1), Bottom: overviewHeadingBottom}.Layout(gtx, heading)
				}),
				layout.Rigid(th.Separator),
				layout.Flexed(1, prose),
			)
		}),
		layout.Rigid(func(gtx C) D { return th.VerticalRule(gtx) }),
		layout.Rigid(func(gtx C) D {
			w := gtx.Dp(overviewFactsWidth)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
			return layout.Inset{Left: ui.Space(5), Right: ui.Space(5), Top: ui.Space(5)}.Layout(gtx, func(gtx C) D { return skeletonFacts(gtx, th) })
		}),
	)
}

// skeletonProseLines are a paragraph's ragged lines, as fractions of the
// reading width.
var skeletonProseLines = []float32{1, 0.96, 0.98, 0.93, 0.6}

// skeletonProse is the description's Markdown as paragraphs of body lines,
// a block gap apart, down to the bottom of the column.
func skeletonProse(gtx C, th *ui.Theme) D {
	full := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
	line := unit.Dp(gtx.Metric.PxToDp(int(float32(gtx.Sp(markdown.BodySize)) * markdown.LineHeightScale)))
	bar := unit.Dp(gtx.Metric.PxToDp(gtx.Sp(markdown.BodySize)))
	var children []layout.FlexChild
	y, gap := 0, gtx.Dp(ui.Space(markdown.BlockGap))
	for y < gtx.Constraints.Max.Y {
		if len(children) > 0 {
			children = append(children, ui.VGap(markdown.BlockGap))
			y += gap
		}
		for _, w := range skeletonProseLines {
			children = append(children, layout.Rigid(th.SkeletonBar(full*unit.Dp(w), line, bar).Layout))
			y += gtx.Dp(line)
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// skeletonFacts is facts' sections, a heading over its value each.
func skeletonFacts(gtx C, th *ui.Theme) D {
	var children []layout.FlexChild
	for i, w := range []unit.Dp{ui.Space(32), ui.Space(40), ui.Space(28)} {
		if i > 0 {
			children = append(children, ui.VGap(5))
		}
		children = append(children,
			layout.Rigid(th.SkeletonBar(ui.Space(18), skeletonXSLine, ui.Space(3)).Layout),
			ui.VGap(2),
			layout.Rigid(th.SkeletonBar(w, skeletonSMLine, ui.Space(3.5)).Layout),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// skeletonTab is the open tab's content while the pull request loads.
func (v *pullView) skeletonTab(gtx C, th *ui.Theme) D {
	tab := tabOverview
	if i := v.tabs.Selected; i >= 0 && i < len(skeletonTabOrder) {
		tab = skeletonTabOrder[i]
	}
	switch tab {
	case tabConversation:
		return skeletonConversation(gtx, th)
	case tabCommits:
		return skeletonCommits(gtx, th)
	case tabFiles:
		return v.skeletonFiles(gtx, th)
	case tabChecks:
		return skeletonChecks(gtx, th)
	}
	return skeletonOverview(gtx, th)
}
