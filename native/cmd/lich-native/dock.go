package main

// The right dock (RightDock.tsx): the panel at the terminal's right that the
// footer's Browse code and Review changes buttons open, on its Code or its
// Review tab. It owns the chrome around both bodies: the drag-resizable width
// (one, persisted, for both tabs), full screen, the tab bar and the close
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
// layoutActions is drawn at the header's right, before full screen, only
// while the Review tab is on and the checkout has changes, as RightDock.tsx
// does. th is the header's: the actions sit on bg-sidebar. holdsKeys keeps
// the terminal, which takes focus every frame it is not told otherwise, off a
// field of the body; the terminal asks before the dock is laid out, so a flag
// the body sets during its own layout answers one frame late, and that frame
// is the one the terminal takes the focus back in. iconAction is the
// IconAction both halves draw them with, and notice the panels' Notice.

import (
	"context"
	"image"
	"math"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n; the web's en catalog
// (dock.rightDock).
const (
	dockTabCode        = "Code"
	dockTabReview      = "Review"
	dockFullScreen     = "Full screen"
	dockExitFullScreen = "Exit full screen"
	dockClosePanel     = "Close panel"
)

var (
	icDockCode     = icons.Lucide("code")
	icDockReview   = icons.Lucide("file-diff")
	icDockMaximize = icons.Lucide("maximize-2")
	icDockMinimize = icons.Lucide("minimize-2")
	icDockClose    = icons.Lucide("x")
)

// The web's localStorage key for the dock's width, kept as a backend setting,
// and RightDock's bounds in rem: wide enough for a diff line at the review's
// 12px font, short of eating the terminal whole.
const (
	dockWidthKey     = "lich.dock.width"
	dockMinRem       = 20
	dockMaxRem       = 60
	dockDefaultRem   = 28
	dockHeaderHeight = unit.Dp(40) // h-10
)

type dockTab uint8

const (
	dockClosed dockTab = iota
	dockCode
	dockReview
)

// toggled is App.tsx toggleDock: asking for the tab on screen closes the dock.
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
	checkout  string
	git       lichclient.DiffStats
	isGit     bool
	light     bool
}

type reviewTab interface {
	layoutBody(gtx C, th *ui.Theme, in dockInput) error
	layoutActions(gtx C, th *ui.Theme) D
	hidden()
	holdsKeys(gtx C) bool
}

type dock struct {
	tab        dockTab
	widthRem   float64
	fullscreen bool
	resize     ui.ResizeHandle
	tabs       ui.TabsState
	full       iconActionState
	close      iconActionState
	code       *codeTab
	review     reviewTab
}

func newDock(code *codeTab, review reviewTab) *dock {
	return &dock{widthRem: dockDefaultRem, resize: ui.ResizeHandle{Edge: ui.PanelEdgeLeft}, code: code, review: review}
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
	}
	return false
}

// width is the dock's width beside the terminal, 0 while closed or covering it.
func (d *dock) width(gtx C) int {
	if d.tab == dockClosed || d.fullscreen {
		return 0
	}
	return gtx.Dp(unit.Dp(d.widthRem * remDp))
}

// layout draws the dock in gtx.Constraints.Max, the box App.tsx gives it: its
// column beside the terminal, or the whole row in full screen. It reports
// whether a drag of its width ended, the moment to persist it.
func (d *dock) layout(gtx C, th *ui.Theme, root *ui.Root, in dockInput) (saveWidth bool, err error) {
	if d.tab != dockCode {
		d.code.hidden()
	}
	if d.tab != dockReview {
		d.review.hidden()
	}
	if !d.fullscreen {
		w, released := d.resize.Update(gtx, unit.Dp(d.widthRem*remDp), dockMinRem*remDp, dockMaxRem*remDp)
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
			err = d.body(gtx, on, root, in)
			return D{Size: gtx.Constraints.Max}
		}),
	)
	if !d.fullscreen {
		d.resize.Layout(gtx, on)
	}
	return saveWidth, err
}

func (d *dock) body(gtx C, th *ui.Theme, root *ui.Root, in dockInput) error {
	if d.tab == dockReview {
		return d.review.layoutBody(gtx, th, in)
	}
	return d.code.layout(gtx, th, root, in)
}

