package main

import (
	"fmt"
	"image"
	"image/color"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The PR island's padding under the toolbar strip: the sidebar's, an 8px
// island edge plus a row's 10px, so text lands where a session card's does
// and a row's hover fill stops at the edge.
var (
	pullsRowEdge    = ui.Space(2)
	pullsRowPadX    = ui.Space(2.5)
	pullsPadX       = pullsRowEdge + pullsRowPadX
	pullsTitleSize  = unit.Sp(18)
	pullsToolbarPad = ui.Space(2)
	// pullsHeadingTop centers the title's first line on the h-8 buttons
	// beside it.
	pullsHeadingTop = ui.Space(0.5)
)

// pullsTitleMinWidth is the least the title keeps beside the actions; less
// and the actions move to their own line above it.
var pullsTitleMinWidth = unit.Dp(320)

// pullsAction is what the screen asks of the window this frame.
type pullsAction struct {
	session  *backend.PRDetail // open (or go to) a session on its branch
	handoff  string            // ... and write this at its prompt
	merge    string            // a method to merge with
	admin    bool
	approve  bool
	refresh  bool
	external string // a URL for the browser
	create   bool   // GitHub's new pull request page for the checkout's branch
	comment  string // a comment on the pull request itself
	reply    *threadReply
	resolve  *threadResolve
	threads  []backend.PRReviewThread // review threads for the PR's session
	edit     *commentEdit
	title    string  // a new title for the pull request
	body     *string // a new description; empty clears it
}

// open sends url to the browser: a link clicked in a Markdown body.
func (a *pullsAction) open(url string) { a.external = url }

// pullView is PullRequestView: the toolbar, the title, the status line, the
// tabs and the open tab's panel.
type pullView struct {
	session, handoff, moreClick              widget.Clickable
	moreMenu                                 ui.MenuState
	mergeClick                               widget.Clickable
	mergeMenu                                ui.MenuState
	back, indexLink, prev, next, switchClick widget.Clickable
	switchMenu                               ui.MenuState
	tips                                     map[string]*ui.Tooltip
	tabs                                     ui.TabsState
	panel                                    widget.List
	checksStat                               widget.Clickable
	reviewStat                               widget.Clickable
	commitOpen                               map[string]*widget.Bool
	commitClicks                             map[string]*widget.Clickable
	checkClicks                              map[string]*widget.Clickable
	conflictsPanel                           ui.DropdownPanelState
	conflictsRows                            widget.List
	// files is the Files tab, the dock's Review cards in pull request mode;
	// viewed is each PR's ticks, by URL, kept while the window lives.
	files  *reviewPanel
	viewed map[string]map[string]string
	// busy is the action in flight ("session", "approve", "merge"), which its
	// button says it is doing; set off the frame.
	busyMu sync.Mutex
	busy   string
	after  []func()

	talk      talkState
	tree      filesTree
	inline    inlineFrame
	root      *ui.Root
	md        markdownCache
	pending   map[string]*pendingReview // by PR URL
	submit    submitState
	mergeEdit mergeDialog
	edit      editState
	// shotMergeEdit is -pulls-merge-edit, opened once the detail is read.
	shotMergeEdit string
	// shotEdit is -pulls-edit, opened once the detail is read.
	shotEdit string
}

// onFrame queues f to run at the start of the next frame: what a write off
// the frame leaves the widgets to do once it landed.
func (v *pullView) onFrame(f func()) {
	v.busyMu.Lock()
	v.after = append(v.after, f)
	v.busyMu.Unlock()
}

func (v *pullView) runQueued() {
	v.busyMu.Lock()
	after := v.after
	v.after = nil
	v.busyMu.Unlock()
	for _, f := range after {
		f()
	}
}

func (v *pullView) setBusy(what string) {
	v.busyMu.Lock()
	v.busy = what
	v.busyMu.Unlock()
}

func (v *pullView) busyNow() string {
	v.busyMu.Lock()
	defer v.busyMu.Unlock()
	return v.busy
}

func (v *pullView) init() {
	v.panel.Axis = layout.Vertical
	v.conflictsRows.Axis = layout.Vertical
	v.tips = map[string]*ui.Tooltip{}
	v.commitOpen = map[string]*widget.Bool{}
	v.commitClicks = map[string]*widget.Clickable{}
	v.checkClicks = map[string]*widget.Clickable{}
	v.viewed = map[string]map[string]string{}
	v.pending = map[string]*pendingReview{}
	v.talk.init()
	v.md = markdownCache{}
	v.tree.closed = map[string]bool{}
	v.tree.rows.list.Axis = layout.Vertical
}

func (v *pullView) tip(key string) *ui.Tooltip {
	t, ok := v.tips[key]
	if !ok {
		t = new(ui.Tooltip)
		v.tips[key] = t
	}
	return t
}

func (v *pullView) layout(gtx C, th *ui.Theme, p *pullsScreen, act *pullsAction) D {
	v.runQueued()
	read := p.store.detail(p.project.Path, p.selected)
	switch {
	case read.val == nil && read.err != "":
		return v.withToolbar(gtx, th, p, nil, func(gtx C) D { return notice(gtx, th, read.err) })
	case read.val == nil && !read.loaded, read.val == nil && read.loading:
		return v.withToolbar(gtx, th, p, func(gtx C) D { return skeletonHeading(gtx, th) }, v.detailSkeleton(th))
	case read.val == nil:
		return v.withToolbar(gtx, th, p, nil, func(gtx C) D { return notice(gtx, th, i18n.T("pulls.pullsEmptyState.none")) })
	}
	d := v.edit.shown(*read.val, read, p.store.lastStale())
	if stillSettling(d) && !read.loading {
		gtx.Execute(op.InvalidateCmd{At: read.at.Add(pendingRecheck)})
	}
	if v.shotMergeEdit != "" {
		v.mergeEdit.show(v.shotMergeEdit, false, d)
		v.shotMergeEdit = ""
	}
	if v.shotEdit != "" {
		v.edit.openForShot(v.shotEdit, d)
		v.shotEdit = ""
	}
	heading := func(gtx C) D { return v.titleRow(gtx, th, p, d, act) }
	return v.withToolbar(gtx, th, p, heading, func(gtx C) D { return v.body(gtx, th, p, d, act) })
}

// withToolbar lays the strip over w: the crumb, then heading, the pull
// request's title row or its skeleton, when there is one.
func (v *pullView) withToolbar(gtx C, th *ui.Theme, p *pullsScreen, heading, w layout.Widget) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.UniformInset(pullsToolbarPad).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return v.crumb(gtx, th, p) }),
					layout.Rigid(func(gtx C) D {
						if heading == nil {
							return D{}
						}
						return heading(gtx)
					}),
				)
			})
		}),
		layout.Flexed(1, w),
	)
}

