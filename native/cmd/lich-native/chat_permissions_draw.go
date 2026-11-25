package main

import (
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// permissionsView is the /permissions panel: the Allow, Ask, Deny and
// Folders tabs and the tab shown, the add form or a rule's detail; Codex's
// modes for a provider that reports no rules.
func (u *window) permissionsView(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	t := &p.tools
	t.mu.Lock()
	l, note, running := t.set.perms, t.set.note, t.set.running
	t.mu.Unlock()
	if !l.Reported {
		return u.modesView(gtx, th, t, sv)
	}
	switch t.detail {
	case "":
	case permAddDetail:
		return t.addRuleView(gtx, th, l, running)
	default:
		return t.ruleDetail(gtx, th, l, running)
	}
	var lines []layout.Widget
	if t.set.tabs.Selected == permTabFolders {
		lines = folderLines(th, l)
	} else {
		lines = t.ruleLines(th, l)
	}
	var foot []string
	if note != "" {
		foot = append(foot, note)
	}
	if l.Locked {
		foot = append(foot, i18n.T("chat.permissions.locked"))
	}
	if len(l.Errors) > 0 {
		foot = append(foot, i18n.T("chat.permissions.skipped", i18n.Args{"files": strings.Join(l.Errors, "; ")}))
	}
	return t.scrolled(gtx, th, func(gtx C) D { return t.permissionsHead(gtx, th, l) }, lines, footNotes(th, foot))
}

func (t *chatTools) permissionsHead(gtx C, th *ui.Theme, l api.ChatPermissionList) D {
	tabs := th.Tabs(&t.set.tabs,
		ui.TabsTrigger{Text: i18n.T("chat.permissions.tab.allow")},
		ui.TabsTrigger{Text: i18n.T("chat.permissions.tab.ask")},
		ui.TabsTrigger{Text: i18n.T("chat.permissions.tab.deny")},
		ui.TabsTrigger{Text: i18n.T("chat.permissions.tab.folders")},
	)
	tabs.Variant = ui.TabsVariantGhost
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return toolsHead(gtx, th, i18n.T("chat.permissions.title"), i18n.T("chat.permissions.ruleCount", i18n.Args{"count": len(l.Rules)}), nil)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = 0
			d := layout.Inset{Left: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D { return tabs.Layout(gtx) })
			if t.set.tabs.Changed() {
				t.cursor, t.moved, t.focus = 0, true, true
				t.mu.Lock()
				t.set.note = ""
				t.mu.Unlock()
				gtx.Execute(op.InvalidateCmd{})
			}
			return d
		}),
	)
}

// ruleLines is the tab's rows: Add a rule first where a file takes one, then
// its rules, or a line saying there are none.
func (t *chatTools) ruleLines(th *ui.Theme, l api.ChatPermissionList) []layout.Widget {
	rules := tabRules(l, permTabBehaviors[t.set.tabs.Selected])
	add := len(l.Destinations) > 0
	t.ensureRows(len(rules) + 1)
	var lines []layout.Widget
	row := 0
	if add {
		lines = append(lines, func(gtx C) D {
			return toolsPick(gtx, th, &t.rows[0], t.cursor == 0, func(gtx C, th *ui.Theme) D {
				return iconRow(gtx, th, func(gtx C) D { return icTabNew.Layout(gtx, toolsGlyph, th.MutedForeground) }, th.Text(ui.TextSM, i18n.T("chat.permissions.add")).Layout)
			})
		})
		row = 1
	}
	if len(rules) == 0 {
		return append(lines, func(gtx C) D { return toolsNote(gtx, th, i18n.T("chat.permissions.noRules")) })
	}
	for i, r := range rules {
		at := row + i
		lines = append(lines, func(gtx C) D {
			return toolsPick(gtx, th, &t.rows[at], t.cursor == at, func(gtx C, th *ui.Theme) D { return ruleRow(gtx, th, r) })
		})
	}
	return lines
}

func ruleRow(gtx C, th *ui.Theme, r api.ChatPermissionRule) D {
	name := th.Text(ui.TextSM, cleanLine(r.Rule))
	glyph := func(gtx C) D { return icMode.Layout(gtx, toolsGlyph, th.MutedForeground) }
	if r.Ignored {
		name = name.Muted()
		glyph = func(gtx C) D { return icToolsOff.Layout(gtx, toolsGlyph, th.MutedForeground) }
	}
	end := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, permSourceLabel(r.Source)).Muted().Layout)}
	return descRow(gtx, th, glyph, name.Truncated().Layout, r.Description, end)
}

// iconRow is a glyph and a line.
func iconRow(gtx C, th *ui.Theme, glyph, text layout.Widget) D {
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(glyph), ui.Gap(2), layout.Flexed(1, text))
	})
}

// folderLines is the folder the session started in, then those it may also
// work in, each with where it was granted.
func folderLines(th *ui.Theme, l api.ChatPermissionList) []layout.Widget {
	lines := []layout.Widget{folderLine(th, l.Cwd, i18n.T("chat.permissions.startedIn"))}
	for _, f := range l.Folders {
		lines = append(lines, folderLine(th, f.Path, permSourceLabel(f.Source)))
	}
	return lines
}

func folderLine(th *ui.Theme, path, meta string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return icFolder.Layout(gtx, toolsGlyph, th.MutedForeground) }),
				ui.Gap(2),
				layout.Flexed(1, th.Text(ui.TextSM, path).Truncated().Layout),
				ui.Gap(toolsMetaGap),
				layout.Rigid(th.Text(ui.TextXS, meta).Muted().Layout),
			)
		})
	}
}

