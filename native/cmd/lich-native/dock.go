package main

// The right dock: the panel at the terminal's right, on its Code, its Review
// or its Artifacts tab, and the tab it was last on when the window opens. It
// never closes: it stands beside the terminal or, collapsed, comes out from
// the window's edge. It owns the chrome around the bodies: the drag-resizable
// width (one, persisted, for every tab), full screen, collapse and the pill of
// icon tabs at its foot.
//
// The Review tab is a body the dock hands the frame to, through reviewTab:
//
//	type reviewTab interface {
//		layoutBody(gtx C, th *ui.Theme, in dockInput) error // the panel under the header
//		layoutActions(gtx C, th *ui.Theme) D                // hide whitespace, collapse all
//		hidden()                                            // off screen: stop polling
//		holdsKeys(gtx C) bool                               // a field of its own has focus
//	}
//
// layoutActions is drawn at the header's right, before collapse, only while
// the Review tab is on and the checkout has changes. th is the header's: the
// actions sit on the sidebar surface. holdsKeys keeps the terminal, which takes
// focus every frame it is not told otherwise, off a field of the body; the
// terminal asks before the dock is laid out, so a flag the body sets during its
// own layout answers one frame late, and that frame is the one the terminal
// takes the focus back in. iconAction is what both halves draw them with, and
// notice is the panels' quiet line.

import (
	"context"
	"image"
	"math"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	driver "github.com/lichdotdev/gio-driver"
	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icDockCode     = icons.Lucide("code")
	icDockReview   = icons.Lucide("file-diff")
	icDockMaximize = icons.Lucide("maximize-2")
	icDockMinimize = icons.Lucide("minimize-2")
	icDockClose    = icons.Lucide("x")
	icDockCollapse = icons.Lucide("panel-right-close")
	icDockExpand   = icons.Lucide("panel-right")
)

// The dock's width key, a backend setting, and its bounds in rem: wide enough for a diff line at the
// review's 12px font, short of eating the terminal whole.
const (
	dockWidthKey = "lich.dock.width"
	// dockCollapsedKey stores whether the dock comes out from the window's
	// edge rather than standing beside the terminal.
	dockCollapsedKey = "lich.dock.collapsed"
	// dockTabKey stores the tab the dock opens on.
	dockTabKey     = "lich.dock.tab"
	dockMinRem     = 20
	dockMaxRem     = 60
	dockDefaultRem = 28
	// dockHeaderHeight is the strip the actions sit in, 8dp from the top and
	// from the Separator under it, the inset the sidebar's header keeps.
	dockHeaderHeight = unit.Dp(44)
)

type dockTab uint8

// dockClosed is no tab, before the stored one is read. The dock itself never
// closes.
const (
	dockClosed dockTab = iota
	dockCode
	dockReview
	dockArtifacts
	// dockExtension is an extension's dock surface, the one dock.ext names.
	dockExtension
)

// dockBuiltinTabs is how many tabs lich's own are, before the extensions'.
const dockBuiltinTabs = int(dockArtifacts)

// dockHeaderSpare is what the header needs past its actions: its two insets
// and gaps, and a gap's room to their left.
var dockHeaderSpare = ui.Space(8)

// dockInput is what the dock reads of the active session each frame. checkout
// is where the session works now (useActiveSession's path), which is what both
// tabs browse and review; git is its status, isGit false outside a repository.
// light is the palette on screen, for what draws with its own theme.
type dockInput struct {
	sessionID string
	inject    func(string) bool
	submit    func(string, func(error)) bool
	checkout  string
	git       backend.DiffStats
	isGit     bool
	light     bool
	code      godemirror.Theme
	// placed is the extensions' dock surfaces, a tab each after lich's.
	placed []placedSurface
}

type reviewTab interface {
	layoutBody(gtx C, th *ui.Theme, in dockInput) error
	layoutActions(gtx C, th *ui.Theme) D
	hidden()
	holdsKeys(gtx C) bool
}

type dock struct {
	// tour is the window's guided tour, which points at the dock's tabs and
	// body.
	tour *driver.Tour
	tab  dockTab
	// shown is the tab the dock last opened on, which it keeps drawing while
	// it slides out.
	shown      dockTab
	slide      dockSlide
	widthRem   float64
	fullscreen bool
	resize     ui.ResizeHandle
	full       ui.IconActionState
	collapse   ui.IconActionState
	// collapsed is the dock off the stage, out over it from the window's
	// right edge (peek); saveCollapsed is a click on collapse to store.
	collapsed     bool
	saveCollapsed bool
	// saved is the tab last stored, which a change of tab is told from.
	saved     dockTab
	peek      dockPeek
	code      *codeTab
	review    reviewTab
	artifacts *artifactsTab
	// ext is the extension surface of the dockExtension tab, drawn by
	// extensions; extAct is what its failed state asked for this frame.
	ext        extensionKey
	extensions *extensionScreen
	extAct     extensionAction
	// tabButtons are the pill's buttons, one per tab; placed the extension
	// tabs they were drawn for, which actTabs picks from.
	tabButtons []*dockTabButton
	placed     []placedSurface
	// actionsWidth and tabBarWidth are the header's actions and the pill with
	// its insets, which fitTabs reads once both are laid out.
	actionsWidth, tabBarWidth int
	// minRem is the width the header's actions and the pill need, measured on
	// the last frame: the dock is never narrower, so every tab stays in reach.
	minRem float64
}

