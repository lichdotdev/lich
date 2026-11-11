package main

import (
	"image/color"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/ui"
)

func (g *gallery) statSection(gtx C, th *ui.Theme) D {
	glyph := func(name string, fg color.NRGBA) layout.Widget {
		return func(gtx C) D { return g.icon(name).Layout(gtx, ui.Space(3.5), fg) }
	}
	ring := func(gtx C) D { return th.BusyRing(gtx, ui.Space(3.5), th.ToneWait, th.RingTrack(th.ToneWait)) }
	narrow := func(w layout.Widget) layout.Widget {
		return func(gtx C) D {
			gtx.Constraints.Max.X = gtx.Dp(140)
			return w(gtx)
		}
	}
	return ui.Stack(4,
		caption(th, "Stat: a glyph widget and its words in one tone (tone-pass, muted)"),
		ui.Row(4,
			func(gtx C) D { return th.Stat(gtx, glyph("circle-check", th.TonePass), "Checks passing", th.TonePass) },
			func(gtx C) D {
				return th.Stat(gtx, glyph("git-merge", th.MutedForeground), "Merged", th.MutedForeground)
			},
		),
		caption(th, "Stat with a BusyRing as the glyph"),
		func(gtx C) D { return th.Stat(gtx, ring, "Checks running", th.ToneWait) },
		caption(th, "Stat wrapping onto a second line in 140"),
		narrow(func(gtx C) D {
			return th.Stat(gtx, glyph("circle-x", th.Destructive), "Two checks failed on this branch", th.Destructive)
		}),
	)(gtx)
}
