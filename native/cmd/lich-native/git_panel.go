package main

// The Git panel: the region between the stage and the footer, opened from the
// footer, that reads the repository of the active session. Log, the only tab
// for now, is the branch tree, the history and the selected commit's detail.

import (
	"context"
	"image"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The panel's height key, a backend setting, and its bounds in rem. The floor
// keeps a few rows of the log; gitStageFloorRem is what it leaves the stage.
const (
	gitHeightKey     = "lich.git.height"
	gitMinRem        = 12
	gitMaxRem        = 60
	gitDefaultRem    = 19
	gitStageFloorRem = 10

	gitTreeWidthKey   = "lich.git.tree.width"
	gitDetailWidthKey = "lich.git.detail.width"
	gitMessageKey     = "lich.git.detail.message"
	// gitFilesFloorRem is what dragging the message up leaves the file list.
	gitFilesFloorRem = 5
	// gitLogFloorRem is what dragging either side column leaves the log.
	gitLogFloorRem = 18
)

// gitColumn is a side column whose width is dragged and kept.
type gitColumn struct {
	key                 string
	rem, minRem, maxRem float64
	resize              ui.ResizeHandle
}

// width is the column's width in a body with room to spare for it, reading the
// frame's drag; released says the drag ended.
func (c *gitColumn) width(gtx C, room unit.Dp) (px int, released bool) {
	ceiling := max(min(float64(room), c.maxRem*remDp), c.minRem*remDp)
	got, released := c.resize.Update(gtx, unit.Dp(c.rem*remDp), unit.Dp(c.minRem*remDp), unit.Dp(ceiling))
	c.rem = min(float64(got), ceiling) / remDp
	return gtx.Dp(unit.Dp(c.rem * remDp)), released
}

// gitPanelInput is what the panel reads of the window each frame. sessions
// are the project's, by the branch they are on.
type gitPanelInput struct {
	path     string
	sessions map[string]sessionView
	// here is the session working in path, nil for none: a checkout there
	// changes the branch under its agent.
	here *sessionView
	code godemirror.Theme
}

// gitRowState is a list row's clickable and its tooltip.
type gitRowState struct {
	click         widget.Clickable
	tip, moreTip  ui.Tooltip
	more          widget.Clickable
	menu, context ui.MenuState
}

type gitPanel struct {
	root   *ui.Root
	store  *gitStore
	family string
	size   unit.Sp

	open      bool
	heightRem float64
	resize    ui.ResizeHandle
	close     ui.IconActionState
	full      ui.IconActionState
	expanded  bool
	// slide opens the panel and grow expands it, each on the dock's curve.
	slide, grow dockSlide
	// covers is a frame where the panel has finished expanding and takes the
	// stage's place; pendingPx is the height the stage has yet to give it.
	covers    bool
	pendingPx int
	path      string
	treeCol   gitColumn
	detailCol gitColumn
	// messageCol is the commit message's height under the file list.
	messageCol gitColumn

	filter                gitLogFilter
	query, file           widget.Editor
	typedQuery, typedFile string
	typedAt               time.Time
	sinceDays             int
	authorMenu, dateMenu  ui.MenuState
	author, date          widget.Clickable

	treeField widget.Editor
	tree      widget.List
	treeRows  map[string]*gitRowState

	log      widget.List
	rows     map[string]*gitRowState
	selected string

	detail gitDetailPane
	branchOps
}

func newGitPanel(ctx context.Context, client *backend.Client, root *ui.Root, invalidate func(), notify func(ui.Toast), family string, size unit.Sp) *gitPanel {
	g := &gitPanel{root: root, family: family, size: size, heightRem: gitDefaultRem,
		branchOps:  branchOps{notify: notify, invalidate: invalidate},
		resize:     ui.ResizeHandle{Edge: ui.PanelEdgeTop},
		treeCol:    gitColumn{key: gitTreeWidthKey, rem: 12.5, minRem: 9, maxRem: 32, resize: ui.ResizeHandle{Edge: ui.PanelEdgeRight}},
		detailCol:  gitColumn{key: gitDetailWidthKey, rem: 16.25, minRem: 12, maxRem: 40, resize: ui.ResizeHandle{Edge: ui.PanelEdgeLeft}},
		messageCol: gitColumn{key: gitMessageKey, rem: 7.5, minRem: 4, maxRem: 40, resize: ui.ResizeHandle{Edge: ui.PanelEdgeTop}},
		store:      &gitStore{ctx: ctx, client: client, invalidate: invalidate},
		treeRows:   map[string]*gitRowState{}, rows: map[string]*gitRowState{},
	}
	g.tree.Axis, g.log.Axis = layout.Vertical, layout.Vertical
	g.detail.init()
	return g
}

// moved is whether this frame began or ended a slide, when the terminal takes
// its new size at once. A window built without the panel has none.
func (g *gitPanel) moved() bool {
	return g != nil && (g.slide.began || g.slide.landed || g.grow.began || g.grow.landed)
}

// pending is the height the stage has yet to give the panel, 0 without one.
func (g *gitPanel) pending() int {
	if g == nil {
		return 0
	}
	return g.pendingPx
}

// hidden stops the reads of a panel off the screen.
func (g *gitPanel) hidden() { g.store.close() }

func (g *gitPanel) holdsKeys(gtx C) bool {
	if !g.open {
		return false
	}
	return gtx.Focused(&g.query) || gtx.Focused(&g.file) || gtx.Focused(&g.treeField) || g.authorMenu.Opened() || g.dateMenu.Opened() ||
		g.editing != nil || g.menuOpen
}

// height is what the panel takes of a column maxY tall, reading the frame's
// drag; released says the drag ended, the moment to store it.
func (g *gitPanel) height(gtx C, maxY int) (h int, released bool) {
	ceiling := min(float32(gitMaxRem*remDp), float32(gtx.Metric.PxToDp(maxY))-gitStageFloorRem*remDp)
	floor := float32(gitMinRem * remDp)
	ceiling = max(ceiling, floor)
	got, released := g.resize.Update(gtx, unit.Dp(g.heightRem*remDp), unit.Dp(floor), unit.Dp(ceiling))
	g.heightRem = min(float64(got)/remDp, float64(ceiling)/remDp)
	return gtx.Dp(unit.Dp(g.heightRem * remDp)), released
}

// act runs the header's clicks before the window reads whether the panel is
// open and how much it covers, so the frame shows their outcome.
func (g *gitPanel) act(gtx C) {
	for g.close.Click.Clicked(gtx) {
		g.open = false
	}
	for g.full.Click.Clicked(gtx) {
		g.expanded = !g.expanded
	}
}

// layout draws the panel in gtx.Constraints.Max.
func (g *gitPanel) layout(gtx C, th *ui.Theme, in gitPanelInput) {
	if in.path != g.path {
		g.path, g.selected = in.path, ""
		g.filter.ref = ""
		g.detail.closeFile()
	}
	g.applyTyped(gtx)
	g.store.follow(in.path, g.filter)
	view := g.store.snapshot()

	size := gtx.Constraints.Max
	gtx.Constraints = layout.Exact(size)
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops).Pop()
	ui.Fill(gtx, th.Sidebar, 0)
	on := th.On(th.Sidebar)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return g.header(gtx, on) }),
		layout.Rigid(on.Separator),
		layout.Flexed(1, func(gtx C) D { return g.body(gtx, on, view, in) }),
	)
	if !g.expanded {
		g.resize.Layout(gtx, on)
	}
}

