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
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
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
	icPullBack    = icons.Lucide("chevron-left")
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

// pullsScreen is the Pulls screen over the stage, like Settings: the index of
// the project's pull requests, or one of them across the whole island. It reads through the window's pullsStore and
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
	selected int // the PR in view, 0 for the index
	// branchPR is the PR of the checkout's branch, which the screen opens on.
	branchPR int
	branch   string // the checkout's branch, which the index's New names
	state    pullsState
	filter   ui.TabsState
	// cursor is the index row the keyboard is on, by number; it is drawn
	// only once the keyboard moved it, so a mouse user sees no selection.
	cursor int
	keyed  bool

	query      widget.Editor
	clearQuery widget.Clickable
	stateMenu  ui.MenuState
	stateClick widget.Clickable
	create     widget.Clickable
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

// layout draws the island across gtx and answers what was asked of the
// window; th is on the window's canvas.
func (p *pullsScreen) layout(gtx C, th *ui.Theme) pullsAction {
	var act pullsAction
	gtx.Constraints.Min = gtx.Constraints.Max
	if p.focused {
		gtx.Execute(op.InvalidateCmd{At: p.recheck(gtx.Now)})
	}
	rows := p.store.list(p.project.Path, p.state)
	p.keys(gtx, rows.val)
	for n, click := range p.rowClicks {
		if p.selected == 0 && click.Clicked(gtx) {
			p.keyed = false
			p.openPR(n)
		}
	}
	if p.selected != 0 {
		p.view.navClicks(gtx, p, p.visible(rows.val))
	}
	p.spans = append(p.spans[:0], islandSpan{x0: 0, x1: gtx.Constraints.Max.X, color: th.Background})
	pullsIsland(gtx, th, th.Background, p.squareX == 0, func(gtx C, th *ui.Theme) D {
		if p.selected == 0 {
			return p.indexLayout(gtx, th, rows, &act)
		}
		return p.view.layout(gtx, th, p, &act)
	})
	return act
}

// visible is the index's rows under its filter and query, the order the
// detail's previous and next walk.
func (p *pullsScreen) visible(rows []backend.PRSummary) []backend.PRSummary {
	return filterPulls(rows, pullsFilter(p.filter.Selected), p.query.Text())
}

// openPR shows pull request n, the index's cursor following it. Both it and
// backToIndex ask for a frame: a menu's choice lands after the frame drew.
func (p *pullsScreen) openPR(n int) {
	p.selected, p.cursor = n, n
	p.store.invalidate()
}

// backToIndex leaves the pull request for the index, its filter, query and
// cursor as they were.
func (p *pullsScreen) backToIndex() {
	if p.selected != 0 {
		p.cursor = p.selected
	}
	p.selected = 0
	p.store.invalidate()
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