// detailHead is a detail's Back and its title.
func (t *chatTools) detailHead(th *ui.Theme, back, title string) layout.Widget {
	return func(gtx C) D {
		return toolsBackHead(gtx, th, &t.back, back, th.Text(ui.TextSM, title).Weight(font.SemiBold), "", nil)
	}
}

// addRuleView is the add form: the rule typed, then a row per file it can
// be saved in, Enter or a click saving it there.
func (t *chatTools) addRuleView(gtx C, th *ui.Theme, l api.ChatPermissionList, running bool) D {
	title := i18n.T("chat.permissions.addTitle.allow")
	switch permTabBehaviors[t.set.tabs.Selected] {
	case api.PermissionAsk:
		title = i18n.T("chat.permissions.addTitle.ask")
	case api.PermissionDeny:
		title = i18n.T("chat.permissions.addTitle.deny")
	}
	head := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(t.detailHead(th, i18n.T("chat.permissions.back"), title)),
			layout.Rigid(func(gtx C) D {
				in := th.Input(&t.set.rule, i18n.T("chat.permissions.ruleHint"))
				in.Bare, in.Height = true, pickerRowH
				return in.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return toolsGroupLine(th, i18n.T("chat.permissions.saveIn"), "")(gtx)
			}),
		)
	}
	t.ensureRows(len(l.Destinations))
	lines := make([]layout.Widget, len(l.Destinations))
	for i, d := range l.Destinations {
		lines[i] = func(gtx C) D {
			if running {
				gtx = gtx.Disabled()
			}
			return toolsPick(gtx, th, &t.rows[i], t.cursor == i, func(gtx C, th *ui.Theme) D {
				who, file := permDestination(d)
				end := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, file).Mono().Muted().Layout)}
				return descRow(gtx, th, func(gtx C) D { return icFolder.Layout(gtx, toolsGlyph, th.MutedForeground) },
					th.Text(ui.TextSM, permSourceLabel(d)).Truncated().Layout, who, end)
			})
		}
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

// ruleDetail is a rule, its text the title: what it means, where it comes
// from and whether it applies, with Remove where it can be removed.
func (t *chatTools) ruleDetail(gtx C, th *ui.Theme, l api.ChatPermissionList, running bool) D {
	var r api.ChatPermissionRule
	for _, rule := range l.Rules {
		if ruleKey(rule) == t.detail {
			r = rule
		}
	}
	var lines []layout.Widget
	if r.Description != "" {
		lines = append(lines, mcpFact(th, i18n.T("chat.permissions.fact.means"), th.Text(ui.TextSM, r.Description).Wrapped().Layout))
	}
	lines = append(lines, mcpFact(th, i18n.T("chat.permissions.fact.source"), th.Text(ui.TextSM, permSourceLabel(r.Source)).Layout))
	var foot layout.Widget
	switch {
	case r.Ignored:
		foot = toolsFootNote(th, i18n.T("chat.permissions.ignored"))
	case r.Removable:
		remove := []chatPluginAction{{action: api.PermissionRemove, label: i18n.T("chat.permissions.remove")}}
		busy := ""
		if running {
			busy = remove[0].key()
		}
		foot = func(gtx C) D { return t.pluginButtons(gtx, th, remove, busy) }
	default:
		foot = toolsFootNote(th, i18n.T("chat.permissions.readOnly"))
	}
	// the rule stands whole under the head, wrapped: Remove names it as the
	// settings file holds it
	rule := func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx, th.Text(ui.TextSM, r.Rule).Weight(font.SemiBold).Wrapped().Layout)
	}
	return t.scrolled(gtx, th, t.detailHead(th, i18n.T("chat.permissions.back"), ""), append([]layout.Widget{rule}, lines...), foot)
}

// modesView is Codex's /permissions: its modes, the one in force checked
// and a dangerous one marked, as the mode chip's menu has them.
func (u *window) modesView(gtx C, th *ui.Theme, t *chatTools, sv sessionView) D {
	opt, ok := u.model.chat.optionOf(sv.ID, api.OptionMode)
	if !ok {
		return toolsNote(gtx, th, i18n.T("chat.permissions.none", i18n.Args{"provider": u.providerName(sv)}))
	}
	t.ensureRows(len(opt.Choices))
	lines := make([]layout.Widget, len(opt.Choices))
	for i, c := range opt.Choices {
		lines[i] = func(gtx C) D {
			return toolsPick(gtx, th, &t.rows[i], t.cursor == i, func(gtx C, th *ui.Theme) D {
				glyph := pickerCheck(th, c.ID == opt.Current)
				if c.Danger && c.ID != opt.Current {
					glyph = func(gtx C) D { return rainbowBolt(th)(gtx, toolsGlyph) }
				}
				return iconRow(gtx, th, glyph, th.Text(ui.TextSM, modeLabel(sv.Kind, c)).Layout)
			})
		}
	}
	head := func(gtx C) D { return toolsHead(gtx, th, i18n.T("chat.permissions.title"), "", nil) }
	var foot layout.Widget
	if u.chatBusy(sv.ID) {
		foot = toolsFootNote(th, i18n.T("chat.option.busy"))
	}
	return t.scrolled(gtx, th, head, lines, foot)
}

// footNotes is the muted notes under a list, nil when there are none.
func footNotes(th *ui.Theme, notes []string) layout.Widget {
	if len(notes) == 0 {
		return nil
	}
	return func(gtx C) D {
		children := make([]layout.FlexChild, len(notes))
		for i, n := range notes {
			children[i] = layout.Rigid(toolsFootNote(th, n))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
