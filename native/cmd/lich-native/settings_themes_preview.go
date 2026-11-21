package main

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The detail's preview, the whole window in the theme, from the approved
// mockup's .bp (temas-catalogo, frame 2).
var (
	previewTabsH     = unit.Dp(30) // .tabsrow height 30px
	previewTabPad    = layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9), Top: unit.Dp(2), Bottom: unit.Dp(2)}
	previewAreaGap   = unit.Dp(5) // .areas gap 5px, padding 0 6px 6px
	previewAreaPad   = unit.Dp(6)
	previewSideW     = unit.Dp(170) // .sb width 170px
	previewSidePad   = unit.Dp(7)
	previewRowH      = ui.Space(6) // .sb .r height 24px
	previewRowPadX   = unit.Dp(6)
	previewIslandR   = ui.RadiusMD
	previewStagePad  = layout.Inset{Top: ui.Space(3), Bottom: ui.Space(3), Left: ui.Space(4), Right: ui.Space(4)}
	previewStageGap  = ui.Space(2)
	previewBubblePad = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(11), Right: unit.Dp(11)}
	previewCodePad   = layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(11), Right: unit.Dp(11)}
	previewTermPad   = layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(2.5), Right: ui.Space(2.5)}
	previewSwatch    = image.Pt(16, 10) // .ansi b 16x10
	previewSwatchGap = unit.Dp(3)
	previewSendSize  = unit.Dp(22)
	previewMarkBox   = ui.Space(4)
	previewMarkCell  = unit.Dp(1)
)

const (
	previewText        = unit.Sp(12) // .st font-size 12px
	previewTextLeading = unit.Sp(17)
	previewSmall       = unit.Sp(11) // .bp pre, .termp, .tabsrow span
	previewCodeLeading = unit.Sp(16.5)
	// previewComposerMix is the composer's edge, the border 40% of the way to
	// the primary.
	previewComposerMix = 0.4
)

// previewSession is a row of the preview's sidebar: the provider it runs, the
// name's key and the branch.
type previewSession struct{ provider, nameKey, branch string }

var previewSessions = []previewSession{
	{"claude", "settings.themesPage.preview.sessionRefactor", "main"},
	{"codex", "settings.themesPage.preview.sessionFlaky", "fix/ci"},
	{"crush", "settings.themesPage.preview.sessionReview", "pr-42"},
}

// themePreview is the window in t: the tabs, the sidebar and a session's chat,
// in size.
func themePreview(gtx C, th *ui.Theme, t themeLook, size image.Point) D {
	p := t.palette
	gtx.Constraints = layout.Exact(size)
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusLG)).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, p.Canvas)
	tabsH, gap, pad := gtx.Dp(previewTabsH), gtx.Dp(previewAreaGap), gtx.Dp(previewAreaPad)
	previewTabs(gtx, th, p, tabsH)
	side := image.Rect(pad, tabsH, pad+gtx.Dp(previewSideW), size.Y-pad)
	stage := image.Rect(side.Max.X+gap, tabsH, size.X-pad, size.Y-pad)
	previewIsland(gtx, th, p.Sidebar, p.Border, side, func(gtx C, th *ui.Theme) D { return previewSidebar(gtx, th, p) })
	previewIsland(gtx, th, p.Background, p.Border, stage, func(gtx C, th *ui.Theme) D { return previewStage(gtx, th, t) })
	ui.InsetBorder(gtx, size, gtx.Dp(ui.RadiusLG), th.Over(th.Border))
	return D{Size: size}
}

// previewTabs is the top strip: the project in a pill, another beside it.
func previewTabs(gtx C, th *ui.Theme, p ui.Palette, height int) {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
	layout.Inset{Left: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.Y = 0
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return inkBox(gtx, p.Accent, ui.RadiusSM, previewTabPad, th.Text(previewSmall, "lich").In(p.Foreground).Layout)
				}),
				ui.Gap(1.5),
				layout.Rigid(func(gtx C) D {
					return previewTabPad.Layout(gtx, th.Text(previewSmall, "todo-api").In(p.MutedForeground).Layout)
				}),
			)
		})
	})
}