func (v *pullView) titleRow(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) D {
	actions, size := v.recordActions(gtx, th, p, d, act)
	if size.Y > gtx.Dp(ui.Space(8)) || gtx.Constraints.Max.X-size.X < gtx.Dp(pullsTitleMinWidth) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return v.actions(gtx, th, p, d, act)
			}),
			layout.Rigid(func(gtx C) D { return v.heading(gtx, th, p, d, act) }),
		)
	}
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D { return v.heading(gtx, th, p, d, act) }),
		layout.Rigid(func(gtx C) D {
			actions.Add(gtx.Ops)
			return D{Size: size}
		}),
	)
}

// navClicks carries out the crumb's clicks before the frame draws the pull
// request, so the frame that took one already draws where it leads.
func (v *pullView) navClicks(gtx C, p *pullsScreen, rows []backend.PRSummary) {
	if v.back.Clicked(gtx) || v.indexLink.Clicked(gtx) {
		p.backToIndex()
	}
	if v.prev.Clicked(gtx) {
		p.stepPR(rows, -1)
	}
	if v.next.Clicked(gtx) {
		p.stepPR(rows, 1)
	}
}

// crumb is the line over the title: back to the index, the index's name and
// the pull request's number, which opens the switcher; at the right its place
// in the index between previous and next.
func (v *pullView) crumb(gtx C, th *ui.Theme, p *pullsScreen) D {
	rows := p.visible(p.store.list(p.project.Path, p.state).val)
	at := position(rows, p.selected)
	return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				b := th.Button(&v.back, "")
				b.Icon, b.Variant, b.Size = icPullBack, ui.VariantGhost, ui.SizeIconSM
				return v.withTip(gtx, th, p, "back", i18n.T("pulls.pullsNav.back"), b.Layout)
			}),
			ui.Gap(2.5),
			layout.Rigid(func(gtx C) D { return hoverText(gtx, th, &v.indexLink, i18n.T("tabs.projectTabs.pullRequests")) }),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D { return icPullRight.Layout(gtx, ui.Space(3), th.MutedForeground) }),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D { return v.switcher(gtx, th, p, rows) }),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				if at < 0 {
					return D{}
				}
				return v.stepper(gtx, th, p, at, len(rows))
			}),
		)
	})
}