func newDock(code *codeTab, review reviewTab, artifacts *artifactsTab, extensions *extensionScreen) *dock {
	return &dock{tour: new(driver.Tour), widthRem: dockDefaultRem, minRem: dockMinRem, resize: ui.ResizeHandle{Edge: ui.PanelEdgeLeft}, code: code, review: review, artifacts: artifacts, extensions: extensions}
}

// showsExtension is whether the dock is open on surface k.
func (d *dock) showsExtension(k extensionKey) bool { return d.tab == dockExtension && d.ext == k }

// showExtension opens the dock on surface k. A collapsed dock comes out.
func (d *dock) showExtension(k extensionKey) {
	d.tab, d.ext = dockExtension, k
	d.bringOut()
}

// showing is whether the dock is on screen on t.
func (d *dock) showing(t dockTab) bool { return d.tab == t && d.visible() }

// takeExtensionAction is what the extension tab's failed state asked for,
// once.
func (d *dock) takeExtensionAction() extensionAction {
	act := d.extAct
	d.extAct = extensionAction{}
	return act
}

// holdsKeys reports whether a field of the dock has focus. The terminal asks
// before the dock is laid out, so the Code tab answers from the focus as it
// stands rather than a flag its last layout left.
func (d *dock) holdsKeys(gtx C) bool {
	switch d.tab {
	case dockCode:
		return d.code.focused(gtx)
	case dockReview:
		return d.review.holdsKeys(gtx)
	case dockArtifacts:
		return d.artifacts.holdsKeys()
	case dockExtension:
		return d.extensions.holdsKeys(gtx, d.ext)
	}
	return false
}

// width is the dock's width beside the terminal, open or not.
func (d *dock) width(gtx C) int {
	return gtx.Dp(unit.Dp(max(d.widthRem, d.minRem) * remDp))
}

// layout draws the dock in gtx.Constraints.Max: its column beside the
// terminal, or the whole row in full screen. It reports
// whether a drag of its width ended, the moment to persist it.
func (d *dock) layout(gtx C, th *ui.Theme, root *ui.Root, in dockInput) (saveWidth bool, err error) {
	if d.tab != dockCode {
		d.code.hidden()
	}
	if d.tab != dockReview {
		d.review.hidden()
	}
	if d.tab == dockExtension {
		if _, ok := findPlaced(in.placed, d.ext); !ok {
			d.tab = dockReview
		}
	}
	if !d.fullscreen {
		w, released := d.resize.Update(gtx, unit.Dp(max(d.widthRem, d.minRem)*remDp), unit.Dp(d.minRem*remDp), dockMaxRem*remDp)
		d.widthRem, saveWidth = float64(w)/remDp, released
	}
	size := gtx.Constraints.Max
	gtx.Constraints = layout.Exact(size)
	if !d.fullscreen {
		// In the gap between the stage and the dock, outside the dock's own
		// clip.
		d.resize.LayoutGap(gtx, gtx.Dp(gutter))
	}
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops).Pop()
	ui.Fill(gtx, th.Sidebar, 0)
	on := th.On(th.Sidebar)
	rr := gtx.Dp(ui.RadiusLG)
	defer ui.IslandBorder(gtx, size, rr, on.Over(on.Border))
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return d.header(gtx, on, root, in) }),
		layout.Rigid(on.Separator),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			return d.tour.Anchor(gtx, tourReviewPanel, func(gtx C) D {
				err = d.body(gtx, on, root, in)
				return D{Size: gtx.Constraints.Max}
			})
		}),
		layout.Rigid(func(gtx C) D { return d.tabBar(gtx, on, root, in) }),
	)
	d.fitTabs(gtx, max(d.actionsWidth+gtx.Dp(dockHeaderSpare), d.tabBarWidth))
	return saveWidth, err
}

func (d *dock) body(gtx C, th *ui.Theme, root *ui.Root, in dockInput) error {
	switch d.tab {
	case dockReview:
		return d.review.layoutBody(gtx, th, in)
	case dockArtifacts:
		d.artifacts.layout(gtx, th, root, in.sessionID)
		return nil
	case dockExtension:
		d.extAct = d.extensions.placed(gtx, th, placeDock, d.ext)
		return nil
	}
	d.code.layout(gtx, th, in)
	return nil
}

