package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// pullsListWidth is PullsList's w-80.
var pullsListWidth = unit.Dp(320)

var (
	icPullMerged  = icons.Lucide("git-merge")
	icPullClosed  = icons.Lucide("git-pull-request-closed")
	icPullBranch  = icons.Lucide("git-branch")
	icPullCheck   = icons.Lucide("check")
	icPullX       = icons.Lucide("x")
	icPullClock   = icons.Lucide("clock")
	icPullDashed  = icons.Lucide("circle-dashed")
	icPullCheck2  = icons.Lucide("check-check")
	icPullRefresh = icons.Lucide("refresh-cw")
	icPullOut     = icons.Lucide("external-link")
	icPullSession = icons.Lucide("square-terminal")
	icPullWrench  = icons.Lucide("wrench")
	icPullDown    = icons.Lucide("chevron-down")
	icPullRight   = icons.Lucide("chevron-right")
	icPullHide    = icons.Lucide("panel-left-close")
	icPullShow    = icons.Lucide("panel-left-open")
	icPullSearch  = icons.Lucide("search")
	icPullTerm    = icons.Lucide("terminal")
	icPullTeam    = icons.Lucide("users")
	icPullTalk    = icons.Lucide("message-square")
)

// pullsTab is the PR island's open tab.
type pullsTab int

const (
	tabOverview pullsTab = iota
	tabConversation
	tabCommits
	tabFiles
	tabChecks
)

// pullsScreen is the Pulls screen: the list island and the pull request island
// over the stage, like Settings. It reads through the window's pullsStore and
// leaves what an action means for the window (pullsAction).
type pullsScreen struct {
	store *pullsStore
	root  *ui.Root

	open     bool
	project  lichclient.Project
	selected int // the PR in view, 0 for none yet
	// branchPR is the PR of the checkout's branch, which the screen opens on
	// until a row is chosen.
	branchPR   int
	branch     string // the checkout's branch, which the empty state names
	state      pullsState
	filter     ui.TabsState
	listHidden bool
	// slide eases the list island in from the left and back out, on the
	// dock's curve.
	slide dockSlide

	query      widget.Editor
	clearQuery widget.Clickable
	stateMenu  ui.MenuState
	stateClick widget.Clickable
	toggleList iconActionState
	rows       widget.List
	rowClicks  map[int]*widget.Clickable

	view    pullView
	confirm removeConfirm
	// files is the window's active session and checkout, for the Files tab.
	files filesInput
	// sessionAt is whether a session of the project sits in a checkout.
	sessionAt func(projectID, path string) bool
}

func (p *pullsScreen) hasSessionAt(path string) bool {
	return p.sessionAt != nil && p.sessionAt(p.project.ID, path)
}

func newPullsScreen(store *pullsStore, root *ui.Root) *pullsScreen {
	p := &pullsScreen{store: store, root: root, state: pullsOpen, rowClicks: map[int]*widget.Clickable{}}
	p.rows.Axis = layout.Vertical
	p.slide.snap(true)
	p.view.init()
	p.view.root = root
	return p
}

// show opens the screen on project, at its checkout's PR when it has one.
func (p *pullsScreen) show(project lichclient.Project, branch string, branchPR int) {
	if p.project.ID != project.ID {
		p.selected = 0
	}
	p.open, p.project, p.branch, p.branchPR = true, project, branch, branchPR
	if branchPR != 0 {
		p.selected = branchPR
	}
	p.store.stale()
}

func (p *pullsScreen) hide() { p.open = false }

// layout draws the two islands across gtx and answers what was asked of the
// window; th is on the window's canvas.
func (p *pullsScreen) layout(gtx C, th *ui.Theme) pullsAction {
	var act pullsAction
	gtx.Constraints.Min = gtx.Constraints.Max
	rows := p.store.list(p.project.Path, p.state)
	if p.selected == 0 && p.branchPR == 0 && len(rows.val) > 0 {
		p.selected = filterPulls(rows.val, filterAll, "")[0].Number
	}
	share := p.slide.at(gtx, !p.listHidden)
	size := gtx.Constraints.Max
	w := gtx.Dp(pullsListWidth)
	slot := int(share * float32(w+gtx.Dp(gutter)))
	if slot > 0 {
		clipped := clip.Rect{Max: image.Pt(slot, size.Y)}.Push(gtx.Ops)
		offset := op.Offset(image.Pt(slot-w-gtx.Dp(gutter), 0)).Push(gtx.Ops)
		list := gtx
		list.Constraints = layout.Exact(image.Pt(w, size.Y))
		pullsIsland(list, th, th.Sidebar, func(gtx C, th *ui.Theme) D { return p.listLayout(gtx, th, rows) })
		offset.Pop()
		clipped.Pop()
	}
	defer op.Offset(image.Pt(slot, 0)).Push(gtx.Ops).Pop()
	view := gtx
	view.Constraints = layout.Exact(image.Pt(max(size.X-slot, 0), size.Y))
	pullsIsland(view, th, th.Background, func(gtx C, th *ui.Theme) D { return p.view.layout(gtx, th, p, &act) })
	return act
}

// pullsIsland is a rounded-lg box of bg the size of gtx.
func pullsIsland(gtx C, th *ui.Theme, bg color.NRGBA, w func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	ui.Fill(gtx, bg, ui.RadiusLG)
	return w(gtx, th.On(bg))
}

// checkedOut is the checkout holding branch, ok false when none does.
func (p *pullsScreen) checkedOut(branch string) (lichclient.Worktree, bool) {
	for _, c := range p.store.checkoutsOf(p.project.Path) {
		if c.Name == branch {
			return c, true
		}
	}
	return lichclient.Worktree{}, false
}
