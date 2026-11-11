package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/filetree"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

// skeletonWidths are ragged fractions a list's bars take, so it reads as
// rows of different text.
var skeletonWidths = []float32{0.62, 0.48, 0.7, 0.55, 0.4, 0.66, 0.5}

func skeletonWidth(i int) float32 { return skeletonWidths[i%len(skeletonWidths)] }

// The tab skeletons' rows: bounded for an unbounded height, the panel's
// form inset as panelList is, each row padded as panelRow is.
var (
	skeletonRows      = ui.SkeletonFill{MaxRows: listSkeletonMaxRows}
	skeletonPanelRows = ui.SkeletonFill{MaxRows: listSkeletonMaxRows, PadY: ui.Space(1)}
	skeletonRowBox    = ui.SkeletonRow{PadX: pullsRowEdge + pullsRowPadX, PadY: ui.Space(2)}
)

// skeletonTwoLines is a text-sm line over a text-xs one, w of the room.
func skeletonTwoLines(th *ui.Theme, w float32) layout.Widget {
	return func(gtx C) D {
		full := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.SkeletonText(ui.TextSM, full*unit.Dp(w)).Layout),
			layout.Rigid(th.SkeletonText(ui.TextXS, full*unit.Dp(w)*0.6).Layout),
		)
	}
}

// skeletonCommits is commitRows filling the panel.
func skeletonCommits(gtx C, th *ui.Theme) D {
	return skeletonPanelRows.Layout(gtx, func(gtx C, i int) D { return skeletonCommitRow(gtx, th, i) })
}

// skeletonCommitRow is commitRow: a headline over who and when, the short
// oid at the right.
func skeletonCommitRow(gtx C, th *ui.Theme, i int) D {
	return skeletonRowBox.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, ui.Spanning(skeletonTwoLines(th, skeletonWidth(i)))),
			layout.Rigid(th.SkeletonText(ui.TextXS, ui.Space(12.5)).Layout),
		)
	})
}

// skeletonChecks is checkRows filling the panel.
func skeletonChecks(gtx C, th *ui.Theme) D {
	return skeletonPanelRows.Layout(gtx, func(gtx C, i int) D { return skeletonCheckRow(gtx, th, i) })
}

// skeletonCheckRow is a check's row: the state glyph, its name over its
// description, the duration at the right.
func skeletonCheckRow(gtx C, th *ui.Theme, i int) D {
	return skeletonRowBox.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Skeleton(ui.Space(4), ui.Space(4)).Layout),
			ui.Gap(3),
			layout.Flexed(1, ui.Spanning(skeletonTwoLines(th, skeletonWidth(i)))),
			layout.Rigid(th.SkeletonText(ui.TextXS, ui.Space(8)).Layout),
		)
	})
}

// skeletonConversation is talkEntry after talkEntry: the avatar beside the
// byline and a few lines of Markdown.
func skeletonConversation(gtx C, th *ui.Theme) D {
	return skeletonPanelRows.Layout(gtx, func(gtx C, i int) D {
		return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(3), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(th.Skeleton(authorAvatarSize, authorAvatarSize).Layout),
				layout.Rigid(layout.Spacer{Width: authorAvatarGap}.Layout),
				layout.Flexed(1, func(gtx C) D {
					full := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
					children := []layout.FlexChild{
						layout.Rigid(th.SkeletonText(ui.TextSM, ui.Space(40)).Layout),
						ui.VGap(1),
					}
					for j := range 1 + i%3 {
						children = append(children, layout.Rigid(skeletonMarkdownLine(th, full*unit.Dp(skeletonProseLines[j]))))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				}),
			)
		})
	})
}

// skeletonMarkdownLine is a line of a Markdown body w wide.
func skeletonMarkdownLine(th *ui.Theme, w unit.Dp) layout.Widget {
	return func(gtx C) D {
		line := gtx.Metric.PxToDp(int(float32(gtx.Sp(markdown.BodySize)) * markdown.LineHeightScale))
		return th.SkeletonBar(w, line, gtx.Metric.PxToDp(gtx.Sp(markdown.BodySize))).Layout(gtx)
	}
}

// skeletonFiles is filesTab: the toolbar over the tree, when shown, beside
// the file cards, each a header over lines of diff.
func (v *pullView) skeletonFiles(gtx C, th *ui.Theme) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(th.Skeleton(ui.Space(6), ui.Space(6)).Layout),
					ui.Gap(2),
					layout.Rigid(th.SkeletonText(ui.TextXS, ui.Space(22)).Layout),
				)
			})
		}),
		layout.Rigid(th.Separator),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if v.tree.hidden {
						return D{}
					}
					rem := v.tree.widthRem
					if rem == 0 {
						rem = treeDefaultRem
					}
					w := gtx.Dp(unit.Dp(rem * remDp))
					gtx.Constraints = layout.Exact(image.Pt(w, gtx.Constraints.Max.Y))
					return skeletonTree(gtx, th)
				}),
				layout.Rigid(func(gtx C) D {
					if v.tree.hidden {
						return D{}
					}
					return th.VerticalRule(gtx)
				}),
				layout.Flexed(1, func(gtx C) D { return skeletonCards(gtx, th) }),
			)
		}),
	)
}

// skeletonTree is the tree's rows: a folder, then its files indented under it.
func skeletonTree(gtx C, th *ui.Theme) D {
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return skeletonRows.Layout(gtx, func(gtx C, i int) D {
			depth := min(i%4, 2)
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			indent := filetree.IndentBase + filetree.Indent*unit.Dp(depth)
			return layout.Inset{Left: indent, Right: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(th.SkeletonText(ui.TextXS, ui.Space(16+4*float32(i%3))).Layout),
					layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
					layout.Rigid(th.SkeletonText(ui.TextXS, ui.Space(6)).Layout),
				)
			})
		})
	})
}

// skeletonDiffLines is how many lines of diff each skeleton card shows.
const skeletonDiffLines = 6

// skeletonCards is cardItem after cardItem: a separator between cards, each
// a header on Sidebar over its lines of diff.
func skeletonCards(gtx C, th *ui.Theme) D {
	return skeletonRows.Layout(gtx, func(gtx C, i int) D {
		inset := layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(2.5)}
		if i == 0 {
			inset.Top = ui.Space(2)
		}
		return inset.Layout(gtx, func(gtx C) D {
			children := []layout.FlexChild{}
			if i > 0 {
				children = append(children, layout.Rigid(func(gtx C) D { return layout.Inset{Bottom: ui.Space(2.5)}.Layout(gtx, th.Separator) }))
			}
			children = append(children, layout.Rigid(func(gtx C) D { return skeletonCardHeader(gtx, th, i) }))
			full := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
			for j := range skeletonDiffLines {
				w := full * unit.Dp(skeletonProseLines[j%len(skeletonProseLines)]) * 0.7
				children = append(children, layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: ui.Space(2)}.Layout(gtx, th.SkeletonText(ui.TextXS, w).Layout)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

// skeletonCardHeader is fileCard.header: the file's name and its +/- in a
// Sidebar strip as tall as the header's icon buttons make it.
func skeletonCardHeader(gtx C, th *ui.Theme, i int) D {
	on := th.On(th.Sidebar)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Sidebar, 0) },
		func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(layout.Spacer{Height: ui.Space(6)}.Layout),
					layout.Flexed(1, ui.Spanning(on.SkeletonText(ui.TextXS, ui.Space(40+8*float32(i%3))).Layout)),
					layout.Rigid(on.SkeletonText(ui.TextXS, ui.Space(10)).Layout),
				)
			})
		},
	)
}
