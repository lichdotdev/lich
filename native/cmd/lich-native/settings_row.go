package main

import (
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The row's opacities: a part of a row that cannot be used now dims to half,
// and the search's flash is the wait tone at 14%.
const (
	rowOffOpacity = 0.5
	rowLitFill    = 0.14
)

var (
	rowPadX     = ui.Space(2)
	rowPadY     = ui.Space(3.5)
	rowGap      = ui.Space(6)
	rowTextMinW = ui.Space(40)
	rowNoteSize = ui.TextXS
	icRowFolder = icons.Lucide("folder")
	icRowReset  = icons.Lucide("undo-2")
)

// settingRow is SettingRow: a title and its description on the left, the
// control on the right, and the line under the description that says where
// the value comes from. It lives here rather than in the kit because every
// state it draws is lich's Global/project scoping.
type settingRow struct {
	title, description string
	kind               rowKind
	overriddenIn       []string
	globalValue        string        // rowChanged's "Global: <value>"
	reset              *ui.Clickable // rowChanged's Use Global
	// from names where an unchanged value comes from when it is not Global:
	// the profile a project's chats are born in.
	from string
	lit  float32 // the search's flash, 0 to 1
	// lead is drawn before the title when set: a provider's mark.
	lead layout.Widget
	// detail is a line under the description saying what the control's
	// value does.
	detail string
	// sample is drawn in the description's place when set, for a
	// description that is not text in the interface's font.
	sample layout.Widget
	// warning is drawn under the scope line, for a row whose value something
	// outside it overrides.
	warning func(gtx C, th *ui.Theme) D
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

// frame is what every row draws around its content: the divider and the
// search's flash.
func (r settingRow) frame(gtx C, th *ui.Theme, first bool, content func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	bg := th.Surface
	if r.lit > 0 {
		bg = th.Over(ui.Alpha(th.ToneWait, rowLitFill*r.lit))
	}
	macro := op.Record(gtx.Ops)
	d := content(gtx, th.On(bg))
	call := macro.Stop()
	if r.lit > 0 {
		lit := gtx
		lit.Constraints = layout.Exact(d.Size)
		ui.Fill(lit, bg, ui.RadiusMD)
	}
	call.Add(gtx.Ops)
	return d
}

// content puts the control beside the text, or under it when the control
// would leave the text less than rowTextMinW, as layoutBlock does.
func (r settingRow) content(gtx C, th *ui.Theme, control func(gtx C, th *ui.Theme) D) D {
	return layout.Inset{Left: rowPadX, Right: rowPadX, Top: rowPadY, Bottom: rowPadY}.Layout(gtx, func(gtx C) D {
		loose := gtx
		loose.Constraints.Min = image.Point{}
		drawn := record(loose, func(gtx C) D { return control(gtx, th) })
		textW := func(gtx C) D { return r.text(gtx, th) }
		if gtx.Constraints.Max.X-drawn.size.X-gtx.Dp(rowGap) < gtx.Dp(rowTextMinW) {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(textW), ui.VGap(3), layout.Rigid(drawn.layout))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, textW),
			layout.Rigid(layout.Spacer{Width: rowGap}.Layout),
			layout.Rigid(drawn.layout),
		)
	})
}

// text is the title, the description and the scope line under them.
func (r settingRow) text(gtx C, th *ui.Theme) D {
	desc := th.Text(settingsDescSize, r.description).Muted().Leading(20)
	desc.MaxLines = 0
	descLayout := desc.Layout
	if r.sample != nil {
		descLayout = r.sample
	}
	title := th.Text(ui.TextSM, r.title).Weight(font.Medium).Layout
	if r.lead != nil {
		lead, name := r.lead, title
		title = func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(lead), ui.Gap(2), layout.Rigid(name))
		}
	}
	children := []layout.FlexChild{layout.Rigid(title)}
	if r.description != "" || r.sample != nil {
		children = append(children, ui.VGap(0.5), layout.Rigid(descLayout))
	}
	if r.detail != "" {
		detail := th.Text(rowNoteSize, r.detail).Muted()
		detail.MaxLines = 0
		children = append(children, ui.VGap(1.5), layout.Rigid(detail.Layout))
	}
	if note := r.note(th); note != nil {
		children = append(children, ui.VGap(1), layout.Rigid(note))
	}
	if r.warning != nil {
		children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D { return r.warning(gtx, th) }))
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
		if r.from != "" {
			return th.Text(rowNoteSize, i18n.T("settings.settingsRow.followsFrom", i18n.Args{"name": r.from, "value": r.globalValue})).Muted().Layout
		}
		return th.Text(rowNoteSize, i18n.T("settings.settingsRow.followsGlobal")).Muted().Layout
	case rowProjectOnly:
		return th.Text(rowNoteSize, i18n.T("settings.settingsRow.projectOnly")).Muted().Layout
	case rowChanged:
		value, reset := i18n.T("settings.settingsRow.globalValue", i18n.Args{"value": r.globalValue}), i18n.T("settings.settingsRow.useGlobal")
		if r.from != "" {
			value = i18n.T("settings.settingsRow.fromValue", i18n.Args{"name": r.from, "value": r.globalValue})
			reset = i18n.T("settings.settingsRow.useFrom", i18n.Args{"name": r.from})
		}
		return line(
			layout.Rigid(th.Text(rowNoteSize, value).In(th.ToneWait).Layout),
			ui.Gap(2.5),
			layout.Rigid(func(gtx C) D { return resetButton(gtx, th, r.reset, reset) }),
		)
	}
	return nil
}

// resetButton is the mockup's Use Global: h-22 px-7 rounded, an inset
// border, the foreground's text.
func resetButton(gtx C, th *ui.Theme, click *ui.Clickable, label string) D {
	return click.Layout(gtx, func(gtx C) D {
		macro := op.Record(gtx.Ops)
		gtx.Constraints.Min = image.Point{}
		d := layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Dp(22)
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icRowReset.Layout(gtx, ui.Space(3), th.Foreground) }),
				ui.Gap(1),
				layout.Rigid(th.Text(rowNoteSize, label).Layout),
			)
		})
		call := macro.Stop()
		if click.Hovered() {
			hover := gtx
			hover.Constraints = layout.Exact(d.Size)
			ui.Fill(hover, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), unit.Dp(5))
		}
		ui.InsetBorder(gtx, d.Size, gtx.Dp(5), th.Over(th.Border))
		call.Add(gtx.Ops)
		return d
	})
}
