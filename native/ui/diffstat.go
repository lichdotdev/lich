package ui

import (
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
)

// The added and deleted colors are fixed shades, not theme tokens.
var (
	colAddedLight   = OKLCH(0.596, 0.145, 163.225, 1)
	colDeletedLight = OKLCH(0.577, 0.245, 27.325, 1)
	colAddedDark    = OKLCH(0.765, 0.177, 163.223, 1)
	colDeletedDark  = OKLCH(0.704, 0.191, 22.216, 1)
)

// diffStatColors picks the light or dark shades by the palette the window
// paints with; Light is the only light one it has.
func diffStatColors(th *Theme) (added, deleted color.NRGBA) {
	if th.Palette == Light {
		return colAddedLight, colDeletedLight
	}
	return colAddedDark, colDeletedDark
}

// DiffStatStyle is the +added -deleted pair every diff surface shows.
type DiffStatStyle struct {
	Added, Deleted int
	theme          *Theme
}

func (th *Theme) DiffStat(added, deleted int) DiffStatStyle {
	return DiffStatStyle{Added: added, Deleted: deleted, theme: th}
}

func (s DiffStatStyle) Layout(gtx C) D {
	th := s.theme
	addedColor, deletedColor := diffStatColors(th)
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(th.Text(TextXS, "+"+strconv.Itoa(s.Added)).In(addedColor).Weight(font.Medium).Layout),
		Gap(1.5),
		layout.Rigid(th.Text(TextXS, "-"+strconv.Itoa(s.Deleted)).In(deletedColor).Weight(font.Medium).Layout),
	)
}
