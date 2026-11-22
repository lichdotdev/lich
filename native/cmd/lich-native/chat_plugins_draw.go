package main

import (
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icMarket = icons.Lucide("store")

// pluginsView is the /plugin panel: its title, the Discover, Installed and
// Marketplaces tabs and the tab shown, or the detail open.
func (u *window) pluginsView(gtx C, th *ui.Theme, p *chatPane, sv sessionView, l api.ChatPluginList) D {
	t := &p.tools
	if !l.Reported {
		return toolsNote(gtx, th, i18n.T("chat.plugins.none", i18n.Args{"provider": u.providerName(sv)}))
	}
	if t.detail != "" {
		return u.pluginDetailView(gtx, th, t, l)
	}
	t.mu.Lock()
	gen := t.gen
	t.mu.Unlock()
	var lines []layout.Widget
	var foot layout.Widget
	search := false
	switch t.plug.tabs.Selected {
	case pluginTabDiscover:
		search = true
		lines = t.pluginLines(th, t.discovered(l.Plugins, gen), false, i18n.T("chat.plugins.noneFound"))
	case pluginTabMarkets:
		lines, foot = t.marketLines(th, l.Marketplaces), t.addMarketLine(th)
	default:
		lines = t.pluginLines(th, installedPlugins(l.Plugins), true, i18n.T("chat.plugins.noneInstalled"))
	}
	head := func(gtx C) D { return t.pluginsHead(gtx, th, l, search) }
	return t.scrolled(gtx, th, head, lines, foot)
}

func (t *chatTools) pluginsHead(gtx C, th *ui.Theme, l api.ChatPluginList, search bool) D {
	installed := 0
	for _, pl := range l.Plugins {
		if pl.Installed {
			installed++
		}
	}
	tabs := th.Tabs(&t.plug.tabs,
		ui.TabsTrigger{Text: i18n.T("chat.plugins.tab.discover")},
		ui.TabsTrigger{Text: i18n.T("chat.plugins.tab.installed")},
		ui.TabsTrigger{Text: i18n.T("chat.plugins.tab.markets")},
	)
	tabs.Variant = ui.TabsVariantGhost
	rows := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return toolsHead(gtx, th, i18n.T("chat.plugins.title"), i18n.T("chat.plugins.installedCount", i18n.Args{"count": installed}), nil)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = 0
			d := layout.Inset{Left: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D { return tabs.Layout(gtx) })
			// The tabs read their clicks as they draw, after the rows of the
			// old one: the next frame draws the new one, from its top.
			if t.plug.tabs.Changed() {
				t.cursor, t.moved, t.focus = 0, true, true
				t.mu.Lock()
				t.plug.note = ""
				t.mu.Unlock()
				gtx.Execute(op.InvalidateCmd{})
			}
			return d
		}),
	}
	if search {
		rows = append(rows, layout.Rigid(func(gtx C) D {
			in := th.Input(&t.query, i18n.T("chat.plugins.search"))
			in.Bare, in.Height = true, pickerRowH
			return in.Layout(gtx)
		}))
	}
	rows = append(rows, layout.Rigid(th.Separator))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// pluginLines is the rows of plugins, each marketplace's under its name when
// grouped, or the empty note when there are none.
func (t *chatTools) pluginLines(th *ui.Theme, plugins []api.ChatPlugin, grouped bool, empty string) []layout.Widget {
	if len(plugins) == 0 {
		return []layout.Widget{func(gtx C) D { return toolsNote(gtx, th, empty) }}
	}
	t.ensureRows(len(plugins))
	var lines []layout.Widget
	market := ""
	for i, pl := range plugins {
		if grouped && pl.Marketplace != market {
			market = pl.Marketplace
			lines = append(lines, pluginGroupLine(th, market))
		}
		lines = append(lines, func(gtx C) D {
			return pickerRow(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return chatPluginRow(gtx, th, pl) })
		})
	}
	return lines
}

func pluginGroupLine(th *ui.Theme, name string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Top: ui.Space(2), Bottom: ui.Space(0.5)}.Layout(gtx,
			th.Text(ui.TextXS, name).Weight(font.SemiBold).Layout)
	}
}

// pluginGlyph is a plugin's mark: on, off, or not installed.
func pluginGlyph(th *ui.Theme, pl api.ChatPlugin) layout.Widget {
	ic, ink := icDownload, th.MutedForeground
	switch {
	case pl.Installed && pl.Enabled:
		ic, ink = icCheck, th.TonePass
	case pl.Installed:
		ic = icToolsOff
	}
	return func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }
}