// previewIsland is w on a rounded island of bg at box, its hairline edge in
// border.
func previewIsland(gtx C, th *ui.Theme, bg, border color.NRGBA, box image.Rectangle, w func(gtx C, th *ui.Theme) D) {
	defer op.Offset(box.Min).Push(gtx.Ops).Pop()
	size := box.Size()
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(previewIslandR)).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bg)
	gtx.Constraints = layout.Exact(size)
	w(gtx, th)
	ui.InsetBorder(gtx, size, gtx.Dp(previewIslandR), border)
}

// previewSidebar is New Session over three sessions, the first selected.
func previewSidebar(gtx C, th *ui.Theme, p ui.Palette) D {
	rows := []layout.FlexChild{layout.Rigid(func(gtx C) D {
		return previewRow(gtx, p.Sidebar, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icPlus.Layout(gtx, ui.Space(3), p.Foreground) }),
				ui.Gap(1.5),
				layout.Rigid(th.Text(previewSmall, i18n.T("sidebar.sessionSidebar.newSessionButton")).In(p.Foreground).Layout),
			)
		})
	})}
	for i, s := range previewSessions {
		bg := p.Sidebar
		if i == 0 {
			bg = p.Accent
		}
		rows = append(rows, layout.Rigid(func(gtx C) D {
			return previewRow(gtx, bg, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, s.provider, previewMarkBox, previewMarkCell) }),
					ui.Gap(1.5),
					layout.Flexed(1, th.Text(previewSmall, i18n.T(s.nameKey)).In(p.Foreground).Truncated().Layout),
					layout.Rigid(th.Text(ui.Text2XS, s.branch).Mono().In(p.MutedForeground).Layout),
				)
			})
		}))
	}
	return layout.UniformInset(previewSidePad).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

// previewRow is one sidebar row on bg, its content padded and centred.
func previewRow(gtx C, bg color.NRGBA, w layout.Widget) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(previewRowH))
	paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusSM)).Op(gtx.Ops))
	gtx.Constraints = layout.Exact(size)
	layout.Inset{Left: previewRowPadX, Right: previewRowPadX}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, w)
	})
	return D{Size: size}
}

// previewStage is a session's chat: the person's bubble, the agent's answer
// with its code, the Done line, the terminal's colors and the composer.
func previewStage(gtx C, th *ui.Theme, t themeLook) D {
	p := t.palette
	gap := layout.Rigid(layout.Spacer{Height: previewStageGap}.Layout)
	text := func(s string) ui.LabelStyle {
		return th.Text(previewText, s).In(p.Foreground).Leading(previewTextLeading)
	}
	return previewStagePad.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return inkBox(gtx, p.Accent, ui.RadiusLG, previewBubblePad, text(i18n.T("settings.themesPage.preview.ask")).Layout)
				})
			}),
			gap,
			layout.Rigid(func(gtx C) D { return previewAgentLine(gtx, th, p) }),
			gap,
			layout.Rigid(text(i18n.T("settings.themesPage.preview.reply")).Wrapped().Layout),
			gap,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return inkBox(gtx, p.Card, ui.RadiusMD, previewCodePad, func(gtx C) D {
					return codeLines(gtx, th, previewCode(t), previewSmall, previewCodeLeading)
				})
			}),
			gap,
			layout.Rigid(func(gtx C) D { return previewDone(gtx, th, p) }),
			gap,
			layout.Rigid(func(gtx C) D { return previewTerminal(gtx, th, t) }),
			layout.Flexed(1, func(gtx C) D {
				return layout.S.Layout(gtx, func(gtx C) D { return previewComposer(gtx, th, p) })
			}),
		)
	})
}

// previewAgentLine is the answer's header: the provider's mark, its name, and
// the model and effort muted.
func previewAgentLine(gtx C, th *ui.Theme, p ui.Palette) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, "claude", previewMarkBox, previewMarkCell) }),
		ui.Gap(2),
		layout.Rigid(th.Text(previewText, "Claude Code").In(p.Foreground).Weight(font.Medium).Layout),
		layout.Rigid(th.Text(previewText, " · Opus 5.5 · high").In(p.MutedForeground).Layout),
	)
}