// hoverText is a click in muted words that brighten under the pointer.
func hoverText(gtx C, th *ui.Theme, click *widget.Clickable, text string) D {
	return click.Layout(gtx, func(gtx C) D {
		fg := th.MutedForeground
		if click.Hovered() {
			fg = th.Foreground
		}
		return th.Text(ui.TextXS, text).In(fg).Layout(gtx)
	})
}

// switcher is the pull request's number, a menu of the index's rows to go to
// another one.
func (v *pullView) switcher(gtx C, th *ui.Theme, p *pullsScreen, rows []backend.PRSummary) D {
	entries := make([]ui.MenuEntry, len(rows))
	for i, r := range rows {
		entries[i] = ui.MenuEntry{Kind: ui.MenuRadioItem, Label: fmt.Sprintf("#%d %s", r.Number, r.Title), Checked: r.Number == p.selected,
			OnClick: func() { p.openPR(r.Number) }}
	}
	face := func(gtx C) D {
		return v.switchClick.Layout(gtx, func(gtx C) D {
			bg := th.Muted
			if v.switchClick.Hovered() {
				bg = th.Accent
			}
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
				func(gtx C) D {
					on := th.On(bg)
					return layout.Inset{Left: ui.Space(2), Right: ui.Space(1.5), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(on.Text(ui.TextXS, fmt.Sprintf("#%d", p.selected)).Mono().Layout),
							ui.Gap(1),
							layout.Rigid(func(gtx C) D { return icPullDown.Layout(gtx, ui.Space(3), on.MutedForeground) }),
						)
					})
				})
		})
	}
	if len(entries) == 0 {
		return face(gtx)
	}
	return v.withTip(gtx, th, p, "switch", i18n.T("pulls.pullsNav.switch"), func(gtx C) D {
		return th.DropdownMenu(p.root, &v.switchMenu, entries).Layout(gtx, face)
	})
}

// stepper is "N of M" between previous and next, along the index's filter.
func (v *pullView) stepper(gtx C, th *ui.Theme, p *pullsScreen, at, total int) D {
	arrow := func(click *widget.Clickable, ic *icons.Icon, key, tip string, disabled bool) layout.Widget {
		return func(gtx C) D {
			b := th.Button(click, "")
			b.Icon, b.Variant, b.Size, b.Disabled = ic, ui.VariantGhost, ui.SizeIconSM, disabled
			return v.withTip(gtx, th, p, key, tip, b.Layout)
		}
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(arrow(&v.prev, icPullBack, "prev", i18n.T("pulls.pullsNav.previous"), at == 0)),
		ui.Gap(1),
		layout.Rigid(th.Text(ui.TextXS, i18n.T("pulls.pullsNav.position", i18n.Args{"index": at + 1, "total": total})).Mono().Muted().Layout),
		ui.Gap(1),
		layout.Rigid(arrow(&v.next, icPullRight, "next", i18n.T("pulls.pullsNav.next"), at == total-1)),
	)
}

// recordActions lays the actions at the width they want on one line, for
// withToolbar to place or to drop for the stacked layout.
func (v *pullView) recordActions(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) (op.CallOp, image.Point) {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	size := v.actions(gtx, th, p, d, act).Size
	return macro.Stop(), size
}

// heading is the title over its stats, its left edge on the toolbar strip's.
func (v *pullView) heading(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) D {
	return layout.Inset{Right: ui.Space(4), Top: pullsHeadingTop}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return v.pullTitle(gtx, th, p, d, act) }),
			ui.VGap(3),
			layout.Rigid(func(gtx C) D { return v.stats(gtx, th, p, d) }),
		)
	})
}

func (v *pullView) body(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: pullsToolbarPad, Right: pullsToolbarPad}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					ui.VGap(4),
					layout.Rigid(func(gtx C) D { return v.tabStrip(gtx, th, p, d) }),
				)
			})
		}),
		layout.Rigid(th.Separator),
		layout.Flexed(1, func(gtx C) D { return v.panelLayout(gtx, th, p, d, act) }),
	)
}

// stat is PullsStats' Stat: a size-3.5 glyph and its words in one tone.
func stat(gtx C, th *ui.Theme, ic *icons.Icon, text string, fg color.NRGBA) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return pullGlyph(gtx, th, ic, ui.Space(3.5), fg) }),
		ui.Gap(1.5),
		layout.Rigid(func(gtx C) D {
			l := th.Text(ui.TextXS, text).In(fg).Weight(font.Medium)
			l.MaxLines = 0
			return l.Layout(gtx)
		}),
	)
}

