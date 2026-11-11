package main

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// marksState is the live state of the marks section: the bell the dot sits on.
type marksState struct {
	bell widget.Clickable
}

// sized fixes w's box, for a widget that fills what it is given.
func sized(w, h unit.Dp, fn layout.Widget) layout.Widget {
	return func(gtx C) D {
		size := image.Pt(gtx.Dp(w), gtx.Dp(h))
		gtx.Constraints = layout.Exact(size)
		fn(gtx)
		return D{Size: size}
	}
}

func (g *gallery) dotsSection(gtx C, th *ui.Theme) D {
	s := &g.marksState
	bell := func(gtx C) D {
		d := th.BarIconButton(&s.bell, g.icon("bell"), false).Layout(gtx)
		th.NotifyDot(gtx, d.Size)
		return d
	}
	filter := func(gtx C) D {
		return sized(32, 32, func(gtx C) D {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx C) D { return g.icon("search").Layout(gtx, ui.Space(4), th.MutedForeground) }),
				layout.Expanded(func(gtx C) D { return th.CornerDot(gtx, th.Foreground) }),
			)
		})(gtx)
	}
	pulse := sized(24, 24, func(gtx C) D {
		center := f32.Pt(float32(gtx.Constraints.Min.X)/2, float32(gtx.Constraints.Min.Y)/2)
		th.PulseRing(gtx, center, float32(gtx.Dp(4)), float32(gtx.Dp(6)), th.ToneWait)
		ui.DotAt(gtx, center, float32(gtx.Dp(4)), th.ToneWait)
		return D{}
	})
	return stack(4,
		caption(th, "StatusDot: pass, wait, destructive; HollowDot: muted"),
		row(4,
			func(gtx C) D { return ui.StatusDot(gtx, th.TonePass) },
			func(gtx C) D { return ui.StatusDot(gtx, th.ToneWait) },
			func(gtx C) D { return ui.StatusDot(gtx, th.Destructive) },
			func(gtx C) D { return ui.HollowDot(gtx, th.MutedForeground) },
		),
		caption(th, "BeatingDot (one beat, then rest), PulseRing on a DotAt"),
		row(4, func(gtx C) D { return th.BeatingDot(gtx, th.TonePass) }, pulse),
		caption(th, "MarkDot: wait, pass, beside a title"),
		row(5,
			row(2, func(gtx C) D { return ui.MarkDot(gtx, th.ToneWait) }, th.Text(ui.TextSM, "Needs you").Layout),
			row(2, func(gtx C) D { return ui.MarkDot(gtx, th.TonePass) }, th.Text(ui.TextSM, "Done").Layout),
		),
		caption(th, "Dot: 6, 8 and 12dp"),
		row(3,
			func(gtx C) D { return ui.Dot(gtx, th.Foreground, 6) },
			func(gtx C) D { return ui.Dot(gtx, th.TonePass, 8) },
			func(gtx C) D { return ui.Dot(gtx, th.MutedForeground, 12) },
		),
		caption(th, "Swatch"),
		row(2,
			func(gtx C) D { return ui.Swatch(gtx, ui.Dark.Primary) },
			func(gtx C) D { return ui.Swatch(gtx, th.TonePass) },
			func(gtx C) D { return ui.Swatch(gtx, th.ToneWait) },
			func(gtx C) D { return ui.Swatch(gtx, th.Destructive) },
		),
		caption(th, "NotifyDot on a bar button, CornerDot on a rail button"),
		row(4, bell, filter),
	)(gtx)
}

