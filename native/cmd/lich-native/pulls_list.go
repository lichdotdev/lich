package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

func (s pullsState) label() string {
	switch s {
	case pullsClosed:
		return i18n.T("pulls.pullsStats.closed")
	case pullsMerged:
		return i18n.T("pulls.pullsStats.merged")
	}
	return i18n.T("pulls.pullsStats.open")
}

func (s pullsState) none() string {
	switch s {
	case pullsClosed:
		return i18n.T("pulls.pullsList.none.closed")
	case pullsMerged:
		return i18n.T("pulls.pullsList.none.merged")
	}
	return i18n.T("pulls.pullsList.none.open")
}

// indexLayout is the index: the heading with the field, the state menu and
// New, the counted quick filters, then a row per pull request.
func (p *pullsScreen) indexLayout(gtx C, th *ui.Theme, list pullsRead[[]backend.PRSummary], act *pullsAction) D {
	if p.create.Clicked(gtx) {
		act.create = true
	}
	counts := filterCounts(list.val, p.query.Text())
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return p.indexHeader(gtx, th) }),
		layout.Rigid(func(gtx C) D {
			tabs := p.filterTabs(th, counts)
			return layout.Inset{Left: pullsPadX, Right: pullsPadX, Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D { return tabs.Layout(gtx) })
		}),
		layout.Rigid(th.Separator),
		layout.Flexed(1, func(gtx C) D {
			// Read after the filter tabs took their click.
			return p.rowList(gtx, th, list, p.visible(list.val))
		}),
	)
}

// indexSearchWidth is the index's filter field, wide enough for its hint.
var indexSearchWidth = unit.Dp(320)

// indexHeader is the index's title, then at the right the field, the state
// menu and New.
func (p *pullsScreen) indexHeader(gtx C, th *ui.Theme) D {
	field := th.Input(&p.query, i18n.T("pulls.pullsList.searchHint"))
	field.Leading, field.Height, field.TextSize, field.Clear = icPullSearch, ui.Space(8), ui.TextXS, &p.clearQuery
	return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(4), Bottom: ui.Space(3)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(pullsTitleSize, i18n.T("tabs.projectTabs.pullRequests")).Weight(font.SemiBold).Layout),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				w := min(gtx.Dp(indexSearchWidth), gtx.Constraints.Max.X)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
				return field.Layout(gtx)
			}),
			ui.Gap(3),
			layout.Rigid(func(gtx C) D { return p.stateRow(gtx, th) }),
			ui.Gap(3),
			layout.Rigid(func(gtx C) D {
				b := th.Button(&p.create, i18n.T("pulls.pullsNav.create"))
				b.Icon, b.Variant, b.Size = icPullOut, ui.VariantOutline, ui.SizeSM
				return b.Layout(gtx)
			}),
		)
	})
}

func (p *pullsScreen) filterTabs(th *ui.Theme, counts [pullsFilterCount]int) ui.TabsStyle {
	triggers := make([]ui.TabsTrigger, pullsFilterCount)
	for i := range triggers {
		n := counts[i]
		count := filterCountColor(th, pullsFilter(i), n)
		triggers[i] = ui.TabsTrigger{Text: pullsFilter(i).label(), Trailing: func(gtx C) D {
			return th.Text(ui.TextXS, fmt.Sprint(n)).In(count).Layout(gtx)
		}}
	}
	tabs := th.Tabs(&p.filter, triggers...)
	tabs.Variant = ui.TabsVariantGhost
	return tabs
}

// emptyFilterOpacity fades a filter with nothing in it, the chip's opacity-40.
const emptyFilterOpacity = 0.4

// filterCountColor reads a filter's state off its count: a failing pull
// request in the destructive color, an empty filter faded.
func filterCountColor(th *ui.Theme, f pullsFilter, n int) color.NRGBA {
	switch {
	case n == 0:
		return ui.Alpha(th.MutedForeground, emptyFilterOpacity)
	case f == filterFailing:
		return th.Destructive
	}
	return th.MutedForeground
}

// stateRow is the uppercase state beside the field, a menu of the states gh
// is asked for.
func (p *pullsScreen) stateRow(gtx C, th *ui.Theme) D {
	entries := make([]ui.MenuEntry, len(pullsStates))
	for i, s := range pullsStates {
		entries[i] = ui.MenuEntry{Kind: ui.MenuRadioItem, Label: s.label(), Checked: s == p.state, OnClick: func() { p.state = s }}
	}
	menu := th.DropdownMenu(p.root, &p.stateMenu, entries)
	return menu.Layout(gtx, func(gtx C) D {
		return p.stateClick.Layout(gtx, func(gtx C) D {
			fg := th.MutedForeground
			if p.stateClick.Hovered() {
				fg = th.Foreground
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, strings.ToUpper(p.state.label())).In(fg).Layout),
				ui.Gap(1),
				layout.Rigid(func(gtx C) D { return icPullDown.Layout(gtx, ui.Space(3), fg) }),
			)
		})
	})
}

