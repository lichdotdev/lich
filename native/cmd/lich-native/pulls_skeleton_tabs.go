package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// skeletonWidths are ragged fractions a list's bars take, so it reads as
// rows of different text.
var skeletonWidths = []float32{0.62, 0.48, 0.7, 0.55, 0.4, 0.66, 0.5}

func skeletonWidth(i int) float32 { return skeletonWidths[i%len(skeletonWidths)] }

// skeletonFill lays row after row down to the bottom of gtx, as a loaded
// list fills its panel.
func skeletonFill(gtx C, row func(gtx C, i int) D) D {
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	y := 0
	for i := 0; y < gtx.Constraints.Max.Y && i < listSkeletonMaxRows; i++ {
		g := gtx
		g.Constraints.Min.Y = 0
		stack := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		y += row(g, i).Size.Y
		stack.Pop()
	}
	return D{Size: image.Pt(gtx.Constraints.Max.X, min(y, gtx.Constraints.Max.Y))}
}

// skeletonPanel is panelList's inset around rows filling it.
func skeletonPanel(gtx C, row func(gtx C, i int) D) D {
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D { return skeletonFill(gtx, row) })
}

// skeletonPanelRow is panelRow's insets around content.
func skeletonPanelRow(gtx C, content layout.Widget) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	pad := pullsRowEdge + pullsRowPadX
	return layout.Inset{Left: pad, Right: pad, Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, content)
}

// skeletonTwoLines is a text-sm line over a text-xs one, w of the room.
func skeletonTwoLines(th *ui.Theme, w float32) layout.Widget {
	return func(gtx C) D {
		full := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(skeletonText(th, ui.TextSM, full*unit.Dp(w))),
			layout.Rigid(skeletonText(th, ui.TextXS, full*unit.Dp(w)*0.6)),
		)
	}
}

// skeletonCommits is commitRows filling the panel.
func skeletonCommits(gtx C, th *ui.Theme) D {
	return skeletonPanel(gtx, func(gtx C, i int) D { return skeletonCommitRow(gtx, th, i) })
}

// skeletonCommitRow is commitRow: a headline over who and when, the short
// oid at the right.
func skeletonCommitRow(gtx C, th *ui.Theme, i int) D {
	return skeletonPanelRow(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, spanning(skeletonTwoLines(th, skeletonWidth(i)))),
			layout.Rigid(skeletonText(th, ui.TextXS, ui.Space(12.5))),
		)
	})
}

// skeletonChecks is checkRows filling the panel.
func skeletonChecks(gtx C, th *ui.Theme) D {
	return skeletonPanel(gtx, func(gtx C, i int) D { return skeletonCheckRow(gtx, th, i) })
}

// skeletonCheckRow is a check's row: the state glyph, its name over its
// description, the duration at the right.
func skeletonCheckRow(gtx C, th *ui.Theme, i int) D {
	return skeletonPanelRow(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Skeleton(ui.Space(4), ui.Space(4)).Layout),
			ui.Gap(3),
			layout.Flexed(1, spanning(skeletonTwoLines(th, skeletonWidth(i)))),
			layout.Rigid(skeletonText(th, ui.TextXS, ui.Space(8))),
		)
	})
}

// skeletonConversation is talkEntry after talkEntry: the avatar beside the
// byline and a few lines of Markdown.
func skeletonConversation(gtx C, th *ui.Theme) D {
	return skeletonPanel(gtx, func(gtx C, i int) D {
		return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(3), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(th.Skeleton(authorAvatarSize, authorAvatarSize).Layout),
				layout.Rigid(layout.Spacer{Width: authorAvatarGap}.Layout),
				layout.Flexed(1, func(gtx C) D {
					full := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
					children := []layout.FlexChild{
						layout.Rigid(skeletonText(th, ui.TextSM, ui.Space(40))),
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
		line := gtx.Metric.PxToDp(int(float32(gtx.Sp(markdownBodySize)) * markdownLineHeightScale))
		return skeletonBar(th, w, line, gtx.Metric.PxToDp(gtx.Sp(markdownBodySize)))(gtx)
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
					layout.Rigid(skeletonText(th, ui.TextXS, ui.Space(22))),
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
					return verticalRule(gtx, th)
				}),
				layout.Flexed(1, func(gtx C) D { return skeletonCards(gtx, th) }),
			)
		}),
	)
}

// skeletonTree is the tree's rows: a folder, then its files indented under it.
func skeletonTree(gtx C, th *ui.Theme) D {
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return skeletonFill(gtx, func(gtx C, i int) D {
			depth := min(i%4, 2)
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			indent := treeIndentBase + treeIndent*unit.Dp(depth)
			return layout.Inset{Left: indent, Right: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(skeletonText(th, ui.TextXS, ui.Space(16+4*float32(i%3)))),
					layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
					layout.Rigid(skeletonText(th, ui.TextXS, ui.Space(6))),
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
	return skeletonFill(gtx, func(gtx C, i int) D {
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
					return layout.Inset{Left: ui.Space(2)}.Layout(gtx, skeletonText(th, ui.TextXS, w))
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
					layout.Flexed(1, spanning(skeletonText(on, ui.TextXS, ui.Space(40+8*float32(i%3))))),
					layout.Rigid(skeletonText(on, ui.TextXS, ui.Space(10))),
				)
			})
		},
	)
}
