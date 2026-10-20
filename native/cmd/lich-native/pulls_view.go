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

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The PR island's padding: PullRequestView's px-6 under the toolbar strip.
var (
	pullsPadX       = ui.Space(6)
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
	session  *lichclient.PRDetail // open (or go to) a session on its branch
	handoff  string               // ... and write this at its prompt
	merge    string               // a method to merge with
	admin    bool
	approve  bool
	refresh  bool
	external string // a URL for the browser
	create   bool   // GitHub's new pull request page for the checkout's branch
	comment  string // a comment on the pull request itself
	reply    *threadReply
	resolve  *threadResolve
}

// open sends url to the browser: a link clicked in a Markdown body.
func (a *pullsAction) open(url string) { a.external = url }

// pullView is PullRequestView: the toolbar, the title, the status line, the
// tabs and the open tab's panel.
type pullView struct {
	session, handoff, approve, refresh, external, create widget.Clickable
	mergeClick                                           widget.Clickable
	mergeMenu                                            ui.MenuState
	showList                                             iconActionState
	tips                                                 map[string]*ui.Tooltip
	tabs                                                 ui.TabsState
	panel                                                widget.List
	checksStat                                           widget.Clickable
	reviewStat                                           widget.Clickable
	commitOpen                                           map[string]*widget.Bool
	commitClicks                                         map[string]*widget.Clickable
	checkClicks                                          map[string]*widget.Clickable
	conflictsPanel                                       ui.DropdownPanelState
	conflictsRows                                        widget.List
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
	// shotMergeEdit is -pulls-merge-edit, opened once the detail is read.
	shotMergeEdit string
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
	if v.showList.click.Clicked(gtx) {
		p.listHidden = false
	}
	if p.selected == 0 {
		return v.withToolbar(gtx, th, p, nil, act, func(gtx C) D { return v.empty(gtx, th, p, act) })
	}
	read := p.store.detail(p.project.Path, p.selected)
	switch {
	case read.val == nil && read.err != "":
		return v.withToolbar(gtx, th, p, nil, act, func(gtx C) D { return notice(gtx, th, read.err) })
	case read.val == nil && !read.loaded, read.val == nil && read.loading:
		return v.withToolbar(gtx, th, p, nil, act, func(gtx C) D { return detailSkeleton(gtx, th) })
	case read.val == nil:
		return v.withToolbar(gtx, th, p, nil, act, func(gtx C) D { return v.empty(gtx, th, p, act) })
	}
	d := *read.val
	if v.shotMergeEdit != "" {
		v.mergeEdit.show(v.shotMergeEdit, false, d)
		v.shotMergeEdit = ""
	}
	return v.withToolbar(gtx, th, p, &d, act, func(gtx C) D { return v.body(gtx, th, p, d, act) })
}

// withToolbar lays the h-12 strip over w: the list's show button when it is
// hidden, then the pull request's title and stats with its actions at the
// right on the title's line. An island too narrow for both on one line puts
// the actions on their own line above the title.
func (v *pullView) withToolbar(gtx C, th *ui.Theme, p *pullsScreen, d *lichclient.PRDetail, act *pullsAction, w layout.Widget) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.UniformInset(pullsToolbarPad).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				if d == nil {
					return v.toolbarRow(gtx, th, p, func(gtx C) D {
						return layout.Spacer{Width: gtx.Metric.PxToDp(gtx.Constraints.Max.X), Height: ui.Space(8)}.Layout(gtx)
					}, nil)
				}
				actions, size := v.recordActions(gtx, th, p, *d, act)
				if size.Y > gtx.Dp(ui.Space(8)) || gtx.Constraints.Max.X-size.X < gtx.Dp(pullsTitleMinWidth) {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return v.toolbarRow(gtx, th, p, func(gtx C) D {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return v.actions(gtx, th, p, *d, act)
							}, nil)
						}),
						layout.Rigid(func(gtx C) D { return v.heading(gtx, th, p, *d, false) }),
					)
				}
				return v.toolbarRow(gtx, th, p, func(gtx C) D { return v.heading(gtx, th, p, *d, p.listHidden) }, func(gtx C) D {
					actions.Add(gtx.Ops)
					return D{Size: size}
				})
			})
		}),
		layout.Flexed(1, w),
	)
}