func (g *gallery) gaugesSection(gtx C, th *ui.Theme) D {
	bar := func(percent int, tone color.NRGBA) layout.Widget {
		return func(gtx C) D { return th.GaugeBar(gtx, percent, tone) }
	}
	donut := func(size unit.Dp, percent int, fg color.NRGBA) layout.Widget {
		return func(gtx C) D { return th.DonutGauge(gtx, size, percent, fg) }
	}
	parts := func(read, write, uncached int) layout.Widget {
		return func(gtx C) D {
			return th.SegmentBar(gtx,
				ui.Segment{Share: read, Color: th.MutedForeground},
				ui.Segment{Share: write, Color: th.ToneWait},
				ui.Segment{Share: uncached, Color: th.Foreground})
		}
	}
	label := func(text string) layout.Widget { return th.Text(ui.TextXS, text).Muted().Layout }
	gaugeRow := func(name string, percent int, tone color.NRGBA, reading, left string) layout.Widget {
		return ui.GaugeRowStyle{
			LabelW: ui.Space(20), ReadingW: ui.Space(10), LeftW: ui.Space(16),
			Label: label(name), Bar: bar(percent, tone),
			Reading: th.Text(ui.TextXS, reading).In(tone).Layout, Left: label(left),
		}.Layout
	}
	return stack(4,
		caption(th, "GaugeBar: empty, 35% pass, 70% wait, 100% destructive"),
		stack(2, bar(0, th.TonePass), bar(35, th.TonePass), bar(70, th.ToneWait), bar(100, th.Destructive)),
		caption(th, "SegmentBar: three parts, one part, no share (bare track)"),
		stack(2, parts(60, 10, 30), parts(1, 0, 0), parts(0, 0, 0)),
		caption(th, "DonutGauge, 16dp: 0, 25, 60, 100%"),
		row(3, donut(16, 0, th.MutedForeground), donut(16, 25, th.TonePass), donut(16, 60, th.ToneWait), donut(16, 100, th.Destructive)),
		caption(th, "DonutGauge, 32dp"),
		row(3, donut(32, 25, th.TonePass), donut(32, 60, th.ToneWait)),
		caption(th, "GaugeRowStyle: label, bar, reading, time left"),
		stack(2,
			gaugeRow("Session", 12, th.TonePass, "12%", "4h 10m"),
			gaugeRow("Week", 78, th.ToneWait, "78%", "2d"),
			gaugeRow("Week (Opus)", 100, th.Destructive, "100%", "6h"),
		),
	)(gtx)
}

func (g *gallery) badgesSection(gtx C, th *ui.Theme) D {
	count := func(n int) layout.Widget { return func(gtx C) D { return th.CountBadge(gtx, n) } }
	abbr := func(text string, bg, fg color.NRGBA) layout.Widget { return th.AbbrBadge(text, bg, fg).Layout }
	return stack(4,
		caption(th, "CountBadge: 1, 7, 12, 128 (0 draws nothing)"),
		row(3, count(1), count(7), count(12), count(128), count(0)),
		caption(th, "OutlineBadge"),
		row(3, th.OutlineBadge("Draft").Layout, th.OutlineBadge("Fork").Layout),
		caption(th, "AbbrBadge"),
		row(3,
			abbr("TS", ui.MixOKLab(ui.Dark.Card, th.ToneWait, 0.25), th.ToneWait),
			abbr("GO", ui.MixOKLab(ui.Dark.Card, th.TonePass, 0.25), th.TonePass),
			abbr("MD", ui.MixOKLab(ui.Dark.Card, th.Destructive, 0.25), th.Destructive),
		),
		caption(th, "Tag: muted, foreground"),
		row(3, th.Tag("lich", th.MutedForeground).Layout, th.Tag("lich-plugin", th.Foreground).Layout),
		caption(th, "KeyHint"),
		th.KeyHint("Esc", "Close").Layout,
	)(gtx)
}

