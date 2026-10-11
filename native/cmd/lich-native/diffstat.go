package main

import (
	"image/color"
	"strconv"

	"gioui.org/font"

	"gioui.org/layout"

	"github.com/omartelo/lich/native/ui"
)

// DiffStat.tsx's text-emerald-600 dark:text-emerald-400 and text-red-600
// dark:text-red-400: Tailwind colors, not theme tokens.
var (
	colAddedLight   = ui.OKLCH(0.596, 0.145, 163.225, 1)
	colDeletedLight = ui.OKLCH(0.577, 0.245, 27.325, 1)
	colAddedDark    = ui.OKLCH(0.765, 0.177, 163.223, 1)
	colDeletedDark  = ui.OKLCH(0.704, 0.191, 22.216, 1)
)

// diffStatColors picks the light or dark shades by the palette the window
// paints with; ui.Light is the only light one it has.
func diffStatColors(th *ui.Theme) (added, deleted color.NRGBA) {
	if th.Palette == ui.Light {
		return colAddedLight, colDeletedLight
	}
	return colAddedDark, colDeletedDark
}

// diffStat is the +added -deleted pair every diff surface shows, spaced by
// the gap-1.5 of the clusters it sits in.
func diffStat(gtx C, th *ui.Theme, added, deleted int) D {
	addedColor, deletedColor := diffStatColors(th)
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextXS, "+"+strconv.Itoa(added)).In(addedColor).Weight(font.Medium).Layout),
		ui.Gap(1.5),
		layout.Rigid(th.Text(ui.TextXS, "-"+strconv.Itoa(deleted)).In(deletedColor).Weight(font.Medium).Layout),
	)
}
