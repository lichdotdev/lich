package main

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The row's opacities: a global-only row seen from a project dims whole, and
// the search's flash is the wait tone at 14%. A row following Global keeps its
// control at full strength: it can be changed, and "Follows Global" says why.
const (
	rowOffOpacity = 0.5
	rowLitFill    = 0.14
)

var (
	rowPadX      = ui.Space(2)
	rowPadY      = ui.Space(3.5)
	rowChangedPL = ui.Space(3.5)
	rowRule      = unit.Dp(2)
	rowGap       = ui.Space(6)
	rowNoteSize  = ui.TextXS
	rowDescSize  = unit.Sp(13)
	icRowFolder  = icons.Lucide("folder")
	icRowReset   = icons.Lucide("undo-2")
)

// settingRow is SettingRow: a title and its description on the left, the
// control on the right, and the line under the description that says where
// the value comes from. It lives here rather than in the kit because every
// state it draws is lich's Global/project scoping.
type settingRow struct {
	title, description string
	kind               rowKind
	overriddenIn       []string
	globalValue        string            // rowChanged's "Global: <value>"
	reset              *widget.Clickable // rowChanged's Use Global
	lit                float32           // the search's flash, 0 to 1
	// sample is drawn in the description's place when set, for a
	// description that is not text in the interface's font.
	sample layout.Widget
}

// layout draws the row across gtx; first drops the divider above it. The
// control is drawn with the theme it is given, faded for the dimmed states.
func (r settingRow) layout(gtx C, th *ui.Theme, first bool, control func(gtx C, th *ui.Theme) D) D {
	return r.frame(gtx, th, first, func(gtx C, th *ui.Theme) D { return r.content(gtx, th, control) })
}

// layoutBlock is SettingBlock: the title and description over the control
// rather than beside it, for a control too wide to share the line.
func (r settingRow) layoutBlock(gtx C, th *ui.Theme, first bool, under func(gtx C, th *ui.Theme) D) D {
	return r.frame(gtx, th, first, func(gtx C, th *ui.Theme) D {
		return layout.Inset{Left: rowPadX, Right: rowPadX, Top: rowPadY, Bottom: rowPadY}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return r.text(gtx, th) }),
				ui.VGap(3),
				layout.Rigid(func(gtx C) D { return under(gtx, th) }),
			)
		})
	})
}

// frame is what every row draws around its content: the divider, the
// search's flash and the changed rule.
func (r settingRow) frame(gtx C, th *ui.Theme, first bool, content func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	bg := th.Surface
	if r.lit > 0 {
		bg = th.Over(ui.Alpha(th.ToneWait, rowLitFill*r.lit))
	}
	macro := op.Record(gtx.Ops)
	d := content(gtx, th.On(bg))
	call := macro.Stop()
	if !first {
		paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: image.Pt(d.Size.X, gtx.Dp(1))}.Op())
	}
	if r.lit > 0 {
		lit := gtx
		lit.Constraints = layout.Exact(d.Size)
		ui.Fill(lit, bg, ui.RadiusMD)
	}
	if r.kind == rowChanged {
		paint.FillShape(gtx.Ops, th.ToneWait, clip.Rect{Max: image.Pt(gtx.Dp(rowRule), d.Size.Y)}.Op())
	}
	call.Add(gtx.Ops)
	return d
}

func (r settingRow) content(gtx C, th *ui.Theme, control func(gtx C, th *ui.Theme) D) D {
	text := th
	if r.kind == rowGlobalOnly {
		text = fadedTheme(th, rowOffOpacity)
	}
	padL := rowPadX
	if r.kind == rowChanged {
		padL = rowChangedPL
	}
	return layout.Inset{Left: padL, Right: rowPadX, Top: rowPadY, Bottom: rowPadY}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return r.text(gtx, text) }),
			layout.Rigid(layout.Spacer{Width: rowGap}.Layout),
			layout.Rigid(func(gtx C) D {
				if r.kind == rowGlobalOnly {
					return pill(gtx, text, text.Text(ui.TextXS, i18n.T("settings.settingsRow.globalOnly")).Muted().Layout, ui.Space(2), ui.Space(2))
				}
				return control(gtx, th)
			}),
		)
	})
}

