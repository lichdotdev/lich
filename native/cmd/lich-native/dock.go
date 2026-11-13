package main

// The right dock: the panel at the terminal's right that the
// footer's Browse code, Review changes and Artifacts buttons open, on its
// Code, its Review or its Artifacts tab. It owns the chrome around both bodies: the drag-resizable width
// (one, persisted, for every tab), full screen, the tab bar and the close
// button.
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
// layoutActions is drawn at the header's right, before full screen, only while
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
)

// The dock's width key, a backend setting, and its bounds in rem: wide enough for a diff line at the
// review's 12px font, short of eating the terminal whole.
const (
	dockWidthKey   = "lich.dock.width"
	dockMinRem     = 20
	dockMaxRem     = 60
	dockDefaultRem = 28
	// dockHeaderHeight holds the 28dp tabs 8dp from the top and from the
	// Separator under it, the inset the sidebar's header keeps.
	dockHeaderHeight = unit.Dp(44)
)

type dockTab uint8

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

// dockHeaderSpare is what the header needs past its tabs and actions: its two
// insets and two gaps, and a gap's room between the tabs and the actions.
var dockHeaderSpare = ui.Space(8)

// toggled returns the tab after asking for to: asking for the tab on screen
// closes the dock.
func (t dockTab) toggled(to dockTab) dockTab {
	if t == to {
		return dockClosed
	}
	return to
}

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
	// islands are the visible stage and dock bounds from the last layout,
	// in the project tab strip's coordinates. The strip uses them to detach
	// its active tab when a resize makes it cross an island edge.
	islands []islandSpan
	// shown is the tab the dock last opened on, which it keeps drawing while
	// it slides out.
	shown      dockTab
	slide      dockSlide
	widthRem   float64
	fullscreen bool
	resize     ui.ResizeHandle
	tabs       ui.TabsState
	full       ui.IconActionState
	close      ui.IconActionState
	code       *codeTab
	review     reviewTab
	artifacts  *artifactsTab
	// ext is the extension surface of the dockExtension tab, drawn by
	// extensions; extAct is what its failed state asked for this frame.
	ext        extensionKey
	extensions *extensionScreen
	extAct     extensionAction
	// minRem is the width the header's tabs and actions need, measured on the
	// last frame: the dock is never narrower, so every tab keeps its name.
	minRem float64
}

func newDock(code *codeTab, review reviewTab, artifacts *artifactsTab, extensions *extensionScreen) *dock {
	return &dock{tour: new(driver.Tour), widthRem: dockDefaultRem, minRem: dockMinRem, resize: ui.ResizeHandle{Edge: ui.PanelEdgeLeft}, code: code, review: review, artifacts: artifacts, extensions: extensions}
}

// showsExtension is whether the dock is open on surface k.
func (d *dock) showsExtension(k extensionKey) bool { return d.tab == dockExtension && d.ext == k }

// toggleExtension opens the dock on surface k, and closes it while it shows k.
func (d *dock) toggleExtension(k extensionKey) {
	if d.showsExtension(k) {
		d.tab = dockClosed
		return
	}
	d.tab, d.ext = dockExtension, k
}

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
			d.tab = dockClosed
		}
	}
	if !d.fullscreen {
		w, released := d.resize.Update(gtx, unit.Dp(max(d.widthRem, d.minRem)*remDp), unit.Dp(d.minRem*remDp), dockMaxRem*remDp)
		d.widthRem, saveWidth = float64(w)/remDp, released
	}
	size := gtx.Constraints.Max
	gtx.Constraints = layout.Exact(size)
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops).Pop()
	ui.Fill(gtx, th.Sidebar, 0)
	on := th.On(th.Sidebar)
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
	)
	if !d.fullscreen {
		d.resize.Layout(gtx, on)
	}
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
	return d.code.layout(gtx, th, root, in)
}

// header is the h-10 strip: the ghost tabs, then at its right the Review
// tab's actions, full screen and close.
func (d *dock) header(gtx C, th *ui.Theme, root *ui.Root, in dockInput) D {
	d.tabs.Selected = d.selectedTab(in.placed)
	changed := in.isGit && in.git.Files > 0
	review := ui.TabsTrigger{Text: i18n.T("dock.rightDock.tabReview"), Icon: icDockReview}
	if changed {
		review.Trailing = func(gtx C) D { return th.DiffStat(in.git.Added, in.git.Deleted).Layout(gtx) }
	}
	artifacts := ui.TabsTrigger{Text: i18n.T("dock.rightDock.tabArtifacts"), Icon: icDockArtifacts}
	if n := d.artifacts.count(in.sessionID); n > 0 {
		artifacts.Trailing = th.Text(ui.TextXS, strconv.Itoa(n)).Mono().Muted().Layout
	}
	triggers := []ui.TabsTrigger{{Text: i18n.T("dock.rightDock.tabCode"), Icon: icDockCode}, review, artifacts}
	for _, p := range in.placed {
		triggers = append(triggers, ui.TabsTrigger{Text: p.title, Icon: p.glyph})
	}
	tabs := th.Tabs(&d.tabs, triggers...)
	tabs.Variant = ui.TabsVariantGhost
	actions := 0
	fullIcon, fullLabel := icDockMaximize, i18n.T("dock.rightDock.fullScreen")
	if d.fullscreen {
		fullIcon, fullLabel = icDockMinimize, i18n.T("dock.rightDock.exitFullScreen")
	}
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(dockHeaderHeight)))
	dims := layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				dims := tabs.Layout(gtx)
				if d.tabs.Changed() {
					d.pickTab(d.tabs.Selected, in.placed)
				}
				return dims
			}),
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
				dims := th.IconAction(root, &d.full, fullIcon, fullLabel, false).Layout(gtx)
				actions += dims.Size.X
				return dims
			}),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D {
				dims := d.closeButton(gtx, th, root)
				actions += dims.Size.X
				return dims
			}),
		)
	})
	d.fitTabs(gtx, tabs.NaturalWidth(gtx)+actions+gtx.Dp(dockHeaderSpare))
	return dims
}

// fitTabs keeps the dock at least as wide as the header needs, px wide, and
// redraws when that changes.
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

// act runs the header's clicks before the stage row reads whether the dock
// is open and how wide, so the frame shows their outcome.
func (d *dock) act(gtx C) {
	for d.full.Click.Clicked(gtx) {
		d.fullscreen = !d.fullscreen
	}
	for d.close.Click.Clicked(gtx) {
		d.tab = dockClosed
	}
}

// closeButton is the dock's ghost ×: a 1rem glyph, where an iconAction would
// draw its icon at 0.875rem.
func (d *dock) closeButton(gtx C, th *ui.Theme, root *ui.Root) D {
	b := th.Button(&d.close.Click, "")
	b.Icon, b.Variant, b.Size = icDockClose, ui.VariantGhost, ui.SizeIconXS
	tip := th.Tooltip(root, &d.close.Tip, i18n.T("dock.rightDock.closePanel"))
	return tip.Layout(gtx, b.Layout)
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