// header is the h-10 strip: at its right the Review tab's actions, collapse
// and full screen. The tabs are the pill at the dock's foot.
func (d *dock) header(gtx C, th *ui.Theme, root *ui.Root, in dockInput) D {
	changed := in.isGit && in.git.Files > 0
	actions := 0
	fullIcon, fullLabel := icDockMaximize, i18n.T("dock.rightDock.fullScreen")
	if d.fullscreen {
		fullIcon, fullLabel = icDockMinimize, i18n.T("dock.rightDock.exitFullScreen")
	}
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(dockHeaderHeight)))
	dims := layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				if d.tab != dockReview || !changed {
					return D{}
				}
				dims := d.review.layoutActions(gtx, th)
				actions += dims.Size.X
				return dims
			}),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D {
				if d.fullscreen {
					return D{}
				}
				icon, label := icDockCollapse, i18n.T("dock.rightDock.collapsePanel")
				if d.collapsed {
					icon, label = icDockExpand, i18n.T("dock.rightDock.expandPanel")
				}
				dims := th.IconAction(root, &d.collapse, icon, label, false).Layout(gtx)
				actions += dims.Size.X
				return dims
			}),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D {
				dims := th.IconAction(root, &d.full, fullIcon, fullLabel, false).Layout(gtx)
				actions += dims.Size.X
				return dims
			}),
		)
	})
	d.actionsWidth = actions
	return dims
}

// fitTabs keeps the dock at least as wide as its header's actions and the
// pill of tabs need, px wide, and redraws when that changes.
func (d *dock) fitTabs(gtx C, px int) {
	need := max(dockMinRem, float64(gtx.Metric.PxToDp(px))/remDp)
	if need != d.minRem {
		d.minRem = need
		gtx.Execute(op.InvalidateCmd{})
	}
}

// selectedTab is the index of the tab on screen among lich's and then the
// extensions'.
func (d *dock) selectedTab(placed []placedSurface) int {
	if d.tab != dockExtension {
		return int(d.tab) - 1
	}
	for i, p := range placed {
		if p.key == d.ext {
			return dockBuiltinTabs + i
		}
	}
	return -1
}

// pickTab opens the tab at index, as selectedTab counts them.
func (d *dock) pickTab(index int, placed []placedSurface) {
	if index < dockBuiltinTabs {
		d.tab = dockTab(index + 1)
		return
	}
	if i := index - dockBuiltinTabs; i < len(placed) {
		d.tab, d.ext = dockExtension, placed[i].key
	}
}

// act runs the header's and the pill's clicks before the stage row reads whether the dock
// is open and how wide, so the frame shows their outcome.
func (d *dock) act(gtx C) {
	d.actTabs(gtx)
	for d.full.Click.Clicked(gtx) {
		d.fullscreen = !d.fullscreen
	}
	for d.collapse.Click.Clicked(gtx) {
		d.setCollapsed(!d.collapsed)
	}
}

// parseDockWidth parses a stored width within the dock's bounds: garbage,
// negatives and an unset value fall back to the default.
func parseDockWidth(raw string) float64 {
	rem, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsInf(rem, 0) || math.IsNaN(rem) || rem <= 0 {
		return dockDefaultRem
	}
	return min(dockMaxRem, max(dockMinRem, rem))
}

func loadDockWidth(ctx context.Context, c *backend.Client) (float64, error) {
	raw, err := c.GetSetting(ctx, dockWidthKey, "")
	return parseDockWidth(raw), err
}

func saveDockWidth(ctx context.Context, c *backend.Client, rem float64) error {
	return c.SetSetting(ctx, dockWidthKey, "", strconv.FormatFloat(rem, 'f', -1, 64))
}

func loadDockCollapsed(ctx context.Context, c *backend.Client) (bool, error) {
	raw, err := c.GetSetting(ctx, dockCollapsedKey, "")
	return parseBoolSetting(raw, false), err
}

func saveDockCollapsed(ctx context.Context, c *backend.Client, collapsed bool) error {
	return c.SetSetting(ctx, dockCollapsedKey, "", strconv.FormatBool(collapsed))
}

// dockTabNames are the tabs stored by name. An extension's tab is not stored:
// its surface is placed only once the extensions report in, after the first
// frames, and a dock that fell back to Review meanwhile would store that.
var dockTabNames = map[dockTab]string{dockCode: "code", dockReview: "review", dockArtifacts: "artifacts"}

// parseDockTab reads a stored tab; anything else, unset included, is Review.
func parseDockTab(raw string) dockTab {
	for t, name := range dockTabNames {
		if name == raw {
			return t
		}
	}
	return dockReview
}

func loadDockTab(ctx context.Context, c *backend.Client) (dockTab, error) {
	raw, err := c.GetSetting(ctx, dockTabKey, "")
	return parseDockTab(raw), err
}

func saveDockTab(ctx context.Context, c *backend.Client, t dockTab) error {
	return c.SetSetting(ctx, dockTabKey, "", dockTabNames[t])
}

// restore opens the dock on t as the window starts, stored and seen by the
// peek already, so a collapsed dock stays at the edge.
func (d *dock) restore(t dockTab) {
	d.tab, d.saved, d.peek.tab = t, t, t
}

// tabToSave reports lich's tab on screen once it differs from the one stored,
// and takes it as stored.
func (d *dock) tabToSave() (dockTab, bool) {
	if dockTabNames[d.tab] == "" || d.tab == d.saved {
		return 0, false
	}
	d.saved = d.tab
	return d.tab, true
}