// pullGlyph is ic at size, icPullRunning drawn as a ui.Ring.
func pullGlyph(gtx C, th *ui.Theme, ic *icons.Icon, size unit.Dp, fg color.NRGBA) D {
	if ic == icPullRunning {
		return ui.Ring(gtx, size, fg, th.RingTrack(fg))
	}
	return ic.Layout(gtx, size, fg)
}

// stats is the status line: state, branches and who opened it, then the
// checks, mergeability and review, which Overview's facts carry instead.
func (v *pullView) stats(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail) D {
	items := []ui.WrapChild{
		ui.WrapItem(func(gtx C) D { return stateStat(gtx, th, d) }),
		ui.WrapItem(func(gtx C) D {
			return metaGlyph(gtx, th, icPullBranch, d.HeadRefName+" → "+d.BaseRefName, th.MutedForeground)
		}),
		ui.WrapItem(th.Text(ui.TextXS, i18n.T("pulls.pullsOverview.openedBy", i18n.Args{"author": d.Author})).Muted().Layout),
	}
	if v.openTab(d) != tabOverview {
		vs := v.verdicts(gtx, th, p, d)
		for _, w := range []layout.Widget{vs.checks, vs.merge, vs.review} {
			if w != nil {
				items = append(items, ui.WrapItem(w))
			}
		}
	}
	return ui.Wrap{GapX: ui.Space(4), GapY: ui.Space(1.5)}.Layout(gtx, items...)
}

// prVerdicts is what the pull request's checks, mergeability and review say,
// each nil when it says nothing.
type prVerdicts struct {
	checks, merge, review layout.Widget
}

// verdicts is the pull request's prVerdicts: the checks and the review open
// their tab, a conflict lists its files.
func (v *pullView) verdicts(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail) prVerdicts {
	if v.checksStat.Clicked(gtx) {
		v.showTab(d, tabChecks)
	}
	if v.reviewStat.Clicked(gtx) {
		v.showTab(d, tabConversation)
	}
	var out prVerdicts
	if ic, text, fg, ok := checksStat(th, d.Checks); ok {
		out.checks = func(gtx C) D {
			return v.checksStat.Layout(gtx, func(gtx C) D { return stat(gtx, th, ic, text, fg) })
		}
	}
	if ic, text, fg, ok := mergeableStat(th, d); ok {
		out.merge = func(gtx C) D {
			face := func(gtx C) D { return stat(gtx, th, ic, text, fg) }
			if conflictsWithBase(d) {
				return v.conflictsStat(gtx, th, p, d, face)
			}
			return face(gtx)
		}
	}
	open, total := threadTally(p.store.conversation(p.project.Path, d.Number).val)
	if ic, text, fg, ok := reviewStat(th, d.ReviewDecision, open, total); ok {
		out.review = func(gtx C) D {
			return v.reviewStat.Layout(gtx, func(gtx C) D { return stat(gtx, th, ic, text, fg) })
		}
	}
	return out
}

func stateStat(gtx C, th *ui.Theme, d backend.PRDetail) D {
	switch {
	case d.State == "MERGED":
		return stat(gtx, th, icPullMerged, i18n.T("pulls.pullsStats.merged"), th.MutedForeground)
	case d.State == "CLOSED":
		return stat(gtx, th, icPullX, i18n.T("pulls.pullsStats.closed"), th.MutedForeground)
	case d.IsDraft:
		return stat(gtx, th, icPullRequests, i18n.T("pulls.pullsStats.draft"), th.ToneWait)
	}
	return stat(gtx, th, icPullRequests, i18n.T("pulls.pullsStats.open"), th.TonePass)
}

func checksStat(th *ui.Theme, c backend.ChecksRollup) (*icons.Icon, string, color.NRGBA, bool) {
	switch verdictOf(c) {
	case verdictFailed:
		return icPullX, i18n.T("pulls.pullsStats.checksFailing", i18n.Args{"failed": c.Failed, "total": c.Total}), th.Destructive, true
	case verdictPending:
		return icPullRunning, i18n.T("pulls.pullsStats.checksRunning", i18n.Args{"pending": c.Pending, "total": c.Total}), th.ToneWait, true
	case verdictPassed:
		return icPullCheck, i18n.T("pulls.pullsStats.checksPassed", i18n.Args{"count": c.Passed}), th.TonePass, true
	}
	return nil, "", color.NRGBA{}, false
}