// previewCode is the answer's Go in the theme's syntax.
func previewCode(t themeLook) [][]inkSpan {
	fg, k := t.palette.Foreground, t.ink
	return [][]inkSpan{
		{{"for", k.keyword}, {" attempt := ", fg}, {"0", k.number}, {"; attempt < ", fg}, {"max", k.variable}, {"; attempt++ {", fg}},
		{{"  resp, err := ", fg}, {"fetch", k.fn}, {"(ctx, ", fg}, {`"https://api"`, k.str}, {")", fg}},
		{{"  ", fg}, {"// done when the server answers", k.comment}},
		{{"  ", fg}, {"if", k.keyword}, {" err == ", fg}, {"nil", k.null}, {" { ", fg}, {"return", k.keyword}, {" resp, ", fg}, {"nil", k.null}, {" }", fg}},
		{{"}", fg}},
	}
}

// previewDone is the turn's end: Done, the files and the lines in the tones,
// and a rule to the edge.
func previewDone(gtx C, th *ui.Theme, p ui.Palette) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Text(previewText, i18n.T("chat.done.done")).In(p.Foreground).Weight(font.Medium).Layout),
		ui.Gap(2),
		layout.Rigid(th.Text(previewText, "· "+i18n.T("chat.done.files", i18n.Args{"count": 2})).In(p.MutedForeground).Layout),
		ui.Gap(2),
		layout.Rigid(th.Text(previewText, "+18").In(p.TonePass).Layout),
		ui.Gap(1.5),
		layout.Rigid(th.Text(previewText, "−4").In(p.Destructive).Layout),
		ui.Gap(2),
		layout.Flexed(1, func(gtx C) D {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
			paint.FillShape(gtx.Ops, p.Border, clip.Rect{Max: size}.Op())
			return D{Size: size}
		}),
	)
}

// previewTerminal is a test run in the terminal's colors over a swatch of all
// sixteen.
func previewTerminal(gtx C, th *ui.Theme, t themeLook) D {
	c := t.terminal
	run := func(s string, i int) layout.FlexChild {
		return layout.Rigid(th.Text(previewSmall, s).Mono().In(ansiOf(t, i)).Leading(previewCodeLeading).Layout)
	}
	plain := func(s string) layout.FlexChild {
		return layout.Rigid(th.Text(previewSmall, s).Mono().In(c.Foreground).Leading(previewCodeLeading).Layout)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	macro := op.Record(gtx.Ops)
	d := previewTermPad.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return layout.Flex{}.Layout(gtx, run("ok", 2), plain("  lich/fetch  0.41s")) }),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{}.Layout(gtx, run("FAIL", 1), plain(" lich/net  "), run("timeout", 3))
			}),
			ui.VGap(1.5),
			layout.Rigid(func(gtx C) D { return ansiSwatches(gtx, t) }),
		)
	})
	call := macro.Stop()
	paint.FillShape(gtx.Ops, c.Background, clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(ui.RadiusSM)).Op(gtx.Ops))
	call.Add(gtx.Ops)
	ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusSM), t.palette.Border)
	return d
}

// ansiSwatches is the sixteen terminal colors in a row.
func ansiSwatches(gtx C, t themeLook) D {
	w, h, gap := gtx.Dp(unit.Dp(previewSwatch.X)), gtx.Dp(unit.Dp(previewSwatch.Y)), gtx.Dp(previewSwatchGap)
	for i := range len(ansiTokens) {
		r := image.Rect(i*(w+gap), 0, i*(w+gap)+w, h)
		paint.FillShape(gtx.Ops, ansiOf(t, i), clip.UniformRRect(r, gtx.Dp(coverRowRadius)).Op(gtx.Ops))
	}
	return D{Size: image.Pt(len(ansiTokens)*(w+gap)-gap, h)}
}

// previewComposer is the composer's outline leaning to the primary, its
// placeholder, and the send button in the primary.
func previewComposer(gtx C, th *ui.Theme, p ui.Palette) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	d := previewCodePad.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, th.Text(previewText, i18n.T("chat.composer.placeholder", i18n.Args{"provider": "Claude Code"})).In(p.MutedForeground).Truncated().Layout),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(previewSendSize), gtx.Dp(previewSendSize)))
				return ui.Fill(gtx, p.Primary, ui.RadiusMD)
			}),
		)
	})
	ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusLG), ui.MixOKLab(p.Border, p.Primary, previewComposerMix))
	return d
}
