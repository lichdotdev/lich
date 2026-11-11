package ui

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
)

// Stat is glyph and, after a gap-1.5, words in fg at text-xs medium, wrapping
// onto as many lines as they need: one fact of a status line. The glyph is
// whatever the caller draws, sized by the caller.
func (th *Theme) Stat(gtx C, glyph layout.Widget, words string, fg color.NRGBA) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(glyph),
		Gap(1.5),
		layout.Rigid(func(gtx C) D {
			l := th.Text(TextXS, words).In(fg).Weight(font.Medium)
			l.MaxLines = 0
			return l.Layout(gtx)
		}),
	)
}
