package main

import (
	"cmp"
	"slices"
	"strings"

	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The /plugin panel's tabs, in the order they are drawn.
const (
	pluginTabDiscover = iota
	pluginTabInstalled
	pluginTabMarkets
)

// chatPlugins is the /plugin panel's own state; chatTools holds what it shares
// with the other panels (the list read, the detail open, the rows).
type chatPlugins struct {
	tabs   ui.TabsState
	source widget.Editor
	add    ui.Clickable
	// note is what the last action left to say and running the key of the
	// action still going, "" for none, both guarded by chatTools.mu.
	note    string
	running string
	// found is the plugins on offer the search holds, kept for query and
	// the read gen it was cut from.
	found      []api.ChatPlugin
	foundQuery string
	foundGen   int
}

func (c *chatPlugins) reset() {
	c.tabs.Selected, c.note, c.found, c.foundGen = pluginTabInstalled, "", nil, -1
	c.source.SingleLine = true
	c.source.SetText("")
}

// chatPluginKey names an installed plugin by its scope too: one can be installed
// for the person and in the project at once.
func chatPluginKey(p api.ChatPlugin) string { return p.ID + "|" + p.Scope }

// installedPlugins is the installed plugins, by marketplace and then by name.
func installedPlugins(all []api.ChatPlugin) []api.ChatPlugin {
	out := slices.DeleteFunc(slices.Clone(all), func(p api.ChatPlugin) bool { return !p.Installed })
	slices.SortStableFunc(out, func(a, b api.ChatPlugin) int {
		return cmp.Or(strings.Compare(a.Marketplace, b.Marketplace), strings.Compare(a.Name, b.Name))
	})
	return out
}

// discovered is the plugins on offer the search holds, those it names before
// those it only describes, then the most installed first, cut once per query
// and read.
func (t *chatTools) discovered(all []api.ChatPlugin, gen int) []api.ChatPlugin {
	q := strings.ToLower(strings.TrimSpace(t.query.Text()))
	c := &t.plug
	if c.foundGen == gen && c.foundQuery == q {
		return c.found
	}
	c.found = slices.DeleteFunc(slices.Clone(all), func(p api.ChatPlugin) bool {
		return p.Installed || q != "" && !strings.Contains(strings.ToLower(p.Name), q) && !strings.Contains(strings.ToLower(p.Description), q)
	})
	named := func(p api.ChatPlugin) int {
		if q != "" && !strings.Contains(strings.ToLower(p.Name), q) {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(c.found, func(a, b api.ChatPlugin) int {
		return cmp.Or(named(a)-named(b), b.Installs-a.Installs, strings.Compare(a.Name, b.Name))
	})
	c.foundQuery, c.foundGen = q, gen
	return c.found
}

// chatPluginShown is the plugin the detail shows: by its key, or by its ID once
// an install or uninstall moved it between the tabs.
func chatPluginShown(all []api.ChatPlugin, key string) (api.ChatPlugin, bool) {
	i := slices.IndexFunc(all, func(p api.ChatPlugin) bool { return chatPluginKey(p) == key || p.ID == key })
	if i < 0 {
		id, _, _ := strings.Cut(key, "|")
		i = slices.IndexFunc(all, func(p api.ChatPlugin) bool { return p.ID == id })
	}
	if i < 0 {
		return api.ChatPlugin{}, false
	}
	return all[i], true
}

// chatPluginAction is one button of a detail: an action on its plugin or
// marketplace, in a scope.
type chatPluginAction struct {
	action, scope, label string
}

func (a chatPluginAction) key() string { return a.action + "|" + a.scope }

// removes is whether a takes the plugin or the marketplace away.
func (a chatPluginAction) removes() bool {
	return a.action == api.PluginUninstall || a.action == api.MarketplaceRemove || a.action == api.PermissionRemove
}

// chatPluginActions is what the detail offers on p, of what its provider takes:
// an update, turning it off or on and uninstalling it once installed,
// installing it in each scope before; nothing on lich's own.
func chatPluginActions(p api.ChatPlugin, l api.ChatPluginList) []chatPluginAction {
	var acts []chatPluginAction
	add := func(action, scope, label string) {
		if slices.Contains(l.Actions, action) {
			acts = append(acts, chatPluginAction{action, scope, label})
		}
	}
	switch {
	case p.Fixed:
	case p.Installed:
		add(api.PluginUpdate, p.Scope, i18n.T("chat.plugins.act.update"))
		if p.Enabled {
			add(api.PluginDisable, p.Scope, i18n.T("chat.plugins.act.disable"))
		} else {
			add(api.PluginEnable, p.Scope, i18n.T("chat.plugins.act.enable"))
		}
		add(api.PluginUninstall, p.Scope, i18n.T("chat.plugins.act.uninstall"))
	default:
		for _, scope := range l.Scopes {
			add(api.PluginInstall, scope, pluginInstallLabel(scope))
		}
	}
	return acts
}

func pluginInstallLabel(scope string) string {
	switch scope {
	case api.PluginScopeProject:
		return i18n.T("chat.plugins.act.installProject")
	case api.PluginScopeLocal:
		return i18n.T("chat.plugins.act.installLocal")
	}
	return i18n.T("chat.plugins.act.installUser")
}

// marketActions is what a marketplace's detail offers: updating and
// removing it, but not lich's own.
func marketActions(m api.ChatMarketplace, l api.ChatPluginList) []chatPluginAction {
	if m.Fixed {
		return nil
	}
	var acts []chatPluginAction
	if slices.Contains(l.Actions, api.MarketplaceUpdate) {
		acts = append(acts, chatPluginAction{api.MarketplaceUpdate, "", i18n.T("chat.plugins.act.marketUpdate")})
	}
	if slices.Contains(l.Actions, api.MarketplaceRemove) {
		acts = append(acts, chatPluginAction{api.MarketplaceRemove, "", i18n.T("chat.plugins.act.marketRemove")})
	}
	return acts
}

// pluginActs runs the /plugin panel's tabs, detail and add button, and gives
// toolsActs the rows of the tab shown and what Enter does on one.
func (u *window) pluginActs(gtx C, p *chatPane, id string, l api.ChatPluginList) (int, func(int)) {
	t := &p.tools
	c := &t.plug
	open := func(key string) { t.detail, t.focus = key, true }
	switch {
	case t.detail != "":
		u.pluginDetailActs(gtx, p, id, l)
		return 0, func(int) {}
	case c.tabs.Selected == pluginTabDiscover:
		t.mu.Lock()
		gen := t.gen
		t.mu.Unlock()
		shown := t.discovered(l.Plugins, gen)
		return len(shown), func(i int) { open(shown[i].ID) }
	case c.tabs.Selected == pluginTabMarkets:
		if c.add.Clicked(gtx) {
			u.submitSource(p, id, &c.source)
		}
		return len(l.Marketplaces), func(i int) { open(l.Marketplaces[i].Name) }
	}
	shown := installedPlugins(l.Plugins)
	return len(shown), func(i int) { open(chatPluginKey(shown[i])) }
}

// pluginDetailActs runs a detail's Back and its actions.
func (u *window) pluginDetailActs(gtx C, p *chatPane, id string, l api.ChatPluginList) {
	t := &p.tools
	var target string
	var acts []chatPluginAction
	if t.plug.tabs.Selected == pluginTabMarkets {
		if i := slices.IndexFunc(l.Marketplaces, func(m api.ChatMarketplace) bool { return m.Name == t.detail }); i >= 0 {
			target, acts = l.Marketplaces[i].Name, marketActions(l.Marketplaces[i], l)
		}
	} else if pl, ok := chatPluginShown(l.Plugins, t.detail); ok {
		target, acts = pl.ID, chatPluginActions(pl, l)
	}
	if t.back.Clicked(gtx) || target == "" {
		t.detail, t.focus = "", true
		return
	}
	for _, a := range acts {
		if t.act(a.key()).Clicked(gtx) {
			u.pluginAct(p, id, a.action, target, a.scope)
		}
	}
}

// submitSource adds the marketplace typed in the source field, when tag is
// that field and it holds one, and reports whether it did.
func (u *window) submitSource(p *chatPane, id string, tag *widget.Editor) bool {
	c := &p.tools.plug
	source := strings.TrimSpace(c.source.Text())
	if tag != &c.source || source == "" {
		return false
	}
	c.source.SetText("")
	u.pluginAct(p, id, api.MarketplaceAdd, source, "")
	return true
}

// pluginAct runs action on target, keeps what an update leaves to say and
// reads the plugins again. One runs at a time: the buttons wait on it.
func (u *window) pluginAct(p *chatPane, id, action, target, scope string) {
	t := &p.tools
	t.mu.Lock()
	if t.plug.running != "" {
		t.mu.Unlock()
		return
	}
	t.plug.running, t.plug.note = chatPluginAction{action: action, scope: scope}.key(), ""
	t.mu.Unlock()
	go func() {
		res, err := u.client.ChatPluginAct(u.ctx, id, action, target, scope)
		note := ""
		switch {
		case res.Restart:
			note = i18n.T("chat.plugins.restart")
		case res.Current:
			note = i18n.T("chat.plugins.current")
		}
		t.mu.Lock()
		t.plug.running, t.plug.note = "", note
		t.mu.Unlock()
		u.chatFailed(err)
		u.fetchTools(p, id)
	}()
}
