package main

import (
	"fmt"
	"math"
	"runtime"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icBrowserZoom = icons.Lucide("zoom-in")

// browserZoomPanelWidth fits the Zoom row's label and its three controls.
const browserZoomPanelWidth = unit.Dp(208)

// zoomValueMinWidth keeps the reading as wide as "200%", so stepping never
// shifts the buttons around it.
var zoomValueMinWidth = ui.Space(10)

// browserZoomState is the toolbar's zoom control: the button, its panel and
// the panel's rows.
type browserZoomState struct {
	button         iconActionState
	panel          ui.DropdownPanelState
	out, in, reset widget.Clickable
	outTip, inTip  ui.Tooltip
}

// browserZoomControl is the page zoom's reading, when it is not 100%, and the
// button beside it that opens the panel to step and reset it.
func (u *window) browserZoomControl(gtx C, z *browserZoomState) D {
	th := u.theme
	if z.out.Clicked(gtx) {
		u.setBrowserZoom(clampZoom(u.browserZoom - zoomStep))
	}
	if z.in.Clicked(gtx) {
		u.setBrowserZoom(clampZoom(u.browserZoom + zoomStep))
	}
	if z.reset.Clicked(gtx) {
		u.setBrowserZoom(defaultZoom)
	}
	pop := th.DropdownPanel(&u.root, &z.panel, browserZoomPanelWidth, func(gtx C, th *ui.Theme) D {
		return u.browserZoomPanel(gtx, th, z)
	})
	pop.AlignEnd = true
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if u.browserZoom == defaultZoom {
				return D{}
			}
			return layout.Inset{Right: ui.Space(1)}.Layout(gtx, th.Text(ui.TextXS, zoomDisplay(u.browserZoom)).In(th.MutedForeground).Layout)
		}),
		// The reading stays out of the trigger: the panel keeps the place the
		// trigger had when it opened, and the reading showing up moves it.
		layout.Rigid(func(gtx C) D {
			return pop.Layout(gtx, func(gtx C) D {
				return iconAction(gtx, th, &u.root, &z.button, icBrowserZoom, i18n.T("layouts.browser.zoom"), z.panel.Opened())
			})
		}),
	)
}

// browserZoomPanel is a menu: the Zoom row with out, the reading and in,
// then Reset to 100% with its chord.
func (u *window) browserZoomPanel(gtx C, th *ui.Theme, z *browserZoomState) D {
	isMac := runtime.GOOS == "darwin"
	step := func(click *widget.Clickable, tip *ui.Tooltip, ic *icons.Icon, label, key string, disabled bool) layout.Widget {
		b := th.Button(click, "")
		b.Icon, b.Variant, b.Size, b.Disabled = ic, ui.VariantGhost, ui.SizeIconXS, disabled
		return func(gtx C) D {
			return th.Tooltip(&u.root, tip, label+" · "+zoomChord(key, isMac)).Layout(gtx, b.Layout)
		}
	}
	zoomRow := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Left: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, th.Text(ui.TextSM, i18n.T("settings.appearanceSettings.zoomTitle")).Layout),
				layout.Rigid(step(&z.out, &z.outTip, icMinus, i18n.T("settings.appearanceSettings.zoomOut"), "-", u.browserZoom <= zoomMin)),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Dp(zoomValueMinWidth)
					return layout.Center.Layout(gtx, th.Text(ui.TextSM, zoomDisplay(u.browserZoom)).Layout)
				}),
				layout.Rigid(step(&z.in, &z.inTip, icPlus, i18n.T("settings.appearanceSettings.zoomIn"), "=", u.browserZoom >= zoomMax)),
			)
		})
	}
	resetRow := func(gtx C) D {
		return popoverRow(gtx, th, &z.reset, func(gtx C, th *ui.Theme) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, th.Text(ui.TextSM, i18n.T("layouts.browser.zoomReset")).Layout),
				layout.Rigid(th.Text(ui.TextXS, zoomChord("0", isMac)).Muted().Layout),
			)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(zoomRow),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, th.Separator)
		}),
		layout.Rigid(resetRow),
	)
}

func zoomDisplay(zoom float64) string { return fmt.Sprintf("%d%%", int(math.Round(zoom*100))) }

// zoomChord spells the pageZoomChord that key makes.
func zoomChord(key string, isMac bool) string {
	return formatCombo(combo{Mod: true, Key: key}, isMac)
}