func (p *pullsScreen) rowList(gtx C, th *ui.Theme, list pullsRead[[]backend.PRSummary], rows []backend.PRSummary) D {
	switch {
	case !list.loaded:
		return listSkeleton(gtx, th)
	case list.err != "":
		return notice(gtx, th, list.err)
	case len(list.val) == 0:
		return notice(gtx, th, p.state.none())
	case len(rows) == 0:
		return notice(gtx, th, i18n.T("pulls.pullsList.noMatch"))
	}
	now := gtx.Now
	if p.cursorAt(rows) < 0 {
		p.cursor = rows[0].Number
	}
	return p.rowScroll(gtx, th).Layout(gtx, len(rows), func(gtx C, i int) D { return p.row(gtx, th, rows[i], now) })
}

// cursorAt is the cursor's row in rows, -1 when it is not among them.
func (p *pullsScreen) cursorAt(rows []backend.PRSummary) int {
	return position(rows, p.cursor)
}

// rowScroll is the sidebar's scrollColumn over the rows: the scrollbar takes
// room only once they overflow.
func (p *pullsScreen) rowScroll(gtx C, th *ui.Theme) material.ListStyle {
	list := th.ScrollArea(&p.rows)
	list.Indicator.MinorWidth = ui.Space(2)
	list.Indicator.CornerRadius = ui.Space(1)
	list.Track.MinorPadding = 0
	list.AnchorStrategy = material.Overlay
	return list
}

// The index row's columns at its right: the checks, the verdict and the
// checkout, each its own width so they line up down the index.
var (
	rowChecksWidth   = unit.Dp(88)
	rowVerdictWidth  = unit.Dp(150)
	rowCheckoutWidth = unit.Dp(150)
)

// row is one pull request across the index: "#N title" over who, when and
// its branch, then its checks, verdict and checkout in columns. The cursor's
// row is filled once the keyboard moved it.
func (p *pullsScreen) row(gtx C, th *ui.Theme, pr backend.PRSummary, now time.Time) D {
	click := p.rowClicks[pr.Number]
	if click == nil {
		click = new(widget.Clickable)
		p.rowClicks[pr.Number] = click
	}
	checkout, _ := p.checkedOut(pr.HeadRefName)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		switch {
		case pr.Number == p.cursor && p.keyed:
			bg = th.Accent
		case click.Hovered():
			bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Background{}.Layout(gtx,
					func(gtx C) D { return ui.Fill(gtx, bg, 0) },
					func(gtx C) D {
						on := th.On(bg)
						return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(2.5), Bottom: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D { return rowTitle(gtx, on, pr) }),
										ui.VGap(1),
										layout.Rigid(func(gtx C) D { return rowMeta(gtx, on, pr, now) }),
									)
								}),
								ui.Gap(4),
								layout.Rigid(func(gtx C) D { return rowSide(gtx, on, pr, checkoutName(checkout)) }),
							)
						})
					})
			}),
			layout.Rigid(th.Separator),
		)
	})
}

func rowTitle(gtx C, th *ui.Theme, pr backend.PRSummary) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextSM, fmt.Sprintf("#%d", pr.Number)).Mono().Muted().Layout),
		ui.Gap(1.5),
		layout.Flexed(1, func(gtx C) D {
			l := th.Text(ui.TextSM, pr.Title).Weight(font.Medium)
			l.MaxLines = 1
			return l.Layout(gtx)
		}),
	)
}

// cell lays w in a column w wide, against its right edge.
func cell(gtx C, width unit.Dp, w layout.Widget) D {
	px := gtx.Dp(width)
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
	return layout.E.Layout(gtx, w)
}

// rowSide is the row's columns: the checks, the verdict worth acting on and
// the checkout the branch sits in, which shows only where it fits whole.
func rowSide(gtx C, th *ui.Theme, pr backend.PRSummary, checkout string) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return cell(gtx, rowChecksWidth, func(gtx C) D {
				if verdictOf(pr.Checks) == verdictNone {
					return D{}
				}
				return checksChip(gtx, th, pr.Checks)
			})
		}),
		layout.Rigid(func(gtx C) D { return cell(gtx, rowVerdictWidth, func(gtx C) D { return rowVerdict(gtx, th, pr) }) }),
		layout.Rigid(func(gtx C) D {
			return cell(gtx, rowCheckoutWidth, func(gtx C) D {
				if checkout == "" {
					return D{}
				}
				return wholeOrNothing(gtx, func(gtx C) D { return metaGlyph(gtx, th, icCheckout, checkout, th.MutedForeground) })
			})
		}),
	)
}

// rowVerdict is the review verdict worth acting on; REVIEW_REQUIRED, most
// rows' resting state, shows nothing.
func rowVerdict(gtx C, th *ui.Theme, pr backend.PRSummary) D {
	switch {
	case pr.ReviewDecision == "APPROVED":
		return th.Text(ui.TextXS, i18n.T("pulls.pullsStats.approved")).In(th.TonePass).Layout(gtx)
	case waitsOnReviewer(pr):
		return metaGlyph(gtx, th, icPullCheck2, i18n.T("pulls.pullsStats.changesRequested"), th.ToneWait)
	case pr.ReviewDecision == "CHANGES_REQUESTED":
		return th.Text(ui.TextXS, i18n.T("pulls.pullsStats.changesRequested")).In(th.Destructive).Layout(gtx)
	}
	return D{}
}

