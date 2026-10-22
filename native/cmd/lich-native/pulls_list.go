package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
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

// listLayout is PullsList: the filter field and the counted quick filters,
// the state menu, then the rows.
func (p *pullsScreen) listLayout(gtx C, th *ui.Theme, list pullsRead[[]lichclient.PRSummary]) D {
	text := p.query.Text()
	counts := filterCounts(list.val, text)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return p.listHeader(gtx, th) }),
		layout.Rigid(func(gtx C) D { return p.filterTrack(gtx, th, counts) }),
		layout.Rigid(func(gtx C) D { return p.stateRow(gtx, th) }),
		layout.Flexed(1, func(gtx C) D {
			// Read after the filter tabs took their click.
			rows := filterPulls(list.val, pullsFilter(p.filter.Selected), text)
			return p.rowList(gtx, th, list, rows)
		}),
	)
}

// listHeader is the h-11 strip: the field, the hide button beside it.
func (p *pullsScreen) listHeader(gtx C, th *ui.Theme) D {
	if p.toggleList.click.Clicked(gtx) {
		p.listHidden = true
	}
	field := th.Input(&p.query, i18n.T("pulls.pullsList.searchHint"))
	field.Leading, field.Height, field.TextSize, field.Clear = icPullSearch, ui.Space(7), ui.TextXS, &p.clearQuery
	return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, field.Layout),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D {
				return iconAction(gtx, th, p.root, &p.toggleList, icPullHide, i18n.T("pulls.pullsList.hide"), false)
			}),
		)
	})
}

func (p *pullsScreen) filterTabs(th *ui.Theme, counts [pullsFilterCount]int) ui.TabsStyle {
	triggers := make([]ui.TabsTrigger, pullsFilterCount)
	for i := range triggers {
		n := counts[i]
		triggers[i] = ui.TabsTrigger{Text: pullsFilter(i).label(), Trailing: func(gtx C) D {
			return th.Text(ui.TextXS, fmt.Sprint(n)).Muted().Layout(gtx)
		}}
	}
	return th.Tabs(&p.filter, triggers...)
}

// listWidth is the list island's width: w-80, or what the filter tabs need
// beside their px-2 inset, bounded by pullsListMaxWidth.
func (p *pullsScreen) listWidth(gtx C, th *ui.Theme, counts [pullsFilterCount]int) int {
	need := p.filterTabs(th, counts).NaturalWidth(gtx) + 2*gtx.Dp(filterTrackInset)
	return min(max(gtx.Dp(pullsListWidth), need), gtx.Dp(pullsListMaxWidth))
}

var filterTrackInset = ui.Space(2)

func (p *pullsScreen) filterTrack(gtx C, th *ui.Theme, counts [pullsFilterCount]int) D {
	tabs := p.filterTabs(th, counts)
	return layout.Inset{Left: filterTrackInset, Right: filterTrackInset}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return tabs.Layout(gtx)
	})
}

// stateRow is the uppercase line over the rows, a menu of the states gh is
// asked for.
func (p *pullsScreen) stateRow(gtx C, th *ui.Theme) D {
	entries := make([]ui.MenuEntry, len(pullsStates))
	for i, s := range pullsStates {
		entries[i] = ui.MenuEntry{Kind: ui.MenuRadioItem, Label: s.label(), Checked: s == p.state, OnClick: func() { p.state = s }}
	}
	menu := th.DropdownMenu(p.root, &p.stateMenu, entries)
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
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
	})
}

func (p *pullsScreen) rowList(gtx C, th *ui.Theme, list pullsRead[[]lichclient.PRSummary], rows []lichclient.PRSummary) D {
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
	return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5), Bottom: ui.Space(3)}.Layout(gtx, func(gtx C) D {
		return p.rowScroll(gtx, th).Layout(gtx, len(rows), func(gtx C, i int) D {
			return layout.Inset{Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D { return p.row(gtx, th, rows[i], now) })
		})
	})
}

// rowScroll is the sidebar's scrollColumn over the rows: the scrollbar takes
// room only once they overflow, so the rows sit as far from the right edge as
// from the left until then.
func (p *pullsScreen) rowScroll(gtx C, th *ui.Theme) material.ListStyle {
	list := th.ScrollArea(&p.rows)
	list.Indicator.MinorWidth = ui.Space(2)
	list.Indicator.CornerRadius = ui.Space(1)
	list.Track.MinorPadding = 0
	list.AnchorStrategy = material.Overlay
	if p.rows.Position.Length > gtx.Constraints.Max.Y {
		list.AnchorStrategy = material.Occupy
	}
	return list
}

