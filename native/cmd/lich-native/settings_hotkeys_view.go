package main

import (
	"image"
	"runtime"
	"strings"

	"gioui.org/io/event"
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

var (
	icHotkeyConflict = icons.Lucide("triangle-alert")
	icHotkeyUnassign = icons.Lucide("ban")
	icHotkeyReset    = icons.Lucide("rotate-ccw")
)

// The row's measures: the capture button's minimum width, the row's gap, and
// what the shortcut lines and the terminal notes measure from them.
var (
	hotkeyCaptureMin = unit.Dp(144)
	hotkeyRowGap     = ui.Space(4)
	hotkeyRowPadY    = ui.Space(1.5)
	hotkeyLinePadX   = ui.Space(3)
	hotkeyNoteGap    = ui.Space(2)
	hotkeyKeyCapSize = unit.Sp(10) // text-[0.625rem]
)

// hotkeyNoteCh is the note's max-w-prose, a ch taken as 0.6em as settingsDescCh
// is.
const hotkeyNoteCh = 65

// hotkeyHoverFill and hotkeyLitFill are the row's hover:bg-accent/50 and the
// search's bg-accent/60.
const (
	hotkeyHoverFill = 0.5
	hotkeyLitFill   = 0.6
)

// shortcutRow is one fixed line of the passed-through group.
type shortcutRow struct{ label, keys string }

// passthroughRows are spelled out rather than built from a combo: they are
// matched on Control, so the modifier is Ctrl on macOS too.
func passthroughRows(goos string) []shortcutRow {
	attach := "Ctrl+V"
	switch goos {
	case "windows":
		attach = "Alt+V"
	case "darwin":
		attach = "⌘V or Ctrl+V"
	}
	return []shortcutRow{
		{i18n.T("hotkeys.shortcuts.attachImage"), attach},
		{i18n.T("hotkeys.shortcuts.newline"), "Shift+Enter"},
		{i18n.T("hotkeys.shortcuts.eraseWord"), "Ctrl+Backspace"},
	}
}

// hotkeyPaneRows are the Hotkeys pane's list children: a group per part of
// the app, then the passed-through group under its note.
func (s *settingsScreen) hotkeyPaneRows(th *ui.Theme) []layout.Widget {
	var rows []layout.Widget
	bound := s.hotkeys.current()
	conflicts := hotkeyConflicts(bound)
	index := 0
	for g, group := range hotkeyGroups {
		rows = append(rows, groupHeading(th, hotkeyGroupText(group.id), g == 0))
		for _, a := range hotkeyActions {
			if a.group != group.id {
				continue
			}
			if index == s.scrollTo {
				s.pane.ScrollTo(len(rows))
			}
			index++
			rows = append(rows, func(gtx C) D { return s.hotkeyRow(gtx, th, a, conflicts[a.id]) })
		}
	}
	s.scrollTo = -1
	rows = append(rows, groupHeading(th, i18n.T("hotkeys.shortcuts.passthroughTitle"), false), hotkeyNote(th, i18n.T("settings.hotkeysSettings.passthroughNote")))
	for _, r := range passthroughRows(runtime.GOOS) {
		rows = append(rows, func(gtx C) D { return shortcutLine(gtx, th, r) })
	}
	return rows
}

// hotkeyNote is a group's explanation: text-xs, muted, mb-2, max-w-prose,
// under the heading it explains.
func hotkeyNote(th *ui.Theme, text string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: rowPadX, Bottom: hotkeyNoteGap}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Sp(ui.TextXS)*hotkeyNoteCh*6/10)
			l := th.Text(ui.TextXS, text).Muted()
			l.MaxLines = 0
			return l.Layout(gtx)
		})
	}
}

// hotkeyRow is HotkeyRow: the action's name, the capture button with the
// chord, and the two opposite moves, unassign and reset.
func (s *settingsScreen) hotkeyRow(gtx C, th *ui.Theme, a hotkeyAction, conflicts []hotkeyID) D {
	h := &s.hotkeys
	r := h.row(a.id)
	if r.capture.Clicked(gtx) {
		h.startRecording(a.id)
	}
	if h.recording == a.id {
		s.capture(gtx, a.id, r)
	}
	recording := h.recording == a.id
	if r.unassign.Clicked(gtx) {
		s.setHotkey(a.id, comboUnassigned)
	}
	if r.reset.Clicked(gtx) {
		s.resetHotkey(a.id)
	}
	c := h.current()[a.id]
	lit := float32(0)
	if s.lit == hotkeyActionText(a.id) && gtx.Now.Before(s.litUntil) {
		lit = min(1, float32(s.litUntil.Sub(gtx.Now))/float32(settingsLitFadingFor))
		gtx.Execute(op.InvalidateCmd{})
	}
	return r.hover.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		macro := op.Record(gtx.Ops)
		d := layout.Inset{Left: rowPadX, Right: rowPadX, Top: hotkeyRowPadY, Bottom: hotkeyRowPadY}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return hotkeyName(gtx, th, hotkeyActionText(a.id), hotkeyLabels(conflicts), terminalCost(c))
				}),
				layout.Rigid(layout.Spacer{Width: hotkeyRowGap}.Layout),
				layout.Rigid(func(gtx C) D { return captureButton(gtx, th, r, c, recording, len(conflicts) > 0) }),
				layout.Rigid(layout.Spacer{Width: hotkeyRowGap}.Layout),
				layout.Rigid(func(gtx C) D {
					return hotkeyIconButton(gtx, th, s.root, &r.unassign, &r.unassignTip, icHotkeyUnassign, i18n.T("settings.hotkeysSettings.unassignHint"), c.Key == "")
				}),
				layout.Rigid(layout.Spacer{Width: hotkeyRowGap}.Layout),
				layout.Rigid(func(gtx C) D {
					return hotkeyIconButton(gtx, th, s.root, &r.reset, &r.resetTip, icHotkeyReset, i18n.T("settings.hotkeysSettings.resetHint"), c == defaultHotkeys()[a.id])
				}),
			)
		})
		call := macro.Stop()
		switch {
		case lit > 0:
			fillRow(gtx, th, d.Size, hotkeyLitFill*lit)
		case r.hover.Hovered():
			fillRow(gtx, th, d.Size, hotkeyHoverFill)
		}
		call.Add(gtx.Ops)
		return d
	})
}