// header is the h-10 strip: the ghost tabs, then at its right the Review
// tab's actions, full screen and close.
func (d *dock) header(gtx C, th *ui.Theme, root *ui.Root, in dockInput) D {
	d.act(gtx)
	changed := in.isGit && in.git.Files > 0
	review := ui.TabsTrigger{Text: dockTabReview, Icon: icDockReview}
	if changed {
		review.Trailing = func(gtx C) D { return diffStat(gtx, th, in.git.Added, in.git.Deleted) }
	}
	tabs := th.Tabs(&d.tabs, ui.TabsTrigger{Text: dockTabCode, Icon: icDockCode}, review)
	tabs.Variant = ui.TabsVariantGhost
	fullIcon, fullLabel := icDockMaximize, dockFullScreen
	if d.fullscreen {
		fullIcon, fullLabel = icDockMinimize, dockExitFullScreen
	}
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(dockHeaderHeight)))
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				dims := tabs.Layout(gtx)
				if d.tabs.Changed() {
					d.tab = dockTab(d.tabs.Selected + 1)
				}
				return dims
			}),
			layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
			layout.Rigid(func(gtx C) D {
				if d.tab != dockReview || !changed {
					return D{}
				}
				return d.review.layoutActions(gtx, th)
			}),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return iconAction(gtx, th, root, &d.full, fullIcon, fullLabel, false) }),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return d.closeButton(gtx, th, root) }),
		)
	})
}

// act runs the header's clicks before it is drawn, so the frame shows their
// outcome.
func (d *dock) act(gtx C) {
	d.tabs.Selected = int(d.tab) - 1
	for d.full.click.Clicked(gtx) {
		d.fullscreen = !d.fullscreen
	}
	for d.close.click.Clicked(gtx) {
		d.tab = dockClosed
	}
}

// closeButton is RightDock's ghost icon-xs ×: the size-4 glyph a size-6
// IconAction would draw at 3.5.
func (d *dock) closeButton(gtx C, th *ui.Theme, root *ui.Root) D {
	b := th.Button(&d.close.click, "")
	b.Icon, b.Variant, b.Size = icDockClose, ui.VariantGhost, ui.SizeIconXS
	tip := th.Tooltip(root, &d.close.tip, dockClosePanel)
	return tip.Layout(gtx, b.Layout)
}

// iconActionState is an IconAction's clickable and tooltip.
type iconActionState struct {
	click widget.Clickable
	tip   ui.Tooltip
}

// iconAction is IconAction.tsx: a size-6 bare icon button for a panel's
// header, its meaning in a tooltip, bg-accent while hovered or pressed.
func iconAction(gtx C, th *ui.Theme, root *ui.Root, s *iconActionState, ic *icons.Icon, label string, pressed bool) D {
	tip := th.Tooltip(root, &s.tip, label)
	return tip.Layout(gtx, func(gtx C) D {
		return s.click.Layout(gtx, func(gtx C) D {
			side := gtx.Dp(ui.Space(6))
			gtx.Constraints = layout.Exact(image.Pt(side, side))
			fg := th.MutedForeground
			if pressed || s.click.Hovered() {
				ui.Fill(gtx, th.Accent, ui.RadiusMD)
				fg = th.AccentForeground
			}
			return layout.Center.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), fg) })
		})
	})
}

// notice is Notice.tsx: the quiet line a panel shows instead of content,
// px-3 py-4 text-xs muted.
func notice(gtx C, th *ui.Theme, text string) D {
	l := th.Text(ui.TextXS, text).Muted()
	l.MaxLines = 0
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(4), Bottom: ui.Space(4)}.Layout(gtx, l.Layout)
}

// footnote is a Notice under a list, border-t py-2: what the listing owes
// the reader about itself.
func footnote(gtx C, th *ui.Theme, text string) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Separator),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx,
				func(gtx C) D {
					l := th.Text(ui.TextXS, text).Muted()
					l.MaxLines = 0
					return l.Layout(gtx)
				})
		}),
	)
}

// parseDockWidth is panel-width.ts parseStoredWidth with the dock's bounds:
// garbage, negatives and an unset value fall back to the default.
func parseDockWidth(raw string) float64 {
	rem, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsInf(rem, 0) || math.IsNaN(rem) || rem <= 0 {
		return dockDefaultRem
	}
	return min(dockMaxRem, max(dockMinRem, rem))
}

func loadDockWidth(ctx context.Context, c *lichclient.Client) (float64, error) {
	raw, err := c.GetSetting(ctx, dockWidthKey, "")
	return parseDockWidth(raw), err
}

func saveDockWidth(ctx context.Context, c *lichclient.Client, rem float64) error {
	return c.SetSetting(ctx, dockWidthKey, "", strconv.FormatFloat(rem, 'f', -1, 64))
}
