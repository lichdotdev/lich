package main

import (
	"image"
	"image/color"
	"math"
	"strconv"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n.

const (
	// footerDisabledOpacity is FooterButton's disabled:opacity-40.
	footerDisabledOpacity = 0.4
	// readoutHoverOpacity is FooterReadout's hover:bg-accent/50.
	readoutHoverOpacity = 0.5
	// ringTrackOpacity is ContextRing's opacity-20 track.
	ringTrackOpacity = 0.2
	// ContextRing's viewBox, and the radius and stroke drawn in it.
	ringViewBox = 36
	ringRadius  = 15.9155
	ringStroke  = 4
)

var (
	checkoutMaxWidth = ui.Space(56) // FooterCheckout's max-w-56
	modelMaxWidth    = ui.Space(64) // SessionModel's max-w-64
	footerGlyph      = ui.Space(3.5)
	footerIconGlyph  = ui.Space(4)
)

// footerContent draws a control's inside in fg, on th's surface.
type footerContent func(gtx C, th *ui.Theme, fg color.NRGBA) D

// footerButton is FooterButton: a ghost size-8 square, or h-8 px-2 gap-1.5
// when wide, muted at rest, bg-accent with accent-foreground hovered or
// pressed, its label in a tooltip.
func footerButton(gtx C, th *ui.Theme, root *ui.Root, w *footerWidgets, label string, wide, disabled bool, content footerContent) D {
	tip := th.Tooltip(root, &w.tip, label)
	return tip.Layout(gtx, func(gtx C) D {
		if disabled {
			return buttonFace(gtx, th, wide, false, true, content)
		}
		return w.click.Layout(gtx, func(gtx C) D { return buttonFace(gtx, th, wide, w.click.Hovered() || w.pressed, false, content) })
	})
}

func buttonFace(gtx C, th *ui.Theme, wide, lit, disabled bool, content footerContent) D {
	var bg color.NRGBA
	fg := th.MutedForeground
	if lit {
		bg, fg = th.Accent, th.AccentForeground
	}
	if disabled {
		fg = th.Over(ui.Alpha(fg, footerDisabledOpacity))
	}
	h := gtx.Dp(ui.Space(8))
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	pad := ui.Space(2)
	if !wide {
		gtx.Constraints.Min.X, gtx.Constraints.Max.X, pad = h, h, 0
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return fillIf(gtx, bg, ui.RadiusMD) },
		func(gtx C) D {
			return layout.Inset{Left: pad, Right: pad}.Layout(gtx, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D { return content(gtx, onFill(th, bg), fg) })
			})
		},
	)
}

// fillIf paints c unless it is transparent, a box at rest.
func fillIf(gtx C, c color.NRGBA, radius unit.Dp) D {
	if c.A == 0 {
		return D{Size: gtx.Constraints.Min}
	}
	return ui.Fill(gtx, c, radius)
}

// onFill is th drawing on bg, or on th's own surface when bg is transparent.
func onFill(th *ui.Theme, bg color.NRGBA) *ui.Theme {
	if bg.A == 0 {
		return th
	}
	return th.On(bg)
}

// footerReadoutView is FooterReadout: a reading in its tone, h-8 px-2
// gap-1.5, hover:bg-accent/50, over a card tooltip of its title and detail.
func footerReadoutView(gtx C, th *ui.Theme, root *ui.Root, w *footerWidgets, r footerReadout, detail layout.Widget, content footerContent) D {
	if detail == nil {
		detail = readoutDetail(th.On(th.Popover), r.Detail)
	}
	title := r.Title
	if title == "" {
		title = r.Label
	}
	tip := th.TooltipCard(root, &w.tip, func(gtx C) D {
		pop := th.On(th.Popover)
		return stack(gtx, 1.5, []layout.Widget{pop.Text(ui.TextXS, title).Weight(font.Medium).Layout, detail})
	})
	return tip.Layout(gtx, func(gtx C) D {
		return w.click.Layout(gtx, func(gtx C) D {
			var bg color.NRGBA
			if w.click.Hovered() {
				bg = th.Over(ui.Alpha(th.Accent, readoutHoverOpacity))
			}
			gtx.Constraints.Min.Y = gtx.Dp(ui.Space(8))
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return fillIf(gtx, bg, ui.RadiusMD) },
				func(gtx C) D {
					return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
						return layout.W.Layout(gtx, func(gtx C) D { return content(gtx, onFill(th, bg), toneColor(th, r.Tone)) })
					})
				},
			)
		})
	})
}