func fillRow(gtx C, th *ui.Theme, size image.Point, alpha float32) {
	fill := gtx
	fill.Constraints = layout.Exact(size)
	ui.Fill(fill, th.Over(ui.Alpha(th.Accent, alpha)), ui.RadiusMD)
}

// hotkeyName is the row's left column: the action, the amber line naming who
// else holds its chord, and what the chord costs the terminal.
func hotkeyName(gtx C, th *ui.Theme, label string, conflicts []string, cost string) D {
	children := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, label).Layout)}
	if len(conflicts) > 0 {
		line := th.Text(ui.TextXS, i18n.T("settings.hotkeysSettings.alsoBound", i18n.Args{"actions": strings.Join(conflicts, ", ")})).In(th.ToneWait)
		line.MaxLines = 0
		children = append(children, ui.VGap(0.5), layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx C) D { return icHotkeyConflict.Layout(gtx, ui.Space(3), th.ToneWait) })
				}),
				ui.Gap(1),
				layout.Flexed(1, line.Layout),
			)
		}))
	}
	if cost != "" {
		line := th.Text(ui.TextXS, cost).Muted()
		line.MaxLines = 0
		children = append(children, ui.VGap(0.5), layout.Rigid(line.Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// captureButton is the chord in a bordered button: ring while it records,
// amber at 60% for a chord another action holds, muted for none.
func captureButton(gtx C, th *ui.Theme, r *hotkeyRow, c combo, recording, conflict bool) D {
	label, ink := formatCombo(c, runtime.GOOS == "darwin"), th.Foreground
	switch {
	case recording:
		label, ink = i18n.T("settings.hotkeysSettings.pressKeys"), th.MutedForeground
	case c.Key == "":
		ink = th.MutedForeground
	}
	return r.capture.Layout(gtx, func(gtx C) D {
		macro := op.Record(gtx.Ops)
		gtx.Constraints.Min = image.Point{}
		d := layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, th.Text(ui.TextSM, label).In(ink).Layout)
		call := macro.Stop()
		size := image.Pt(max(d.Size.X, gtx.Dp(hotkeyCaptureMin)), d.Size.Y)
		border := th.Over(th.Border)
		switch {
		case recording:
			border = th.Ring
		case conflict:
			border = th.Over(ui.Alpha(th.ToneWait, 0.6))
		}
		if !recording && r.capture.Hovered() {
			fillRow(gtx, th, size, 1)
		}
		insetBorder(gtx, size, gtx.Dp(ui.RadiusMD), border)
		call.Add(gtx.Ops)
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, &r.tag)
		return D{Size: size}
	})
}

// hotkeyIconButton is a ghost icon button under its tooltip, dimmed and inert while
// disabled.
func hotkeyIconButton(gtx C, th *ui.Theme, root *ui.Root, click *widget.Clickable, tip *ui.Tooltip, ic *icons.Icon, hint string, disabled bool) D {
	b := th.Button(click, "")
	b.Icon, b.Variant, b.Size, b.Disabled = ic, ui.VariantGhost, ui.SizeIcon, disabled
	return th.Tooltip(root, tip, hint).Layout(gtx, b.Layout)
}

// shortcutLine is ShortcutLine: what it does on the left, the chord as a key
// cap on the right. An unassigned action has no chord to cap, and a cap
// would read as a key to press.
func shortcutLine(gtx C, th *ui.Theme, r shortcutRow) D {
	return layout.Inset{Left: hotkeyLinePadX, Right: hotkeyLinePadX, Top: hotkeyRowPadY, Bottom: hotkeyRowPadY}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, th.Text(ui.TextSM, r.label).Layout),
			layout.Rigid(layout.Spacer{Width: hotkeyRowGap}.Layout),
			layout.Rigid(func(gtx C) D {
				if r.keys == hotkeyUnassigned {
					return th.Text(ui.TextXS, r.keys).Muted().Layout(gtx)
				}
				return keyCap(gtx, th, r.keys)
			}),
		)
	})
}

// keyCap is Keys: rounded, a hairline border with a thicker bottom one, on
// bg-muted, 10px mono text.
func keyCap(gtx C, th *ui.Theme, keys string) D {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	d := layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5) + 1}.Layout(gtx,
		th.Text(hotkeyKeyCapSize, keys).Mono().Muted().Leading(hotkeyKeyCapSize).Layout)
	call := macro.Stop()
	box := gtx
	box.Constraints = layout.Exact(d.Size)
	ui.Fill(box, th.Over(th.Palette.Muted), ui.RadiusSM)
	border := th.Over(th.Border)
	insetBorder(gtx, d.Size, gtx.Dp(ui.RadiusSM), border)
	r := gtx.Dp(ui.RadiusSM)
	paint.FillShape(gtx.Ops, border, clip.Rect{Min: image.Pt(r, d.Size.Y-2*gtx.Dp(1)), Max: image.Pt(d.Size.X-r, d.Size.Y-gtx.Dp(1))}.Op())
	call.Add(gtx.Ops)
	return d
}
