package chat

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// A Codex chat's plugins, as measured on 0.170.0: plugin/installed lists the
// installed ones, plugin/list with the local marketplaces those on offer (the
// remote catalog it adds otherwise ran past 14,000 plugins and 43 MB), and
// plugin/read what one brings. Codex installs for the person only. Turning
// one off has no method: it is config/value/write of plugins."<id>".enabled,
// the key Codex itself writes, after which skills/list drops its skills.
// There is no plugin update: an install again copies the marketplace's
// version, but nothing says a newer one exists, so lich offers none.

var codexPluginActions = []string{
	api.PluginInstall, api.PluginUninstall, api.PluginEnable, api.PluginDisable,
	api.MarketplaceAdd, api.MarketplaceUpdate, api.MarketplaceRemove,
}

type codexPlugin struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Installed    bool   `json:"installed"`
	Enabled      bool   `json:"enabled"`
	Version      string `json:"version"`
	LocalVersion string `json:"localVersion"`
	Interface    *struct {
		Short string `json:"shortDescription"`
		Long  string `json:"longDescription"`
	} `json:"interface"`
}

type codexMarketplace struct {
	Name    string        `json:"name"`
	Path    string        `json:"path"`
	Plugins []codexPlugin `json:"plugins"`
}

func (d *codexDriver) plugins(ctx context.Context) (api.ChatPluginList, error) {
	installed, err := d.pluginMarketplaces(ctx, "plugin/installed", map[string]any{"cwds": []string{d.c.cwd}})
	if err != nil {
		return api.ChatPluginList{}, err
	}
	offered, err := d.offeredPlugins(ctx)
	if err != nil {
		return api.ChatPluginList{}, err
	}
	list := api.ChatPluginList{Reported: true, Actions: codexPluginActions, Scopes: []string{api.PluginScopeUser}}
	for _, m := range installed {
		for _, p := range m.Plugins {
			out := codexChatPlugin(m.Name, p)
			out.Parts, out.Description = d.pluginParts(ctx, m, p, out.Description)
			list.Plugins = append(list.Plugins, out)
		}
	}
	for _, m := range offered {
		for _, p := range m.Plugins {
			if !p.Installed {
				list.Plugins = append(list.Plugins, codexChatPlugin(m.Name, p))
			}
		}
	}
	counts := pluginMarketplaceCounts(list.Plugins)
	for name, source := range d.marketplaceSources(ctx) {
		list.Marketplaces = append(list.Marketplaces, api.ChatMarketplace{Name: name, Source: source, Plugins: counts[name], Fixed: name == lichMarketplace})
	}
	slices.SortFunc(list.Marketplaces, func(a, b api.ChatMarketplace) int { return strings.Compare(a.Name, b.Name) })
	return list, nil
}

func (d *codexDriver) offeredPlugins(ctx context.Context) ([]codexMarketplace, error) {
	return d.pluginMarketplaces(ctx, "plugin/list", map[string]any{"cwds": []string{d.c.cwd}, "marketplaceKinds": []string{"local"}})
}