// text is the title, the description and the scope line under them.
func (r settingRow) text(gtx C, th *ui.Theme) D {
	desc := th.Text(rowDescSize, r.description).Muted().Leading(20)
	desc.MaxLines = 0
	descLayout := desc.Layout
	if r.sample != nil {
		descLayout = r.sample
	}
	children := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, r.title).Weight(font.Medium).Layout)}
	if r.description != "" || r.sample != nil {
		children = append(children, ui.VGap(0.5), layout.Rigid(descLayout))
	}
	if note := r.note(th); note != nil {
		children = append(children, ui.VGap(1), layout.Rigid(note))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// note is the scope line, nil for a row with nothing to say.
func (r settingRow) note(th *ui.Theme) layout.Widget {
	line := func(children ...layout.FlexChild) layout.Widget {
		return func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...) }
	}
	switch r.kind {
	case rowOverridden:
		return line(
			layout.Rigid(func(gtx C) D { return icRowFolder.Layout(gtx, ui.Space(3), th.ToneWait) }),
			ui.Gap(1.25),
			layout.Rigid(th.Text(rowNoteSize, i18n.T("settings.settingsRow.overriddenIn")).In(th.ToneWait).Layout),
			ui.Gap(1),
			layout.Rigid(th.Text(rowNoteSize, strings.Join(r.overriddenIn, ", ")).In(th.ToneWait).Weight(font.SemiBold).Layout),
		)
	case rowFollows:
		return th.Text(rowNoteSize, i18n.T("settings.settingsRow.followsGlobal")).Muted().Layout
	case rowProjectOnly:
		return th.Text(rowNoteSize, i18n.T("settings.settingsRow.projectOnly")).Muted().Layout
	case rowChanged:
		return line(
			layout.Rigid(th.Text(rowNoteSize, i18n.T("settings.settingsRow.globalValue", i18n.Args{"value": r.globalValue})).In(th.ToneWait).Layout),
			ui.Gap(2.5),
			layout.Rigid(func(gtx C) D { return resetButton(gtx, th, r.reset) }),
		)
	}
	return nil
}

// resetButton is the mockup's Use Global: h-22 px-7 rounded, an inset
// border, the foreground's text.
func resetButton(gtx C, th *ui.Theme, click *widget.Clickable) D {
	return click.Layout(gtx, func(gtx C) D {
		macro := op.Record(gtx.Ops)
		gtx.Constraints.Min = image.Point{}
		d := layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Dp(22)
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icRowReset.Layout(gtx, ui.Space(3), th.Foreground) }),
				ui.Gap(1),
				layout.Rigid(th.Text(rowNoteSize, i18n.T("settings.settingsRow.useGlobal")).Layout),
			)
		})
		call := macro.Stop()
		if click.Hovered() {
			hover := gtx
			hover.Constraints = layout.Exact(d.Size)
			ui.Fill(hover, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), unit.Dp(5))
		}
		insetBorder(gtx, d.Size, gtx.Dp(5), th.Over(th.Border))
		call.Add(gtx.Ops)
		return d
	})
}

// fadedTheme is th with every ink token at opacity over its surface, the
// opacity-* of a dimmed row composited the way a browser would.
func fadedTheme(th *ui.Theme, opacity float32) *ui.Theme {
	fade := func(c color.NRGBA) color.NRGBA { return ui.Over(th.Surface, ui.Alpha(ui.Over(th.Surface, c), opacity)) }
	f := *th
	f.Foreground, f.MutedForeground = fade(th.Foreground), fade(th.MutedForeground)
	f.Primary, f.Palette.Input, f.Border = fade(th.Primary), fade(th.Palette.Input), fade(th.Border)
	f.Accent, f.Background = fade(th.Accent), fade(th.Background)
	return &f
}

// insetBorder strokes a 1px line just inside size, a box-shadow inset 0 0 0
// 1px.
func insetBorder(gtx C, size image.Point, r int, c color.NRGBA) {
	insetRing(gtx, size, r, gtx.Dp(1), c)
}

// insetRing strokes a line width pixels wide just inside size, ring-inset:
// a stroke twice as wide on the edge, clipped to the inside half.
func insetRing(gtx C, size image.Point, r, width int, c color.NRGBA) {
	rr := clip.UniformRRect(image.Rectangle{Max: size}, r)
	defer rr.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(2 * width)}.Op())
}
