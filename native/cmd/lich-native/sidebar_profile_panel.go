package main

import (
	"fmt"
	"gioui.org/font"
	"strings"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/ui"
)

// panelBody is what the account row opens: the usage of its account, then,
// while the provider has logins to switch between, a separator and what the
// footer's menu always held (the logins, and the other providers').
func (pf *profileFooter) panelBody(gtx C, th *ui.Theme, m *model, a sidebarAccount, act *sidebarAction) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	var children []layout.FlexChild
	if usage := profileUsageRows(th, a.profile, a.plan, a.loaded, gtx.Now, &pf.reset); len(usage) > 0 {
		inset := layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}
		children = append(children, layout.Rigid(func(gtx C) D { return inset.Layout(gtx, ui.Stack(usageBlockGap, usage...)) }))
	}
	if !a.switches() {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	if len(children) > 0 {
		children = append(children, ui.VGap(3))
	}
	for i, e := range flattenEntries(m.footerEntries(th, a.profile, act)) {
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
		return layout.Spacer{Height: ui.Space(2)}.Layout(gtx)
	case ui.MenuLabel:
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
			children := []layout.FlexChild{}
			if e.Icon != nil {
				children = append(children, layout.Rigid(func(gtx C) D { return e.Icon.Layout(gtx, ui.Space(3.5), th.MutedForeground) }), ui.Gap(1.5))
			}
			children = append(children, layout.Flexed(1, th.Text(ui.Text2XS, strings.ToUpper(e.Label)).Leading(text2XSLeading).Muted().Weight(font.SemiBold).Layout))
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	}
	if pf.rows == nil {
		pf.rows = map[string]*ui.Clickable{}
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
					// The chosen login's usage is the block above; the others' help to choose.
					if e.Shortcut != "" && !e.Checked {
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