func (g *gitPanel) header(gtx C, th *ui.Theme) D {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(dockHeaderHeight)))
	return layout.Inset{Left: ui.Space(3.5), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextSM, i18n.T("git.panel.log")).Layout),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				ic, label := icDockMaximize, i18n.T("dock.rightDock.fullScreen")
				if g.expanded {
					ic, label = icDockMinimize, i18n.T("dock.rightDock.exitFullScreen")
				}
				return th.IconAction(g.root, &g.full, ic, label, false).Layout(gtx)
			}),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D {
				return th.IconAction(g.root, &g.close, icDockClose, i18n.T("git.panel.close"), false).Layout(gtx)
			}),
		)
	})
}

func (g *gitPanel) body(gtx C, th *ui.Theme, view gitLogView, in gitPanelInput) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	size := gtx.Constraints.Max
	spare := gtx.Metric.PxToDp(size.X) - unit.Dp(gitLogFloorRem*remDp)
	treeWidth, treeDone := g.treeCol.width(gtx, spare-unit.Dp(g.detailCol.rem*remDp))
	detailWidth, detailDone := g.detailCol.width(gtx, spare-unit.Dp(g.treeCol.rem*remDp))
	if treeDone {
		g.saveColumn(g.treeCol)
	}
	if detailDone {
		g.saveColumn(g.detailCol)
	}
	column := func(width int, w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(width, gtx.Constraints.Max.Y))
			return w(gtx)
		})
	}
	rule := layout.Rigid(func(gtx C) D { return th.VerticalRule(gtx) })
	dims := layout.Flex{}.Layout(gtx,
		column(treeWidth, func(gtx C) D { return g.treeColumn(gtx, th, view, in) }),
		rule,
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			if g.detail.diffOpen() {
				return g.detail.layoutDiff(gtx, th, g, in)
			}
			return g.logColumn(gtx, th, view, in)
		}),
		rule,
		column(detailWidth, func(gtx C) D { return g.detailColumn(gtx, th, in) }),
	)
	g.columnHandles(gtx, th, treeWidth, detailWidth)
	return dims
}

