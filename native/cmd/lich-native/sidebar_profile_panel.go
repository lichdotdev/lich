package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// profilePanelW is the footer panel's width: the context tooltip's bar, the
// panel's padding and the room for a profile's name beside its usage.
var profilePanelW = contextTipWide + ui.Space(2)

// panelBody is what the footer opens: the usage of the shown profile's account,
// a separator, then what the footer's menu always held (the logins to switch
// to, and the other providers').
func (pf *profileFooter) panelBody(gtx C, th *ui.Theme, m *model, f profileFocus, act *sidebarAction) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	plan, loaded := m.profilePlan(f)
	var children []layout.FlexChild
	if usage := profileUsageRows(th, f, plan, loaded, gtx.Now, &pf.reset); len(usage) > 0 {
		inset := layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}
		children = append(children,
			layout.Rigid(func(gtx C) D { return inset.Layout(gtx, ui.Stack(usageBlockGap, usage...)) }),
			layout.Rigid(th.Separator), ui.VGap(1))
	}
	for i, e := range flattenEntries(m.footerEntries(th, f, act)) {
		children = append(children, layout.Rigid(func(gtx C) D { return pf.entryRow(gtx, th, fmt.Sprint(i), e) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// flattenEntries is a menu's entries with each submenu opened in place under a
// label of its own, which is how a panel draws what a menu nests. The submenu
// of submenus (Other providers) gets no label: the providers under it name
// themselves.
func flattenEntries(entries []ui.MenuEntry) []ui.MenuEntry {
	var out []ui.MenuEntry
	for _, e := range entries {
		if e.Kind != ui.MenuSubTrigger {
			out = append(out, e)
			continue
		}
		nested := len(e.Sub) > 0 && e.Sub[0].Kind == ui.MenuSubTrigger
		if !nested {
			out = append(out, ui.MenuEntry{Kind: ui.MenuLabel, Label: e.Label, Icon: e.Icon})
		}
		out = append(out, flattenEntries(e.Sub)...)
	}
	return out
}

// entryRow is one row of the menu part of the panel: a label, a separator, or
// an item with its avatar or icon, its name, its note and a check when chosen.
// Choosing one runs it and closes the panel.
func (pf *profileFooter) entryRow(gtx C, th *ui.Theme, key string, e ui.MenuEntry) D {
	switch e.Kind {
	case ui.MenuSeparator:
		return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, th.Separator)
	case ui.MenuLabel:
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
			children := []layout.FlexChild{}
			if e.Icon != nil {
				children = append(children, layout.Rigid(func(gtx C) D { return e.Icon.Layout(gtx, ui.Space(3.5), th.MutedForeground) }), ui.Gap(1.5))
			}
			children = append(children, layout.Flexed(1, th.Text(ui.TextXS, e.Label).Muted().Layout))
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	}
	if pf.rows == nil {
		pf.rows = map[string]*widget.Clickable{}
	}
	click := clickableOf(pf.rows, key)
	if click.Clicked(gtx) && e.OnClick != nil {
		e.OnClick()
		pf.panel.Close()
	}
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		bg := th.Surface
		if click.Hovered() {
			bg = th.Accent
		}
		on := th.On(bg)
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusSM) },
			func(gtx C) D {
				return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
					children := []layout.FlexChild{}
					if e.Lead != nil {
						children = append(children, layout.Rigid(func(gtx C) D { return e.Lead(gtx, ui.Space(4)) }), ui.Gap(2))
					}
					children = append(children, layout.Flexed(1, on.Text(ui.TextSM, e.Label).Layout))
					if e.Shortcut != "" {
						note := on.Text(ui.TextXS, e.Shortcut).Muted()
						if e.ShortcutDestructive {
							note = on.Text(ui.TextXS, e.Shortcut).In(on.Destructive)
						}
						children = append(children, ui.Gap(2), layout.Rigid(note.Layout))
					}
					if e.Checked {
						children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return icCheck.Layout(gtx, ui.Space(4), on.Foreground) }))
					}
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
				})
			})
	})
}