// pluginMeta is what a plugin's row says at its end: where an installed one
// is and what it costs, how many installed one on offer.
func pluginMeta(pl api.ChatPlugin) string {
	if !pl.Installed {
		if pl.Installs == 0 {
			return ""
		}
		return pluginInstalls(pl.Installs)
	}
	parts := []string{pluginScopeLabel(pl.Scope)}
	if pl.Parts != nil && pl.Parts.Tokens > 0 {
		parts = append(parts, i18n.T("chat.tools.skill.tokens", i18n.Args{"count": pl.Parts.Tokens}))
	}
	return strings.Join(parts, " · ")
}

func pluginScopeLabel(scope string) string {
	switch scope {
	case api.PluginScopeProject:
		return i18n.T("chat.plugins.scope.project")
	case api.PluginScopeLocal:
		return i18n.T("chat.plugins.scope.local")
	}
	return i18n.T("chat.plugins.scope.user")
}

func chatPluginRow(gtx C, th *ui.Theme, pl api.ChatPlugin) D {
	name := th.Text(ui.TextSM, pl.Name)
	if pl.Installed && !pl.Enabled {
		name = name.Muted()
	}
	end := []layout.FlexChild{}
	if pl.Latest != "" {
		end = append(end, layout.Rigid(th.TinyChip(i18n.T("chat.plugins.updateChip"), th.ToneWait).Layout), ui.Gap(1.5))
	}
	end = append(end, layout.Rigid(th.Text(ui.TextXS, pluginMeta(pl)).Muted().Layout))
	return twoLineRow(gtx, th, pluginGlyph(th, pl), name.Layout, pl.Description, end)
}

// twoLineRow is a glyph, a name over its muted description, and the row's
// end.
func twoLineRow(gtx C, th *ui.Theme, glyph, name layout.Widget, desc string, end []layout.FlexChild) D {
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		row := []layout.FlexChild{
			layout.Rigid(glyph),
			ui.Gap(2),
			layout.Flexed(1, func(gtx C) D {
				// the row's height reaches here from pickerRow; a name with no
				// description under it would sit at the top instead of the middle
				gtx.Constraints.Min.Y = 0
				col := []layout.FlexChild{layout.Rigid(name)}
				if desc != "" {
					col = append(col, layout.Rigid(th.Text(ui.TextXS, cleanLine(desc)).Muted().Truncated().Layout))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, col...)
			}),
			ui.Gap(toolsMetaGap),
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, append(row, end...)...)
	})
}

func (t *chatTools) marketLines(th *ui.Theme, markets []api.ChatMarketplace) []layout.Widget {
	t.ensureRows(len(markets))
	lines := make([]layout.Widget, len(markets))
	for i, m := range markets {
		lines[i] = func(gtx C) D {
			return pickerRow(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D {
				glyph := func(gtx C) D { return icMarket.Layout(gtx, toolsGlyph, th.MutedForeground) }
				count := th.Text(ui.TextXS, i18n.T("chat.plugins.pluginCount", i18n.Args{"count": m.Plugins})).Muted().Layout
				return twoLineRow(gtx, th, glyph, th.Text(ui.TextSM, m.Name).Layout, m.Source, []layout.FlexChild{layout.Rigid(count)})
			})
		}
	}
	return lines
}

// addMarketLine is the Marketplaces tab's foot: the source to add and its
// button.
func (t *chatTools) addMarketLine(th *ui.Theme) layout.Widget {
	return func(gtx C) D {
		t.mu.Lock()
		running := t.plug.running
		t.mu.Unlock()
		return layout.UniformInset(ui.Space(1.5)).Layout(gtx, func(gtx C) D {
			in := th.Input(&t.plug.source, i18n.T("chat.plugins.sourceHint"))
			in.Height, in.TextSize = ui.Space(8), ui.TextXS
			b := th.Button(&t.plug.add, i18n.T("chat.plugins.act.marketAdd"))
			b.Variant, b.Size = ui.VariantSecondary, ui.SizeSM
			b.Busy = running == chatPluginAction{action: api.MarketplaceAdd}.key()
			b.Disabled = running != "" && !b.Busy
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, in.Layout), ui.Gap(1.5), layout.Rigid(b.Layout))
		})
	}
}

// pluginInstalls is a plugin's install count as the catalog reads it, short:
// "1.3M installs", the plural picked on the whole number.
func pluginInstalls(n int) string {
	return i18n.T("chat.plugins.installs", i18n.Args{"count": n, "n": compactCount(n, i18n.Current())})
}