// columnHandles lays the two handles out over what the columns drew. The
// tree's sits past its rule, over the log's margin: on the tree's own edge its
// lit strip would cover the tree's scrollbar.
func (g *gitPanel) columnHandles(gtx C, th *ui.Theme, treeWidth, detailWidth int) {
	size := gtx.Constraints.Max
	tree := gtx
	tree.Constraints = layout.Exact(image.Pt(treeWidth+gtx.Dp(1)+gtx.Dp(ui.ResizeWidth), size.Y))
	g.treeCol.resize.Layout(tree, th)
	defer op.Offset(image.Pt(size.X-detailWidth, 0)).Push(gtx.Ops).Pop()
	detail := gtx
	detail.Constraints = layout.Exact(image.Pt(detailWidth, size.Y))
	g.detailCol.resize.Layout(detail, th)
}

func (g *gitPanel) saveColumn(c gitColumn) {
	rem := strconv.FormatFloat(c.rem, 'f', -1, 64)
	go func() {
		if err := g.store.client.SetSetting(g.store.ctx, c.key, "", rem); err != nil {
			log.Printf("git panel width: %v", err)
		}
	}()
}

// loadColumns reads the stored widths; an unset or unreadable one keeps the
// default.
func (g *gitPanel) loadColumns(ctx context.Context, c *backend.Client) error {
	for _, col := range []*gitColumn{&g.treeCol, &g.detailCol, &g.messageCol} {
		raw, err := c.GetSetting(ctx, col.key, "")
		if err != nil {
			return err
		}
		col.rem = parseGitRem(raw, col.rem, col.minRem, col.maxRem)
	}
	return nil
}

// gitNotice is the quiet line a pane shows instead of its content, on the
// rows' 14px edge.
func gitNotice(gtx C, th *ui.Theme, text string) D {
	return th.PanelNotice(text, ui.Space(3.5)).Layout(gtx)
}

// rowFill paints a row's selection or hover, the window's lists' language.
func rowFill(gtx C, th *ui.Theme, selected, hovered bool) {
	switch {
	case selected:
		ui.Fill(gtx, th.Over(th.Accent), ui.RadiusMD)
	case hovered:
		ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, hoverFill)), ui.RadiusMD)
	}
}

// gitScroll is a panel list with its bar over the rows.
func gitScroll(th *ui.Theme, state *widget.List) material.ListStyle {
	list := th.ScrollArea(state)
	list.AnchorStrategy = material.Overlay
	return list
}

func gitRow(rows map[string]*gitRowState, key string) *gitRowState {
	r, ok := rows[key]
	if !ok {
		r = new(gitRowState)
		rows[key] = r
	}
	return r
}

// gitCleanLine is text from a commit made fit for one label: the fonts have no
// glyph for a tab, and a stray newline would grow the row.
func gitCleanLine(s string) string {
	return strings.NewReplacer("\t", "    ", "\n", " ", "\r", "").Replace(s)
}

// gitSessionRing marks a branch a session is on, colored by what it is doing.
func gitSessionRing(gtx C, th *ui.Theme, status string) D {
	size := gtx.Dp(ui.Space(3))
	stroke := float32(gtx.Dp(sessionTreeStroke))
	tone := th.MutedForeground
	switch status {
	case "waiting":
		tone = th.ToneWait
	case "done":
		tone = th.TonePass
	}
	center := float32(size) / 2
	ui.StrokeArc(gtx, f32.Pt(center, center), center-stroke/2, stroke, 0, 2*math.Pi, tone)
	return D{Size: image.Pt(size, size)}
}

// sessionMark is the session's ring with its name in a tooltip.
func (g *gitPanel) sessionMark(gtx C, th *ui.Theme, r *gitRowState, sv sessionView) D {
	tip := th.Tooltip(g.root, &r.tip, sv.Label)
	return tip.Layout(gtx, func(gtx C) D { return gitSessionRing(gtx, th, sv.status) })
}

func parseGitHeight(raw string) float64 {
	return parseGitRem(raw, gitDefaultRem, gitMinRem, gitMaxRem)
}

// parseGitRem is a stored size within its bounds: garbage, negatives and an
// unset value fall back to the default.
func parseGitRem(raw string, fallback, lo, hi float64) float64 {
	rem, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsInf(rem, 0) || math.IsNaN(rem) || rem <= 0 {
		return fallback
	}
	return min(hi, max(lo, rem))
}

func loadGitHeight(ctx context.Context, c *backend.Client) (float64, error) {
	raw, err := c.GetSetting(ctx, gitHeightKey, "")
	return parseGitHeight(raw), err
}

func saveGitHeight(ctx context.Context, c *backend.Client, rem float64) error {
	return c.SetSetting(ctx, gitHeightKey, "", strconv.FormatFloat(rem, 'f', -1, 64))
}