// checkoutName is the folder a PR's branch is checked out in, the name its
// block in the sidebar carries; "" when it is checked out nowhere.
func checkoutName(w backend.Worktree) string {
	if w.Path == "" {
		return ""
	}
	return filepath.Base(w.Path)
}

// rowMeta is the row's second line: who, when, the branch, and whether it is
// a draft or comes from a fork. The author truncates first.
func rowMeta(gtx C, th *ui.Theme, pr backend.PRSummary, now time.Time) D {
	facts := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = 0
			return th.Text(ui.TextXS, pr.Author).Muted().Layout(gtx)
		}),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextXS, updatedAgo(pr.UpdatedAt, now)).Mono().Muted().Layout),
	}
	add := func(w layout.Widget) { facts = append(facts, ui.Gap(2), layout.Rigid(w)) }
	if pr.HeadRefName != "" {
		add(th.Text(ui.TextXS, pr.HeadRefName).Mono().Muted().Layout)
	}
	if pr.IsDraft {
		add(func(gtx C) D { return draftBadge(gtx, th) })
	}
	if pr.IsCrossRepository {
		add(th.Text(ui.TextXS, i18n.T("pulls.pullsList.fork")).Muted().Layout)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, facts...)
}

// draftBorderAlpha keeps the badge's outline visible on a selected row,
// where the Border token sits on the Accent fill at nearly its own shade.
const draftBorderAlpha = 0.5

// draftBadge is the outlined, muted "Draft": the pull request's state, set
// apart from the amber the verdict beside it can take.
func draftBadge(gtx C, th *ui.Theme) D {
	gtx.Constraints.Min = image.Point{}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			rr := clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(ui.RadiusSM))
			border := th.Over(ui.Alpha(th.MutedForeground, draftBorderAlpha))
			paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
			return D{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx C) D {
			return layout.Inset{Left: ui.Space(1), Right: ui.Space(1)}.Layout(gtx,
				th.Text(ui.TextXS, i18n.T("pulls.pullsList.draft")).Muted().Layout)
		}),
	)
}

// waitsOnReviewer is a CHANGES_REQUESTED verdict whose every thread is
// resolved: GitHub keeps the verdict until the reviewer moves.
func waitsOnReviewer(pr backend.PRSummary) bool {
	return pr.ReviewDecision == "CHANGES_REQUESTED" && pr.Threads.Total > 0 && pr.Threads.Open == 0
}

// wholeOrNothing draws w only when its natural width fits: a glyph cut in
// half reads as a broken row.
func wholeOrNothing(gtx C, w layout.Widget) D {
	macro := op.Record(gtx.Ops)
	natural := gtx
	natural.Constraints.Max.X = math.MaxInt32
	d := w(natural)
	call := macro.Stop()
	if d.Size.X > gtx.Constraints.Max.X {
		return D{}
	}
	call.Add(gtx.Ops)
	return d
}

// checksChip is a row's checks at a glance: how many have finished while
// any run, how many failed once one has, all of them once they pass.
func checksChip(gtx C, th *ui.Theme, c backend.ChecksRollup) D {
	ic, fg, text := icPullCheck, th.TonePass, fmt.Sprintf("%d/%d", c.Passed, c.Total)
	switch verdictOf(c) {
	case verdictFailed:
		ic, fg, text = icPullX, th.Destructive, i18n.T("pulls.pullsList.checksFailed", i18n.Args{"count": c.Failed})
	case verdictPending:
		ic, fg, text = icPullRunning, th.ToneWait, fmt.Sprintf("%d/%d", c.Total-c.Pending, c.Total)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return pullGlyph(gtx, th, ic, ui.Space(3), fg) }),
		ui.Gap(1),
		layout.Rigid(th.Text(ui.TextXS, text).Mono().In(fg).Layout),
	)
}

// metaGlyph is a size-3 glyph and its text-xs words, gap-1.
func metaGlyph(gtx C, th *ui.Theme, ic *icons.Icon, text string, fg color.NRGBA) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(3), fg) }),
		ui.Gap(1),
		layout.Rigid(th.Text(ui.TextXS, text).In(fg).Layout),
	)
}

// listSkeleton is ListSkeleton: ragged title bars so it reads as a list.
func listSkeleton(gtx C, th *ui.Theme) D {
	widths := []float32{0.92, 0.8, 1, 0.75, 0.83}
	return layout.Inset{Left: ui.Space(3.5), Right: ui.Space(3.5), Top: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		full := gtx.Metric.PxToDp(gtx.Constraints.Max.X)
		var rows []layout.FlexChild
		for _, w := range widths {
			rows = append(rows,
				layout.Rigid(th.Skeleton(full*unit.Dp(w), ui.Space(3)).Layout),
				ui.VGap(1.5),
				layout.Rigid(th.Skeleton(full/3, ui.Space(2.5)).Layout),
				ui.VGap(3),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}
