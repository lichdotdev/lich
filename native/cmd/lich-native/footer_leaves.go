package main

import (
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

const (
	// footerDisabledOpacity is FooterButton's disabled:opacity-40.
	footerDisabledOpacity = 0.4
	// readoutHoverOpacity is FooterReadout's hover:bg-accent/50.
	readoutHoverOpacity = 0.5
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
		func(gtx C) D { return ui.FillIf(gtx, bg, ui.RadiusMD) },
		func(gtx C) D {
			return layout.Inset{Left: pad, Right: pad}.Layout(gtx, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D { return content(gtx, onFill(th, bg), fg) })
			})
		},
	)
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
		return ui.Stack(1.5, pop.Text(ui.TextXS, title).Weight(font.Medium).Layout, detail)(gtx)
	})
	return tip.Layout(gtx, func(gtx C) D {
		return w.click.Layout(gtx, func(gtx C) D {
			var bg color.NRGBA
			if w.click.Hovered() {
				bg = th.Over(ui.Alpha(th.Accent, readoutHoverOpacity))
			}
			gtx.Constraints.Min.Y = gtx.Dp(ui.Space(8))
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.FillIf(gtx, bg, ui.RadiusMD) },
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
	return func(gtx C) D { return ui.Stack(1, rows...)(gtx) }
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

// branchContent is glyphText for a name that can outgrow the control: it is cut
// with an ellipsis.
func branchContent(ic *icons.Icon, text string) footerContent {
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, footerGlyph, fg) }),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D { return th.Text(ui.TextXS, text).In(fg).Truncated().Layout(gtx) }),
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
func changesContent(git backend.DiffStats) footerContent {
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
			layout.Rigid(func(gtx C) D { return th.DiffStat(git.Added, git.Deleted).Layout(gtx) }),
		)
	}
}

// separatorOpacity is the changes button's opacity-50 dot.
const separatorOpacity = 0.5

// ringContent is the context readout: the ring, then the percentage.
func ringContent(percent int, text string) footerContent {
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return th.DonutGauge(gtx, footerIconGlyph, percent, fg) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, text).In(fg).Layout),
		)
	}
}
