package main

import (
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// pluginDetailView is the plugin or marketplace the detail shows, back to
// the list when it is gone.
func (u *window) pluginDetailView(gtx C, th *ui.Theme, t *chatTools, l api.ChatPluginList) D {
	if t.plug.tabs.Selected == pluginTabMarkets {
		if i := slices.IndexFunc(l.Marketplaces, func(m api.ChatMarketplace) bool { return m.Name == t.detail }); i >= 0 {
			m := l.Marketplaces[i]
			lines := []layout.Widget{
				mcpFact(th, i18n.T("chat.plugins.fact.source"), th.Text(ui.TextXS, m.Source).Mono().Truncated().Layout),
				mcpFact(th, i18n.T("chat.plugins.fact.plugins"), th.Text(ui.TextSM, i18n.T("chat.plugins.pluginCount", i18n.Args{"count": m.Plugins})).Layout),
			}
			return t.pluginDetail(gtx, th, i18n.T("chat.plugins.backMarkets"), m.Name, "", lines, m.Fixed)
		}
	} else if pl, ok := chatPluginShown(l.Plugins, t.detail); ok {
		return t.pluginDetail(gtx, th, i18n.T("chat.plugins.back"), pl.Name, pl.Marketplace, pluginFacts(th, pl), pl.Fixed)
	}
	return D{}
}

// pluginDetail is a detail's head and its lines, with what the last action
// left to say, or why lich's own takes none; its actions stand on the keys'
// line (pluginFoot).
func (t *chatTools) pluginDetail(gtx C, th *ui.Theme, back, name, sub string, lines []layout.Widget, fixed bool) D {
	head := func(gtx C) D {
		return toolsBackHead(gtx, th, &t.back, back, th.Text(ui.TextSM, name).Weight(font.SemiBold), sub, nil)
	}
	t.mu.Lock()
	note := t.plug.note
	t.mu.Unlock()
	if fixed {
		note = i18n.T("chat.plugins.fixed")
	}
	var foot layout.Widget
	if note != "" {
		foot = toolsFootNote(th, note)
	}
	return t.scrolled(gtx, th, head, lines, foot)
}

// pluginShownActs is the actions of the plugin or marketplace the detail
// shows, false when it shows none.
func (t *chatTools) pluginShownActs(l api.ChatPluginList) ([]chatPluginAction, *api.ChatPlugin, bool) {
	if t.detail == "" {
		return nil, nil, false
	}
	if t.plug.tabs.Selected == pluginTabMarkets {
		i := slices.IndexFunc(l.Marketplaces, func(m api.ChatMarketplace) bool { return m.Name == t.detail })
		if i < 0 {
			return nil, nil, false
		}
		return marketActions(l.Marketplaces[i], l), nil, true
	}
	pl, ok := chatPluginShown(l.Plugins, t.detail)
	if !ok {
		return nil, nil, false
	}
	return chatPluginActions(pl, l), &pl, true
}

// pluginFoot is the detail's actions for the keys' line: what removes it
// first, in Destructive, then the rest, an update there is filled at
// the end.
func (t *chatTools) pluginFoot(th *ui.Theme, l api.ChatPluginList) layout.Widget {
	acts, pl, ok := t.pluginShownActs(l)
	if !ok || len(acts) == 0 {
		return nil
	}
	t.mu.Lock()
	running := t.plug.running
	t.mu.Unlock()
	button := func(a chatPluginAction, v ui.Variant) layout.FlexChild {
		b := th.Button(t.act(a.key()), a.label)
		b.Variant, b.Size = v, ui.SizeSM
		b.Busy = running == a.key()
		b.Disabled = running != "" && !b.Busy
		return layout.Rigid(b.Layout)
	}
	var first, rest, last []layout.FlexChild
	for _, a := range acts {
		switch {
		case a.removes():
			first = append(first, button(a, ui.VariantDestructive))
		case a.action == api.PluginUpdate && pl != nil && pl.Latest != "":
			last = append(last, button(a, ui.VariantDefault))
		default:
			rest = append(rest, button(a, ui.VariantSecondary))
		}
	}
	return footButtons(append(append(first, rest...), last...))
}

func (t *chatTools) pluginButtons(gtx C, th *ui.Theme, acts []chatPluginAction, running string) D {
	if len(acts) == 0 {
		return D{}
	}
	var row []layout.FlexChild
	for i, a := range acts {
		if i > 0 {
			row = append(row, ui.Gap(1.5))
		}
		b := th.Button(t.act(a.key()), a.label)
		b.Variant, b.Size = ui.VariantSecondary, ui.SizeSM
		b.Busy = running == a.key()
		b.Disabled = running != "" && !b.Busy
		row = append(row, layout.Rigid(b.Layout))
	}
	return layout.UniformInset(ui.Space(1.5)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	})
}

// pluginFacts is a plugin's description, then its version, state, scope,
// what it brings and costs, a label and its value a line.
func pluginFacts(th *ui.Theme, pl api.ChatPlugin) []layout.Widget {
	var lines []layout.Widget
	if pl.Description != "" {
		lines = append(lines, func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx, th.Text(ui.TextSM, pl.Description).Wrapped().Layout)
		})
	}
	text := func(label, value string) {
		if value != "" {
			lines = append(lines, mcpFact(th, label, th.Text(ui.TextSM, value).Truncated().Layout))
		}
	}
	if pl.Latest != "" {
		version := i18n.T("chat.plugins.versionNewer", i18n.Args{"version": pl.Version, "latest": pl.Latest})
		lines = append(lines, mcpFact(th, i18n.T("chat.plugins.fact.version"), th.Text(ui.TextSM, version).In(th.ToneWait).Truncated().Layout))
	} else {
		text(i18n.T("chat.plugins.fact.version"), pl.Version)
	}
	if !pl.Installed {
		if pl.Installs > 0 {
			text(i18n.T("chat.plugins.fact.installs"), pluginInstalls(pl.Installs))
		}
		return lines
	}
	state := i18n.T("chat.plugins.state.off")
	if pl.Enabled {
		state = i18n.T("chat.plugins.state.on")
	}
	lines = append(lines, mcpFact(th, i18n.T("chat.plugins.fact.state"), func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(pluginGlyph(th, pl)), ui.Gap(1.5), layout.Rigid(th.Text(ui.TextSM, state).Layout))
	}))
	text(i18n.T("chat.plugins.fact.scope"), pluginScopeLabel(pl.Scope))
	if pl.Parts != nil {
		text(i18n.T("chat.plugins.fact.brings"), pluginBrings(*pl.Parts))
		if pl.Parts.Tokens > 0 {
			text(i18n.T("chat.plugins.fact.cost"), i18n.T("chat.tools.skill.tokens", i18n.Args{"count": pl.Parts.Tokens}))
		}
	}
	return lines
}

// pluginBrings is what a plugin brings in words, the kinds it has none of
// left out.
func pluginBrings(p api.ChatPluginParts) string {
	var parts []string
	add := func(n int, text string) {
		if n > 0 {
			parts = append(parts, text)
		}
	}
	add(p.Skills, i18n.T("chat.tools.skillCount", i18n.Args{"count": p.Skills}))
	add(p.Agents, i18n.T("chat.plugins.part.agents", i18n.Args{"count": p.Agents}))
	add(p.Hooks, i18n.T("chat.plugins.part.hooks", i18n.Args{"count": p.Hooks}))
	add(p.MCP, i18n.T("chat.plugins.part.mcp", i18n.Args{"count": p.MCP}))
	return strings.Join(parts, " · ")
}