// readoutDetail is the tooltip's text-xs muted lines, flex-col gap-1.
func readoutDetail(th *ui.Theme, lines []string) layout.Widget {
	rows := make([]layout.Widget, len(lines))
	for i, line := range lines {
		rows[i] = para(th.Text(ui.TextXS, line).Muted()).Layout
	}
	return func(gtx C) D { return stack(gtx, 1, rows) }
}

// toneColor is usageColor's text class.
func toneColor(th *ui.Theme, t usageTone) color.NRGBA {
	switch t {
	case toneWait:
		return th.ToneWait
	case toneDestructive:
		return th.Destructive
	}
	return th.MutedForeground
}

// glyphText is a size-3.5 glyph then text, gap-1.5: the inside of most
// readouts and wide buttons.
func glyphText(ic *icons.Icon, text string) footerContent {
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, footerGlyph, fg) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, text).In(fg).Layout),
		)
	}
}

// footerText is a plain reading in fg.
func footerText(text string) footerContent {
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D { return th.Text(ui.TextXS, text).In(fg).Layout(gtx) }
}

// footerSpan is the hinted spans of FooterCheckout and SessionModel: px-2
// gap-1.5, a glyph and a truncated reading, at most maxWidth wide.
func footerSpan(gtx C, th *ui.Theme, root *ui.Root, tip *ui.Tooltip, hint string, maxWidth unit.Dp, glyph layout.Widget, text string) D {
	return th.Tooltip(root, tip, hint).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(maxWidth))
		gtx.Constraints.Min.Y = gtx.Dp(ui.Space(8))
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.W.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(glyph),
					ui.Gap(1.5),
					layout.Rigid(th.Text(ui.TextXS, text).Muted().Layout),
				)
			})
		})
	})
}

// contextRing is ContextRing: a size-4 donut, a 20% track under an arc
// filled to percent clockwise from the top, with round ends.
func contextRing(gtx C, th *ui.Theme, percent int, fg color.NRGBA) D {
	size := gtx.Dp(footerIconGlyph)
	scale := float32(size) / ringViewBox
	center := f32.Pt(float32(size)/2, float32(size)/2)
	radius, width := ringRadius*scale, ringStroke*scale
	strokeArc(gtx, center, radius, width, 0, 2*math.Pi, th.Over(ui.Alpha(fg, ringTrackOpacity)))
	share := float32(min(max(percent, 0), 100)) / 100
	if share > 0 {
		strokeArc(gtx, center, radius, width, -math.Pi/2, share*2*math.Pi, fg)
		top := center.Sub(f32.Pt(0, radius))
		end := arcPoint(center, radius, share)
		dot(gtx, top, width/2, fg)
		dot(gtx, end, width/2, fg)
	}
	return D{Size: image.Pt(size, size)}
}

func arcPoint(center f32.Point, radius, share float32) f32.Point {
	angle := float64(share)*2*math.Pi - math.Pi/2
	return center.Add(f32.Pt(radius*float32(math.Cos(angle)), radius*float32(math.Sin(angle))))
}

// dot is a stroke's round cap.
func dot(gtx C, at f32.Point, r float32, c color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(at.Add(f32.Pt(r, 0)))
	p.ArcTo(at, at, 2*math.Pi)
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
}

// footerWidgets is one footer control's state.
type footerWidgets struct {
	click widget.Clickable
	tip   ui.Tooltip
	// pressed is aria-pressed: the button's panel is open, so it keeps the
	// fill it shows hovered.
	pressed bool
}

// iconOnly is a square button's size-4 glyph.
func iconOnly(ic *icons.Icon) footerContent {
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D { return ic.Layout(gtx, footerIconGlyph, fg) }
}

// changesContent is the Review changes button: "0" beside the diff glyph on
// a clean checkout, else the file count, a dot and the line counts.
func changesContent(git lichclient.DiffStats) footerContent {
	if git.Files == 0 {
		return glyphText(icDiff, "0")
	}
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icFileText.Layout(gtx, footerGlyph, fg) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(git.Files)).In(fg).Layout),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, "·").In(th.Over(ui.Alpha(fg, separatorOpacity))).Layout),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D { return diffStat(gtx, th, git.Added, git.Deleted) }),
		)
	}
}

// separatorOpacity is the changes button's opacity-50 dot.
const separatorOpacity = 0.5

// ringContent is the context readout: the ring, then the percentage.
func ringContent(percent int, text string) footerContent {
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return contextRing(gtx, th, percent, fg) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, text).In(fg).Layout),
		)
	}
}