// toolbarRow is the list's show button while the list is hidden, fill
// taking the width it leaves, and right at the end.
func (v *pullView) toolbarRow(gtx C, th *ui.Theme, p *pullsScreen, fill, right layout.Widget) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if !p.listHidden {
				return D{}
			}
			return iconAction(gtx, th, p.root, &v.showList, icPullShow, i18n.T("pulls.pullsList.show"), false)
		}),
		layout.Flexed(1, fill),
		layout.Rigid(func(gtx C) D {
			if right == nil {
				return D{}
			}
			return right(gtx)
		}),
	)
}

// recordActions lays the actions at the width they want on one line, for
// withToolbar to place or to drop for the stacked layout.
func (v *pullView) recordActions(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, act *pullsAction) (op.CallOp, image.Point) {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	size := v.actions(gtx, th, p, d, act).Size
	return macro.Stop(), size
}

// heading is the title over its stats, its left edge on the body's px-6.
func (v *pullView) heading(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, besideShowList bool) D {
	left := pullsPadX - pullsToolbarPad
	if besideShowList {
		left = ui.Space(2)
	}
	return layout.Inset{Left: left, Right: ui.Space(4), Top: pullsHeadingTop}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return pullTitle(gtx, th, d) }),
			ui.VGap(3),
			layout.Rigid(func(gtx C) D { return v.stats(gtx, th, p, d) }),
		)
	})
}

// empty is PullsEmptyState: no pull request for the checkout's branch, and
// GitHub's page to open one.
func (v *pullView) empty(gtx C, th *ui.Theme, p *pullsScreen, act *pullsAction) D {
	if v.create.Clicked(gtx) {
		act.create = true
	}
	text := i18n.T("pulls.pullsEmptyState.none")
	if p.branch != "" {
		text = i18n.T("pulls.pullsEmptyState.noneFor", i18n.Args{"branch": p.branch})
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextSM, text).Muted().Layout),
			ui.VGap(3),
			layout.Rigid(func(gtx C) D {
				b := th.Button(&v.create, i18n.T("pulls.pullsEmptyState.openOnGitHub"))
				b.Icon, b.Variant, b.Size = icPullOut, ui.VariantOutline, ui.SizeSM
				return b.Layout(gtx)
			}),
		)
	})
}

func (v *pullView) body(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, act *pullsAction) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: pullsPadX, Right: pullsPadX}.Layout(gtx, func(gtx C) D {
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

func pullTitle(gtx C, th *ui.Theme, d lichclient.PRDetail) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(th.Text(pullsTitleSize, fmt.Sprintf("#%d", d.Number)).Muted().Weight(font.SemiBold).Layout),
		ui.Gap(1.5),
		layout.Flexed(1, func(gtx C) D {
			l := th.Text(pullsTitleSize, d.Title).Weight(font.SemiBold)
			l.MaxLines = 0
			return l.Layout(gtx)
		}),
	)
}

// stat is PullsStats' Stat: a size-3.5 glyph and its words in one tone.
func stat(gtx C, th *ui.Theme, ic *icons.Icon, text string, fg color.NRGBA) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), fg) }),
		ui.Gap(1.5),
		layout.Rigid(th.Text(ui.TextXS, text).In(fg).Weight(font.Medium).Layout),
	)
}

