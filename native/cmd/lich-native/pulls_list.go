package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
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
	rows := filterPulls(list.val, pullsFilter(p.filter.Selected), text)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return p.listHeader(gtx, th) }),
		layout.Rigid(func(gtx C) D { return p.filterTrack(gtx, th, counts) }),
		layout.Rigid(func(gtx C) D { return p.stateRow(gtx, th) }),
		layout.Flexed(1, func(gtx C) D { return p.rowList(gtx, th, list, rows) }),
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

// row is PullRow: the state glyph, "#N title" over two lines at most, then
// who, when and the facts worth a glance.
func (p *pullsScreen) row(gtx C, th *ui.Theme, pr lichclient.PRSummary, now time.Time) D {
	click := p.rowClicks[pr.Number]
	if click == nil {
		click = new(widget.Clickable)
		p.rowClicks[pr.Number] = click
	}
	if click.Clicked(gtx) {
		p.selected = pr.Number
	}
	_, checkedOut := p.checkedOut(pr.HeadRefName)
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
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return rowGlyph(gtx, on, pr) }),
						ui.Gap(2),
						layout.Flexed(1, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D { return rowTitle(gtx, on, pr) }),
								ui.VGap(0.5),
								layout.Rigid(func(gtx C) D { return rowMeta(gtx, on, pr, checkedOut, now) }),
							)
						}),
					)
				})
			})
	})
}

func rowGlyph(gtx C, th *ui.Theme, pr lichclient.PRSummary) D {
	ic := icPullRequests
	switch pr.State {
	case "MERGED":
		ic = icPullMerged
	case "CLOSED":
		ic = icPullClosed
	}
	fg := th.MutedForeground
	if pr.IsDraft {
		fg = th.ToneWait
	}
	return layout.Inset{Top: ui.Space(0.75)}.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), fg) })
}

func rowTitle(gtx C, th *ui.Theme, pr lichclient.PRSummary) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextSM, fmt.Sprintf("#%d", pr.Number)).Mono().Muted().Layout),
		ui.Gap(1),
		layout.Flexed(1, func(gtx C) D {
			l := th.Text(ui.TextSM, pr.Title)
			l.MaxLines = 2
			return l.Layout(gtx)
		}),
	)
}

// rowMeta is the row's second line; only a review verdict worth acting on
// shows, REVIEW_REQUIRED being most rows' resting state.
func rowMeta(gtx C, th *ui.Theme, pr lichclient.PRSummary, checkedOut bool, now time.Time) D {
	items := []layout.FlexChild{
		layout.Rigid(th.Text(ui.TextXS, pr.Author).Muted().Layout),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextXS, updatedAgo(pr.UpdatedAt, now)).Mono().Muted().Layout),
	}
	add := func(w layout.Widget) { items = append(items, ui.Gap(2), layout.Rigid(w)) }
	if v := verdictOf(pr.Checks); v != verdictNone {
		add(func(gtx C) D { return verdictDot(gtx, th, v) })
	}
	if pr.IsDraft {
		add(th.Text(ui.TextXS, i18n.T("pulls.pullsList.draft")).In(th.ToneWait).Layout)
	}
	switch pr.ReviewDecision {
	case "APPROVED":
		add(th.Text(ui.TextXS, i18n.T("pulls.pullsStats.approved")).In(th.TonePass).Layout)
	case "CHANGES_REQUESTED":
		add(th.Text(ui.TextXS, i18n.T("pulls.pullsStats.changesRequested")).In(th.Destructive).Layout)
	}
	if pr.IsCrossRepository {
		add(th.Text(ui.TextXS, i18n.T("pulls.pullsList.fork")).Muted().Layout)
	}
	if checkedOut {
		add(func(gtx C) D {
			return metaGlyph(gtx, th, icPullTerm, i18n.T("pulls.pullsList.checkedOut"), th.MutedForeground)
		})
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, items...)
}

// verdictDot is the size-1.5 dot a row's checks read as.
func verdictDot(gtx C, th *ui.Theme, v checkVerdict) D {
	col := th.TonePass
	switch v {
	case verdictFailed:
		col = th.Destructive
	case verdictPending:
		col = th.MutedForeground
	}
	side := gtx.Dp(ui.Space(1.5))
	defer clip.Ellipse{Max: image.Pt(side, side)}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, col)
	return D{Size: image.Pt(side, side)}
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
