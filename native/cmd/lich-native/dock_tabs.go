package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The dock's tabs are a pill of icon-only buttons at its foot, centred: the
// dock's width follows the drag, and the pill stays where the eye looks for it.
// The pill sits on the dock's own Sidebar surface, so it is the folder's tray
// (folderTrayAlpha) to read against it.
var (
	dockTabPillPad = ui.Space(1)
	dockTabPillGap = ui.Space(0.5)
	// dockTabBarInset keeps the pill off the dock's bottom and side edges.
	dockTabBarInset = ui.Space(2)
	// dockTabMark is the dot on a tab that holds something, near the icon's
	// top right corner.
	dockTabMark       = ui.Space(1.25)
	dockTabMarkOffset = ui.Space(1)
)

// dockTabButton is one tab's clickable and tooltip.
type dockTabButton struct {
	click ui.Clickable
	tip   ui.Tooltip
}

// dockTabSpec is a tab as the pill draws it; mark puts a dot on its icon
// while the tab holds something (changes, artifacts), in ToneWait while
// waiting is set: Review over a checkout with conflicts.
type dockTabSpec struct {
	glyph   icons.Glyph
	label   string
	mark    bool
	waiting bool
}

// tabSpecs are lich's tabs and then the extensions', in the order selectedTab
// and pickTab count them.
func (d *dock) tabSpecs(in dockInput) []dockTabSpec {
	specs := []dockTabSpec{
		{glyph: icDockCode, label: i18n.T("dock.rightDock.tabCode")},
		{glyph: icDockReview, label: i18n.T("dock.rightDock.tabReview"), mark: in.isGit && in.git.Files > 0, waiting: in.isGit && len(in.git.Conflicted) > 0},
		{glyph: icDockArtifacts, label: i18n.T("dock.rightDock.tabArtifacts"), mark: d.artifacts.count(in.sessionID) > 0},
	}
	for _, p := range in.placed {
		specs = append(specs, dockTabSpec{glyph: p.glyph, label: p.title})
	}
	return specs
}

// actTabs opens the tab a click picked. It runs before the stage row reads the
// dock, so the frame that took the click already draws the tab.
func (d *dock) actTabs(gtx C) {
	for i, b := range d.tabButtons {
		for b.click.Clicked(gtx) {
			d.pickTab(i, d.placed)
		}
	}
}

// tabBar is the strip under the body that holds the pill. It records the
// pill's width, which fitTabs keeps the dock from going under.
func (d *dock) tabBar(gtx C, th *ui.Theme, root *ui.Root, in dockInput) D {
	specs := d.tabSpecs(in)
	d.placed = in.placed
	for len(d.tabButtons) < len(specs) {
		d.tabButtons = append(d.tabButtons, new(dockTabButton))
	}
	d.tabButtons = d.tabButtons[:len(specs)]
	selected := d.selectedTab(in.placed)
	pill := func(gtx C) D {
		return layout.UniformInset(dockTabPillPad).Layout(gtx, func(gtx C) D {
			var row []layout.FlexChild
			for i, s := range specs {
				if i > 0 {
					row = append(row, layout.Rigid(layout.Spacer{Width: dockTabPillGap}.Layout))
				}
				row = append(row, layout.Rigid(func(gtx C) D { return dockTabButtonLayout(gtx, th, root, d.tabButtons[i], s, i == selected) }))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
		})
	}
	var size image.Point
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dims := layout.Center.Layout(gtx, func(gtx C) D {
		return layout.UniformInset(dockTabBarInset).Layout(gtx, func(gtx C) D {
			dims := layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, folderTrayAlpha)), ui.RadiusLG) }, pill)
			size = dims.Size
			return dims
		})
	})
	d.tabBarWidth = size.X + 2*gtx.Dp(dockTabBarInset)
	return dims
}

// dockTabButtonLayout is one tab's button: the bar's icon button, accent while its tab is
// on screen, with the tab's name in a tooltip.
func dockTabButtonLayout(gtx C, th *ui.Theme, root *ui.Root, b *dockTabButton, s dockTabSpec, on bool) D {
	button := func(gtx C) D { return th.BarIconButton(&b.click, s.glyph, on).Layout(gtx) }
	tipped := func(gtx C) D { return th.Tooltip(root, &b.tip, s.label).Layout(gtx, button) }
	if !s.mark {
		return tipped(gtx)
	}
	return layout.Stack{Alignment: layout.NE}.Layout(gtx,
		layout.Stacked(tipped),
		layout.Stacked(func(gtx C) D {
			return layout.Inset{Top: dockTabMarkOffset, Right: dockTabMarkOffset}.Layout(gtx, func(gtx C) D {
				side := gtx.Dp(dockTabMark)
				tone := th.Foreground
				if s.waiting {
					tone = th.ToneWait
				}
				paint.FillShape(gtx.Ops, tone, clip.Ellipse{Max: image.Pt(side, side)}.Op(gtx.Ops))
				return D{Size: image.Pt(side, side)}
			})
		}),
	)
}