// stats is the status line: state, branches, checks, mergeability, review.
func (v *pullView) stats(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail) D {
	if v.checksStat.Clicked(gtx) {
		v.showTab(d, tabChecks)
	}
	if v.reviewStat.Clicked(gtx) {
		v.showTab(d, tabConversation)
	}
	open, total := threadTally(p.store.conversation(p.project.Path, d.Number).val)
	items := []ui.WrapChild{
		ui.WrapItem(func(gtx C) D { return stateStat(gtx, th, d) }),
		ui.WrapItem(func(gtx C) D {
			return metaGlyph(gtx, th, icPullBranch, d.HeadRefName+" → "+d.BaseRefName, th.MutedForeground)
		}),
	}
	if ic, text, fg, ok := checksStat(th, d.Checks); ok {
		items = append(items, ui.WrapItem(func(gtx C) D {
			return v.checksStat.Layout(gtx, func(gtx C) D { return stat(gtx, th, ic, text, fg) })
		}))
	}
	if ic, text, fg, ok := mergeableStat(th, d); ok {
		items = append(items, ui.WrapItem(func(gtx C) D {
			face := func(gtx C) D { return stat(gtx, th, ic, text, fg) }
			if conflictsWithBase(d) {
				return v.conflictsStat(gtx, th, p, d, face)
			}
			return face(gtx)
		}))
	}
	if ic, text, fg, ok := reviewStat(th, d.ReviewDecision, open, total); ok {
		items = append(items, ui.WrapItem(func(gtx C) D {
			return v.reviewStat.Layout(gtx, func(gtx C) D { return stat(gtx, th, ic, text, fg) })
		}))
	}
	return ui.Wrap{GapX: ui.Space(4), GapY: ui.Space(1.5)}.Layout(gtx, items...)
}

func stateStat(gtx C, th *ui.Theme, d lichclient.PRDetail) D {
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

func checksStat(th *ui.Theme, c lichclient.ChecksRollup) (*icons.Icon, string, color.NRGBA, bool) {
	switch verdictOf(c) {
	case verdictFailed:
		return icPullX, i18n.T("pulls.pullsStats.checksFailing", i18n.Args{"failed": c.Failed, "total": c.Total}), th.Destructive, true
	case verdictPending:
		return icPullClock, i18n.T("pulls.pullsStats.checksRunning", i18n.Args{"pending": c.Pending, "total": c.Total}), th.ToneWait, true
	case verdictPassed:
		return icPullCheck, i18n.T("pulls.pullsStats.checksPassed", i18n.Args{"count": c.Passed}), th.TonePass, true
	}
	return nil, "", color.NRGBA{}, false
}

func mergeableStat(th *ui.Theme, d lichclient.PRDetail) (*icons.Icon, string, color.NRGBA, bool) {
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
func tabsOf(d lichclient.PRDetail) []pullsTab {
	tabs := []pullsTab{tabOverview, tabConversation, tabCommits, tabFiles}
	if d.Checks.Total > 0 {
		tabs = append(tabs, tabChecks)
	}
	return tabs
}

// openTab is the tab on screen; one the PR does not show falls back to
// Overview.
func (v *pullView) openTab(d lichclient.PRDetail) pullsTab {
	tabs := tabsOf(d)
	if v.tabs.Selected < 0 || v.tabs.Selected >= len(tabs) {
		return tabOverview
	}
	return tabs[v.tabs.Selected]
}

func (v *pullView) showTab(d lichclient.PRDetail, tab pullsTab) {
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

func (v *pullView) tabStrip(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail) D {
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
	return tabs.Layout(gtx)
}

func detailSkeleton(gtx C, th *ui.Theme) D {
	return layout.Inset{Left: pullsPadX, Right: pullsPadX}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.Skeleton(ui.Space(96), ui.Space(5)).Layout),
			ui.VGap(4),
			layout.Rigid(th.Skeleton(ui.Space(72), ui.Space(3)).Layout),
			ui.VGap(6),
			layout.Rigid(th.Skeleton(0, ui.Space(24)).Layout),
		)
	})
}
