package main

import (
	"image"
	"image/color"
	"slices"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// pullsListWidth is PullsList's w-80. The list grows past it, up to
// pullsListMaxWidth, when the filter tabs need more: Portuguese and Spanish
// labels do not fit in 320.
var (
	pullsListWidth    = unit.Dp(320)
	pullsListMaxWidth = unit.Dp(400)
)

var (
	icPullMerged  = icons.Lucide("git-merge")
	icPullClosed  = icons.Lucide("git-pull-request-closed")
	icPullBranch  = icons.Lucide("git-branch")
	icPullCheck   = icons.Lucide("check")
	icPullX       = icons.Lucide("x")
	icPullRunning = icons.Lucide("loader-circle")
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

	open bool
	// focused is whether the window has the user's focus.
	focused bool
	// squareX is the left edge of the island the active project tab sits on
	// the top-left corner of, -1 for none: that island drops the rounding for
	// the tab to grow out of, as the stage's island does.
	squareX int
	// spans are the islands as the last layout placed them, for the tab
	// strip to merge the active tab into the one it sits over.
	spans    []islandSpan
	project  backend.Project
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
	// files is what the Files tab draws with and sends through.
	files filesInput
	// sessionsIn is the project's sessions sitting in a checkout.
	sessionsIn func(projectID, path string) []string
	// sessionLabel names a session.
	sessionLabel func(id string) string
	// waiting is whether a review thread waits for the push that resolves it.
	waiting func(threadID string) bool
}

// reviewSession is where the pull request's comments go: a session in the
// checkout of its head branch, the window's active one when it sits there and
// the first otherwise, "" when none does. target keys its comment batch: that checkout, shared with its
// dock, or the PR's page while the branch has none.
func (p *pullsScreen) reviewSession(d backend.PRDetail) (id, target string) {
	wt, ok := p.checkedOut(d.HeadRefName)
	if !ok {
		return "", d.URL
	}
	ids := p.sessionsIn(p.project.ID, wt.Path)
	switch {
	case slices.Contains(ids, p.files.activeID):
		return p.files.activeID, wt.Path
	case len(ids) > 0:
		return ids[0], wt.Path
	}
	return "", wt.Path
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
func (p *pullsScreen) show(project backend.Project, branch string, branchPR int) {
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

// setFocused follows the window's focus: coming back to it reads again what
// the screen shows, which may have moved on GitHub meanwhile.
func (p *pullsScreen) setFocused(focused bool) {
	if focused && !p.focused && p.open {
		p.store.stale()
	}
	p.focused = focused
}

// prRecheck is how long the Pulls screen believes what it shows before reading
// it again.
const prRecheck = time.Minute

// recheck owes what the screen shows a newer answer once prRecheck has passed
// since the last one: a push or a merge made outside lich announces nothing.
// It answers when the next one is due.
func (p *pullsScreen) recheck(now time.Time) time.Time {
	due := p.store.lastStale().Add(prRecheck)
	if now.Before(due) {
		return due
	}
	p.store.stale()
	return now.Add(prRecheck)
}

// layout draws the two islands across gtx and answers what was asked of the
// window; th is on the window's canvas.
func (p *pullsScreen) layout(gtx C, th *ui.Theme) pullsAction {
	var act pullsAction
	gtx.Constraints.Min = gtx.Constraints.Max
	if p.focused {
		gtx.Execute(op.InvalidateCmd{At: p.recheck(gtx.Now)})
	}
	rows := p.store.list(p.project.Path, p.state)
	if p.selected == 0 && p.branchPR == 0 && len(rows.val) > 0 {
		p.selected = filterPulls(rows.val, filterAll, "")[0].Number
	}
	// Before the slide reads listHidden, so it starts in the frame that
	// took the click.
	for p.toggleList.click.Clicked(gtx) {
		p.listHidden = true
	}
	for p.view.showList.click.Clicked(gtx) {
		p.listHidden = false
	}
	share := p.slide.at(gtx, !p.listHidden)
	size := gtx.Constraints.Max
	w := p.listWidth(gtx, th, filterCounts(rows.val, p.query.Text()))
	slot := int(share * float32(w+gtx.Dp(gutter)))
	p.spans = p.spans[:0]
	if slot > 0 {
		p.spans = append(p.spans, islandSpan{x0: slot - w - gtx.Dp(gutter), x1: slot - gtx.Dp(gutter), color: th.Sidebar})
	}
	p.spans = append(p.spans, islandSpan{x0: slot, x1: size.X, color: th.Background})
	if slot > 0 {
		clipped := clip.Rect{Max: image.Pt(slot, size.Y)}.Push(gtx.Ops)
		offset := op.Offset(image.Pt(slot-w-gtx.Dp(gutter), 0)).Push(gtx.Ops)
		list := gtx
		list.Constraints = layout.Exact(image.Pt(w, size.Y))
		pullsIsland(list, th, th.Sidebar, p.squareX == slot-w-gtx.Dp(gutter), func(gtx C, th *ui.Theme) D { return p.listLayout(gtx, th, rows) })
		offset.Pop()
		clipped.Pop()
	}
	defer op.Offset(image.Pt(slot, 0)).Push(gtx.Ops).Pop()
	view := gtx
	view.Constraints = layout.Exact(image.Pt(max(size.X-slot, 0), size.Y))
	pullsIsland(view, th, th.Background, p.squareX == slot, func(gtx C, th *ui.Theme) D { return p.view.layout(gtx, th, p, &act) })
	return act
}

// pullsIsland is a rounded-lg box of bg the size of gtx, its top-left corner
// square when the active tab grows out of it.
func pullsIsland(gtx C, th *ui.Theme, bg color.NRGBA, squareNW bool, w func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	r := gtx.Dp(ui.RadiusLG)
	nw := r
	if squareNW {
		nw = 0
	}
	paint.FillShape(gtx.Ops, bg, clip.RRect{Rect: image.Rectangle{Max: gtx.Constraints.Max}, NW: nw, NE: r, SE: r, SW: r}.Op(gtx.Ops))
	return w(gtx, th.On(bg))
}

// checkedOut is the checkout holding branch, ok false when none does.
func (p *pullsScreen) checkedOut(branch string) (backend.Worktree, bool) {
	for _, c := range p.store.checkoutsOf(p.project.Path) {
		if c.Name == branch {
			return c, true
		}
	}
	return backend.Worktree{}, false
}
