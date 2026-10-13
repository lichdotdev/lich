package main

import (
	"fmt"
	"image/color"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (pulls.pullRequestView.*).
const (
	pullsOpenInSession = "Open in Session"
	pullsGoToSession   = "Go to session"
	pullsForkBlocked   = "The head branch lives on a fork that does not allow edits by maintainers — its commits could not be pushed back"
	pullsApprove       = "Approve"
	pullsApproved      = "Approved"
	pullsMerge         = "Merge"
	pullsRefresh       = "Refresh"
	pullsOpenExternal  = "Open"
	pullsBypass        = "%s, bypass rules"
	pullsNoneForBranch = "No open pull request for %s."
	pullsNone          = "No open pull request."
	pullsOpenOnGitHub  = "Open on GitHub"
)

// The PR island's padding: PullRequestView's px-6 under the toolbar strip.
var (
	pullsPadX       = ui.Space(6)
	pullsTitleSize  = unit.Sp(18)
	pullsToolbarPad = ui.Space(2)
)

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
// conflicting files, the tabs and the open tab's panel.
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
	conflictsAll                                         widget.Bool
	conflictsMore                                        widget.Clickable
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
// hidden, and the pull request's actions at the right.
func (v *pullView) withToolbar(gtx C, th *ui.Theme, p *pullsScreen, d *lichclient.PRDetail, act *pullsAction, w layout.Widget) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.UniformInset(pullsToolbarPad).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if !p.listHidden {
							return D{}
						}
						return iconAction(gtx, th, p.root, &v.showList, icPullShow, pullsShowList, false)
					}),
					layout.Flexed(1, func(gtx C) D {
						if d == nil {
							return layout.Spacer{Width: gtx.Metric.PxToDp(gtx.Constraints.Max.X), Height: ui.Space(8)}.Layout(gtx)
						}
						return v.actions(gtx, th, p, *d, act)
					}),
				)
			})
		}),
		layout.Flexed(1, w),
	)
}

// empty is PullsEmptyState: no pull request for the checkout's branch, and
// GitHub's page to open one.
func (v *pullView) empty(gtx C, th *ui.Theme, p *pullsScreen, act *pullsAction) D {
	if v.create.Clicked(gtx) {
		act.create = true
	}
	text := pullsNone
	if p.branch != "" {
		text = fmt.Sprintf(pullsNoneForBranch, p.branch)
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextSM, text).Muted().Layout),
			ui.VGap(3),
			layout.Rigid(func(gtx C) D {
				b := th.Button(&v.create, pullsOpenOnGitHub)
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
					layout.Rigid(func(gtx C) D { return pullTitle(gtx, th, d) }),
					ui.VGap(3),
					layout.Rigid(func(gtx C) D { return v.stats(gtx, th, p, d) }),
					layout.Rigid(func(gtx C) D { return v.conflicts(gtx, th, p, d) }),
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
		items = append(items, ui.WrapItem(func(gtx C) D { return stat(gtx, th, ic, text, fg) }))
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
		return stat(gtx, th, icPullMerged, "Merged", th.MutedForeground)
	case d.State == "CLOSED":
		return stat(gtx, th, icPullX, "Closed", th.MutedForeground)
	case d.IsDraft:
		return stat(gtx, th, icPullRequests, "Draft", th.ToneWait)
	}
	return stat(gtx, th, icPullRequests, "Open", th.TonePass)
}

func checksStat(th *ui.Theme, c lichclient.ChecksRollup) (*icons.Icon, string, color.NRGBA, bool) {
	switch verdictOf(c) {
	case verdictFailed:
		return icPullX, fmt.Sprintf("%d of %d checks failing", c.Failed, c.Total), th.Destructive, true
	case verdictPending:
		return icPullClock, fmt.Sprintf("%d of %d checks running", c.Pending, c.Total), th.ToneWait, true
	case verdictPassed:
		if c.Passed == 1 {
			return icPullCheck, "1 check passed", th.TonePass, true
		}
		return icPullCheck, fmt.Sprintf("%d checks passed", c.Passed), th.TonePass, true
	}
	return nil, "", color.NRGBA{}, false
}

func mergeableStat(th *ui.Theme, d lichclient.PRDetail) (*icons.Icon, string, color.NRGBA, bool) {
	switch {
	case d.State != "OPEN":
		return nil, "", color.NRGBA{}, false
	case conflictsWithBase(d):
		return icPullX, fmt.Sprintf(pullsConflictsText, d.BaseRefName), th.Destructive, true
	case d.Mergeable == "MERGEABLE":
		return icPullMerged, "Mergeable", th.TonePass, true
	}
	return icPullDashed, "Checking mergeability…", th.MutedForeground, true
}

// reviewStat is ReviewStat: the verdict, and on changes requested how the
// threads stand, GitHub keeping the verdict until the reviewer moves.
func reviewStat(th *ui.Theme, decision string, open, total int) (*icons.Icon, string, color.NRGBA, bool) {
	switch {
	case decision == "CHANGES_REQUESTED" && total > 0 && open > 0:
		return icPullX, fmt.Sprintf("Changes requested · %d of %d threads unresolved", open, total), th.Destructive, true
	case decision == "CHANGES_REQUESTED" && total > 0:
		return icPullCheck2, "Changes requested · all threads resolved", th.ToneWait, true
	}
	switch decision {
	case "APPROVED":
		return icPullCheck, "Approved", th.TonePass, true
	case "CHANGES_REQUESTED":
		return icPullX, "Changes requested", th.Destructive, true
	case "REVIEW_REQUIRED":
		return icPullDashed, "Review required", th.MutedForeground, true
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

var pullsTabLabels = map[pullsTab]string{tabOverview: "Overview", tabConversation: "Conversation", tabCommits: "Commits", tabFiles: "Files", tabChecks: "Checks"}

func (v *pullView) tabStrip(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail) D {
	items, resolved := conversationTimeline(p.store.conversation(p.project.Path, d.Number).val)
	counts := map[pullsTab]int{tabConversation: len(items) + len(resolved), tabCommits: len(d.Commits), tabFiles: d.ChangedFiles, tabChecks: d.Checks.Total}
	var triggers []ui.TabsTrigger
	for _, t := range tabsOf(d) {
		trigger := ui.TabsTrigger{Text: pullsTabLabels[t]}
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
