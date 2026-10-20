package main

import (
	"runtime"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const shortcutsEsc = "esc"

const (
	shortcutsWidth unit.Dp = 544
	// shortcutsScrim is the scrim's opacity, the palette's and not a dialog's.
	shortcutsScrim = 0.5
	// shortcutsBodyHeight is the share of the room the panel has left that the
	// list may take.
	shortcutsBodyHeight = 80
)

// shortcutGroup is a titled run of listed shortcuts.
type shortcutGroup struct {
	title string
	rows  []shortcutRow
}

// shortcutGroupsOf is shortcutGroups: the actions of every group at the
// chords bound now, then the passed-through chords.
func shortcutGroupsOf(bound hotkeyBindings, goos string) []shortcutGroup {
	groups := make([]shortcutGroup, 0, len(hotkeyGroups)+1)
	for _, g := range hotkeyGroups {
		group := shortcutGroup{title: i18n.T(g.labelKey)}
		for _, a := range hotkeyActions {
			if a.group == g.id {
				group.rows = append(group.rows, shortcutRow{i18n.T(a.labelKey), formatCombo(bound[a.id], goos == "darwin")})
			}
		}
		groups = append(groups, group)
	}
	return append(groups, shortcutGroup{title: i18n.T("hotkeys.shortcuts.passthroughTitle"), rows: passthroughRows(goos)})
}

// shortcutsOverlay is ShortcutsOverlay: what is bound, listed and not
// editable.
type shortcutsOverlay struct {
	open   bool
	dialog ui.DialogState
	list   widget.List
}

// layout draws the overlay over the window. Call it before the content under
// it, so it takes Escape ahead of any key filter there.
func (o *shortcutsOverlay) layout(gtx C, th *ui.Theme, bound hotkeyBindings) {
	on := th.On(th.Popover)
	dlg := th.Dialog(&o.dialog, o.open, i18n.T("shell.shortcutsOverlay.title"), i18n.T("shell.shortcutsOverlay.rebind"))
	dlg.MaxWidth, dlg.HideClose, dlg.Scrim = shortcutsWidth, true, shortcutsScrim
	dlg.Body = func(gtx C) D { return o.body(gtx, on, shortcutGroupsOf(bound, runtime.GOOS)) }
	dlg.Actions = []layout.Widget{func(gtx C) D { return closeHint(gtx, on) }}
	if dlg.Layout(gtx) {
		o.open = false
	}
}

func (o *shortcutsOverlay) body(gtx C, th *ui.Theme, groups []shortcutGroup) D {
	var rows []layout.Widget
	for _, g := range groups {
		rows = append(rows, shortcutsHeading(th, g.title))
		for _, r := range g.rows {
			rows = append(rows, func(gtx C) D { return shortcutLine(gtx, th, r) })
		}
	}
	o.list.Axis = layout.Vertical
	gtx.Constraints.Max.Y = gtx.Constraints.Max.Y * shortcutsBodyHeight / 100
	return th.ScrollArea(&o.list).Layout(gtx, len(rows), func(gtx C, i int) D { return rows[i](gtx) })
}

// shortcutsHeading is the group's px-3 pt-3 pb-1 uppercase label.
func shortcutsHeading(th *ui.Theme, title string) layout.Widget {
	inset := layout.Inset{Left: hotkeyLinePadX, Right: hotkeyLinePadX, Top: ui.Space(3), Bottom: ui.Space(1)}
	return func(gtx C) D {
		return inset.Layout(gtx, th.Text(text2XS, strings.ToUpper(title)).Muted().Weight(font.SemiBold).Layout)
	}
}

// closeHint is the footer's key cap and the word beside it.
func closeHint(gtx C, th *ui.Theme) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return keyCap(gtx, th, shortcutsEsc) }),
		ui.Gap(1.5),
		layout.Rigid(th.Text(ui.TextXS, i18n.T("shell.shortcutsOverlay.close")).Muted().Layout),
	)
}

// hotkeyShortcuts is the chord's toggle: the same one closes it again.
func (u *window) hotkeyShortcuts(C) {
	u.shortcuts.open = !u.shortcuts.open
}