func mergeableStat(th *ui.Theme, d backend.PRDetail) (*icons.Icon, string, color.NRGBA, bool) {
	switch {
	case d.State != "OPEN":
		return nil, "", color.NRGBA{}, false
	case conflictsWithBase(d):
		return icPullX, i18n.T("pulls.pullsStats.conflictsWith", i18n.Args{"base": d.BaseRefName}), th.Destructive, true
	case d.Mergeable == "MERGEABLE":
		return icPullMerged, i18n.T("pulls.pullsStats.mergeable"), th.TonePass, true
	}
	return icPullDashed, i18n.T("pulls.pullsStats.checkingMergeability"), th.MutedForeground, true
}

// reviewStat is ReviewStat: the verdict, and on changes requested how the
// threads stand, GitHub keeping the verdict until the reviewer moves.
func reviewStat(th *ui.Theme, decision string, open, total int) (*icons.Icon, string, color.NRGBA, bool) {
	switch {
	case decision == "CHANGES_REQUESTED" && total > 0 && open > 0:
		return icPullX, i18n.T("pulls.pullsStats.threadsUnresolved", i18n.Args{"open": open, "total": total}), th.Destructive, true
	case decision == "CHANGES_REQUESTED" && total > 0:
		return icPullCheck2, i18n.T("pulls.pullsStats.threadsResolved"), th.ToneWait, true
	}
	switch decision {
	case "APPROVED":
		return icPullCheck, i18n.T("pulls.pullsStats.approved"), th.TonePass, true
	case "CHANGES_REQUESTED":
		return icPullX, i18n.T("pulls.pullsStats.changesRequested"), th.Destructive, true
	case "REVIEW_REQUIRED":
		return icPullDashed, i18n.T("pulls.pullsStats.reviewRequired"), th.MutedForeground, true
	}
	return nil, "", color.NRGBA{}, false
}

// tabsOf is the tabs a pull request shows, in order: Checks only when it has
// any.
func tabsOf(d backend.PRDetail) []pullsTab {
	tabs := []pullsTab{tabOverview, tabConversation, tabCommits, tabFiles}
	if d.Checks.Total > 0 {
		tabs = append(tabs, tabChecks)
	}
	return tabs
}

// openTab is the tab on screen; one the PR does not show falls back to
// Overview.
func (v *pullView) openTab(d backend.PRDetail) pullsTab {
	tabs := tabsOf(d)
	if v.tabs.Selected < 0 || v.tabs.Selected >= len(tabs) {
		return tabOverview
	}
	return tabs[v.tabs.Selected]
}

func (v *pullView) showTab(d backend.PRDetail, tab pullsTab) {
	for i, t := range tabsOf(d) {
		if t == tab {
			v.tabs.Selected = i
		}
	}
}

func (t pullsTab) label() string {
	switch t {
	case tabConversation:
		return i18n.T("pulls.pullRequestView.tab.conversation")
	case tabCommits:
		return i18n.T("pulls.pullRequestView.tab.commits")
	case tabFiles:
		return i18n.T("pulls.pullsView.tabFiles")
	case tabChecks:
		return i18n.T("pulls.pullRequestView.tab.checks")
	}
	return i18n.T("pulls.pullRequestView.tab.overview")
}

func (v *pullView) tabStrip(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail) D {
	items, resolved := conversationTimeline(p.store.conversation(p.project.Path, d.Number).val)
	counts := map[pullsTab]int{tabConversation: len(items) + len(resolved), tabCommits: len(d.Commits), tabFiles: d.ChangedFiles, tabChecks: d.Checks.Total}
	var triggers []ui.TabsTrigger
	for _, t := range tabsOf(d) {
		trigger := ui.TabsTrigger{Text: t.label()}
		if n := counts[t]; n > 0 {
			trigger.Trailing = th.Text(ui.TextSM, fmt.Sprint(n)).Muted().Layout
		}
		triggers = append(triggers, trigger)
	}
	if v.openTab(d) == tabOverview {
		v.tabs.Selected = 0
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
			if d.Additions == 0 && d.Deletions == 0 {
				return D{}
			}
			return layout.Inset{Left: ui.Space(4)}.Layout(gtx, func(gtx C) D { return diffStat(gtx, th, d.Additions, d.Deletions) })
		}),
	)
}