func (g *gallery) surfacesSection(gtx C, th *ui.Theme) D {
	lines := func(gtx C) D {
		return layout.UniformInset(ui.Space(3)).Layout(gtx, stack(1,
			th.Text(ui.TextSM, "feature/toolbar").Layout,
			th.Text(ui.TextXS, "3 sessions").Muted().Layout,
		))
	}
	tray := func(fill color.NRGBA) layout.Widget {
		return func(gtx C) D { return ui.Tray(gtx, fill, lines) }
	}
	dropTray := func(fill, edge color.NRGBA) layout.Widget {
		return func(gtx C) D { return ui.DropTray(gtx, fill, edge, lines) }
	}
	plain, hover := th.Over(ui.Alpha(th.Accent, 0.4)), th.Over(th.Accent)
	tinted := th.Over(ui.MixOKLab(color.NRGBA{}, th.ToneWait, 0.22))
	ring := func(draw func(C, image.Rectangle)) layout.Widget {
		return sized(120, 60, func(gtx C) D {
			box := image.Rectangle{Max: gtx.Constraints.Min}.Inset(gtx.Dp(8))
			ui.FillRRect(gtx, box, 8, ui.Alpha(th.ToneWait, 0.1))
			draw(gtx, box)
			return D{}
		})
	}
	halo := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xb0}
	dashes := ring(func(gtx C, box image.Rectangle) { ui.DashedRing(gtx, box, 8, 1.5, th.ToneWait, halo) })
	marching := ring(func(gtx C, box image.Rectangle) { ui.MarchingRing(gtx, box, 8, 1.5, th.ToneWait, halo) })
	circle := sized(40, 40, func(gtx C) D {
		ui.DashedCircle(gtx, f32.Pt(float32(gtx.Dp(20)), float32(gtx.Dp(20))), 14, 2, 3, th.MutedForeground)
		return D{}
	})
	dashedBox := sized(120, 48, func(gtx C) D {
		ui.DashedBorder(gtx, gtx.Constraints.Min, gtx.Dp(ui.RadiusMD), th.MutedForeground, 4, 3)
		return D{}
	})
	tab := sized(120, 32, func(gtx C) D {
		r := gtx.Dp(ui.RadiusLG)
		size := gtx.Constraints.Min
		ui.Fillet(gtx, image.Pt(0, size.Y-r), r, false, th.Accent)
		ui.Fillet(gtx, image.Pt(size.X-r, size.Y-r), r, true, th.Accent)
		body := gtx
		body.Constraints = layout.Exact(image.Pt(size.X-2*r, size.Y))
		defer op.Offset(image.Pt(r, 0)).Push(gtx.Ops).Pop()
		ui.Fill(body, th.Accent, 0)
		return D{}
	})
	checker := sized(120, 40, func(gtx C) D {
		th.Checker(gtx, gtx.Constraints.Min, ui.Space(1.5))
		return D{}
	})
	fillIf := func(c color.NRGBA) layout.Widget {
		return sized(80, 32, func(gtx C) D { return ui.FillIf(gtx, c, ui.RadiusMD) })
	}
	hint := func(h unit.Dp, confined bool) layout.Widget {
		detail := "Pasted at the prompt"
		if confined {
			detail = "Copied into the sandbox"
		}
		return sized(260, h, func(gtx C) D {
			dark := g.th.On(ui.Dark.Background)
			ui.Fill(gtx, ui.Dark.Background, ui.RadiusMD)
			return dark.DropHint(g.icon("paperclip"), "Attach to ", "feature/toolbar", "", detail).Layout(gtx)
		})
	}
	return stack(4,
		caption(th, "Tray, DropTray (muted edge), DropTray tinted"),
		stack(2, tray(plain), dropTray(hover, th.MutedForeground), dropTray(tinted, th.ToneWait)),
		caption(th, "DashedBorder"),
		dashedBox,
		caption(th, "FillRRect veil under DashedRing, MarchingRing"),
		row(4, dashes, marching),
		caption(th, "DashedCircle"),
		circle,
		caption(th, "Fillet: either side of a tab"),
		tab,
		caption(th, "Checker"),
		checker,
		caption(th, "FillIf: transparent, accent"),
		row(3, fillIf(color.NRGBA{}), fillIf(th.Accent)),
		caption(th, "DropHint: tall enough for its detail, too short for it"),
		stack(2, hint(130, false), hint(100, true)),
	)(gtx)
}