func (d *codexDriver) pluginMarketplaces(ctx context.Context, method string, params map[string]any) ([]codexMarketplace, error) {
	raw, err := d.call(ctx, method, params)
	if err != nil {
		return nil, err
	}
	var r struct {
		Marketplaces []codexMarketplace `json:"marketplaces"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return r.Marketplaces, nil
}

func codexChatPlugin(market string, p codexPlugin) api.ChatPlugin {
	out := api.ChatPlugin{
		ID: p.ID, Name: p.Name, Marketplace: market, Version: cmp.Or(p.LocalVersion, p.Version),
		Installed: p.Installed, Enabled: p.Enabled, Fixed: fixedPlugin(p.ID),
	}
	if p.Installed {
		out.Scope = api.PluginScopeUser
	}
	if p.Interface != nil {
		out.Description = cmp.Or(p.Interface.Short, p.Interface.Long)
	}
	return out
}

// pluginParts is what plugin/read says p brings, and its description when
// the list gave none; nil and desc when it cannot be read.
func (d *codexDriver) pluginParts(ctx context.Context, m codexMarketplace, p codexPlugin, desc string) (*api.ChatPluginParts, string) {
	params := map[string]any{"pluginName": p.Name, "remoteMarketplaceName": m.Name}
	if m.Path != "" {
		params = map[string]any{"pluginName": p.Name, "marketplacePath": m.Path}
	}
	raw, err := d.call(ctx, "plugin/read", params)
	if err != nil {
		return nil, desc
	}
	var r struct {
		Plugin struct {
			Description string            `json:"description"`
			Skills      []json.RawMessage `json:"skills"`
			Hooks       []json.RawMessage `json:"hooks"`
			MCP         []json.RawMessage `json:"mcpServers"`
		} `json:"plugin"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return nil, desc
	}
	return &api.ChatPluginParts{Skills: len(r.Plugin.Skills), Hooks: len(r.Plugin.Hooks), MCP: len(r.Plugin.MCP)}, cmp.Or(desc, r.Plugin.Description)
}

// marketplaceSources is the marketplaces config.toml declares, by name, with
// the source each was added from; none when config/read fails.
func (d *codexDriver) marketplaceSources(ctx context.Context) map[string]string {
	raw, err := d.call(ctx, "config/read", map[string]any{"cwd": d.c.cwd})
	if err != nil {
		return nil
	}
	var r struct {
		Config struct {
			Marketplaces map[string]struct {
				Source string `json:"source"`
			} `json:"marketplaces"`
		} `json:"config"`
	}
	_ = json.Unmarshal(raw, &r)
	out := map[string]string{}
	for name, m := range r.Config.Marketplaces {
		out[name] = m.Source
	}
	return out
}

func (d *codexDriver) pluginAct(ctx context.Context, opts api.ChatPluginActOptions) (api.ChatPluginActResult, error) {
	var err error
	switch opts.Action {
	case api.PluginInstall:
		err = d.installPlugin(ctx, opts.Target)
	case api.PluginUninstall:
		_, err = d.call(ctx, "plugin/uninstall", map[string]any{"pluginId": opts.Target})
	case api.PluginEnable, api.PluginDisable:
		_, err = d.call(ctx, "config/value/write", map[string]any{
			"keyPath": `plugins."` + opts.Target + `".enabled`, "value": opts.Action == api.PluginEnable, "mergeStrategy": "upsert",
		})
	case api.MarketplaceAdd:
		_, err = d.call(ctx, "marketplace/add", map[string]any{"source": opts.Target})
	case api.MarketplaceRemove:
		_, err = d.call(ctx, "marketplace/remove", map[string]any{"marketplaceName": opts.Target})
	case api.MarketplaceUpdate:
		err = d.upgradeMarketplace(ctx, opts.Target)
	default:
		return api.ChatPluginActResult{}, fmt.Errorf("Codex cannot %s a plugin", opts.Action)
	}
	if err != nil {
		return api.ChatPluginActResult{}, err
	}
	d.listSkills(ctx)
	return api.ChatPluginActResult{}, nil
}

// installPlugin installs id from the local marketplace that offers it,
// plugin/install taking the plugin's name and its marketplace's file.
func (d *codexDriver) installPlugin(ctx context.Context, id string) error {
	offered, err := d.offeredPlugins(ctx)
	if err != nil {
		return err
	}
	for _, m := range offered {
		for _, p := range m.Plugins {
			if p.ID == id {
				_, err := d.call(ctx, "plugin/install", map[string]any{"pluginName": p.Name, "marketplacePath": m.Path})
				return err
			}
		}
	}
	return fmt.Errorf("no marketplace of this chat offers %s", id)
}

func (d *codexDriver) upgradeMarketplace(ctx context.Context, name string) error {
	raw, err := d.call(ctx, "marketplace/upgrade", map[string]any{"marketplaceName": name})
	if err != nil {
		return err
	}
	var r struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(raw, &r)
	if len(r.Errors) > 0 {
		return errors.New(r.Errors[0].Message)
	}
	return nil
}