// row is PullRow: "#N title" over two lines at most beside its checks, then
// who, when, the facts worth a glance and the checkout it sits in.
func (p *pullsScreen) row(gtx C, th *ui.Theme, pr lichclient.PRSummary, now time.Time) D {
	click := p.rowClicks[pr.Number]
	if click == nil {
		click = new(widget.Clickable)
		p.rowClicks[pr.Number] = click
	}
	if click.Clicked(gtx) {
		p.selected = pr.Number
	}
	checkout, _ := p.checkedOut(pr.HeadRefName)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		switch {
		case pr.Number == p.selected:
			bg = th.Accent
		case click.Hovered():
			bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
			func(gtx C) D {
				on := th.On(bg)
				return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return rowTitle(gtx, on, pr) }),
						ui.VGap(0.5),
						layout.Rigid(func(gtx C) D { return rowMeta(gtx, on, pr, checkoutName(checkout), now) }),
					)
				})
			})
	})
}

// chipFirstLineTop centers the text-xs checks chip on the text-sm title's
// first line.
var chipFirstLineTop = ui.Space(0.5)

func rowTitle(gtx C, th *ui.Theme, pr lichclient.PRSummary) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextSM, fmt.Sprintf("#%d", pr.Number)).Mono().Muted().Layout),
		ui.Gap(1),
		layout.Flexed(1, func(gtx C) D {
			l := th.Text(ui.TextSM, pr.Title)
			l.MaxLines = 2
			return l.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if verdictOf(pr.Checks) == verdictNone {
				return D{}
			}
			return layout.Inset{Left: ui.Space(2), Top: chipFirstLineTop}.Layout(gtx, func(gtx C) D {
				return checksChip(gtx, th, pr.Checks)
			})
		}),
	)
}

// checkoutName is the folder a PR's branch is checked out in, the name its
// block in the sidebar carries; "" when it is checked out nowhere.
func checkoutName(w lichclient.Worktree) string {
	if w.Path == "" {
		return ""
	}
	return filepath.Base(w.Path)
}

// rowMeta is the row's second line; only a review verdict worth acting on
// shows, REVIEW_REQUIRED being most rows' resting state. The facts take the
// room first: the author truncates once they don't fit, and the checkout at
// the far end shows only where it fits whole.
func rowMeta(gtx C, th *ui.Theme, pr lichclient.PRSummary, checkout string, now time.Time) D {
	facts := []layout.FlexChild{
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min.X = 0
			return th.Text(ui.TextXS, pr.Author).Muted().Layout(gtx)
		}),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextXS, updatedAgo(pr.UpdatedAt, now)).Mono().Muted().Layout),
	}
	add := func(w layout.Widget) { facts = append(facts, ui.Gap(2), layout.Rigid(w)) }
	if pr.IsDraft {
		add(func(gtx C) D { return draftBadge(gtx, th) })
	}
	switch {
	case pr.ReviewDecision == "APPROVED":
		add(th.Text(ui.TextXS, i18n.T("pulls.pullsStats.approved")).In(th.TonePass).Layout)
	case waitsOnReviewer(pr):
		add(func(gtx C) D {
			return metaGlyph(gtx, th, icPullCheck2, i18n.T("pulls.pullsStats.changesRequested"), th.ToneWait)
		})
	case pr.ReviewDecision == "CHANGES_REQUESTED":
		add(th.Text(ui.TextXS, i18n.T("pulls.pullsStats.changesRequested")).In(th.Destructive).Layout)
	}
	if pr.IsCrossRepository {
		add(th.Text(ui.TextXS, i18n.T("pulls.pullsList.fork")).Muted().Layout)
	}
	items := []layout.FlexChild{layout.Rigid(func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, facts...)
	})}
	if checkout != "" {
		items = append(items, ui.Gap(2), layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return wholeOrNothing(gtx, func(gtx C) D { return metaGlyph(gtx, th, icCheckout, checkout, th.MutedForeground) })
			})
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, items...)
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
func waitsOnReviewer(pr lichclient.PRSummary) bool {
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
func checksChip(gtx C, th *ui.Theme, c lichclient.ChecksRollup) D {
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
