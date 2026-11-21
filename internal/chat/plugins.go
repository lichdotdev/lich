package chat

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/agentplugin"
)

// pluginsDriver is a driver whose provider reports its plugins: Claude
// Code's and Codex's.
type pluginsDriver interface {
	plugins(ctx context.Context) (api.ChatPluginList, error)
	// pluginAct takes one of the actions plugins lists; the service has
	// checked it is one and does not touch lich's own plugin.
	pluginAct(ctx context.Context, opts api.ChatPluginActOptions) (api.ChatPluginActResult, error)
}

// pluginTimeout bounds a plugin action: an install or a marketplace add
// clones a repository, which Claude Code itself caps at 120s.
const pluginTimeout = 130 * time.Second

var (
	errNoPlugins    = errors.New("this agent does not report its plugins")
	errFixedPlugin  = errors.New("lich installs and pins its own plugin and marketplace itself")
	errPluginTarget = errors.New("a plugin or marketplace is named, never a flag")
)

// lichMarketplace is the marketplace lich's own plugin comes from.
var lichMarketplace = agentplugin.PluginKey[strings.Index(agentplugin.PluginKey, "@")+1:]

// fixedPlugin reports whether id is lich's own plugin.
func fixedPlugin(id string) bool { return id == agentplugin.PluginKey }

func (s *Service) pluginsOf(ctx context.Context, id string) (pluginsDriver, bool, error) {
	c, err := s.ensure(ctx, id)
	if err != nil {
		return nil, false, err
	}
	d, ok := c.drv.(pluginsDriver)
	return d, ok, nil
}

// Plugins answers api.ChatPlugins.
func (s *Service) Plugins(ctx context.Context, opts api.SessionIDOptions) (api.ChatPluginList, error) {
	d, ok, err := s.pluginsOf(ctx, opts.ID)
	if err != nil || !ok {
		return api.ChatPluginList{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return d.plugins(callCtx)
}

// PluginAct answers api.ChatPluginAct.
func (s *Service) PluginAct(ctx context.Context, opts api.ChatPluginActOptions) (api.ChatPluginActResult, error) {
	d, ok, err := s.pluginsOf(ctx, opts.ID)
	if err != nil {
		return api.ChatPluginActResult{}, err
	}
	if !ok {
		return api.ChatPluginActResult{}, errNoPlugins
	}
	if err := checkPluginAct(opts); err != nil {
		return api.ChatPluginActResult{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, pluginTimeout)
	defer cancel()
	return d.pluginAct(callCtx, opts)
}

// checkPluginAct refuses what no provider takes: an unknown action or scope,
// a target that would read as a flag on the CLI, and any change to lich's own
// plugin or marketplace but adding it.
func checkPluginAct(opts api.ChatPluginActOptions) error {
	if !slices.Contains(pluginActions, opts.Action) {
		return fmt.Errorf("no agent can %s a plugin", opts.Action)
	}
	if opts.Scope != "" && !slices.Contains(claudePluginScopes, opts.Scope) {
		return fmt.Errorf("a plugin is never installed in %q", opts.Scope)
	}
	if opts.Target == "" || strings.HasPrefix(opts.Target, "-") {
		return errPluginTarget
	}
	switch opts.Action {
	case api.MarketplaceUpdate, api.MarketplaceRemove:
		if opts.Target == lichMarketplace {
			return errFixedPlugin
		}
	case api.MarketplaceAdd, api.PluginInstall:
	default:
		if fixedPlugin(opts.Target) {
			return errFixedPlugin
		}
	}
	return nil
}

// pluginActions are every ChatPluginAct action.
var pluginActions = []string{
	api.PluginInstall, api.PluginUninstall, api.PluginEnable, api.PluginDisable, api.PluginUpdate,
	api.MarketplaceAdd, api.MarketplaceUpdate, api.MarketplaceRemove,
}

// pluginMarketplaceCounts is how many plugins each marketplace offers, one
// installed in two scopes counted once.
func pluginMarketplaceCounts(plugins []api.ChatPlugin) map[string]int {
	n, seen := map[string]int{}, map[string]bool{}
	for _, p := range plugins {
		if !seen[p.ID] {
			seen[p.ID] = true
			n[p.Marketplace]++
		}
	}
	return n
}
