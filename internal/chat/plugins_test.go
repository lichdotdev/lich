//go:build !windows

package chat

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func (r *rig) plugins() api.ChatPluginList {
	r.t.Helper()
	list, err := r.svc.Plugins(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("Plugins: %v", err)
	}
	return list
}

func (r *rig) pluginAct(action, target, scope string) (api.ChatPluginActResult, error) {
	return r.svc.PluginAct(r.t.Context(), api.ChatPluginActOptions{ID: "s1", Action: action, Target: target, Scope: scope})
}

func pluginNamed(t *testing.T, list api.ChatPluginList, id string) api.ChatPlugin {
	t.Helper()
	i := slices.IndexFunc(list.Plugins, func(p api.ChatPlugin) bool { return p.ID == id })
	if i < 0 {
		t.Fatalf("no plugin %q in %+v", id, list.Plugins)
	}
	return list.Plugins[i]
}

// Claude Code's list is the CLI's, run in the checkout outside a child
// session: the installed plugins that apply there, the description and what
// each brings from `plugin details`, a newer version where the folder it is read from has one,
// then those on offer; lich's own plugin and marketplace fixed.
func TestClaudePluginList(t *testing.T) {
	t.Setenv(claudeChildEnv, "1")
	r := newChat(t, providers.Claude)
	list := r.plugins()
	if !list.Reported || !slices.Equal(list.Scopes, claudePluginScopes) || !slices.Equal(list.Actions, claudePluginActions) {
		t.Fatalf("list = %+v", list)
	}
	proj := filepath.Join(r.home, "proj")
	if !r.logged("plugin-cli list --json --available in "+proj, "child=false") {
		t.Errorf("the list did not run in the checkout without the child variable: %s", r.fakeLog())
	}
	want := map[string]api.ChatPlugin{
		"ponytail@ponytail": {ID: "ponytail@ponytail", Name: "ponytail", Marketplace: "ponytail", Description: "Forces the laziest solution that works.",
			Version: "5.3.2", Installed: true, Enabled: true, Scope: "user", Parts: &api.ChatPluginParts{Skills: 6, Hooks: 3, MCP: 1, Tokens: 1200}},
		"hello@local-mk": {ID: "hello@local-mk", Name: "hello", Marketplace: "local-mk", Version: "1.0.0", Latest: "1.1.0", Installed: true, Scope: "project"},
		"lich@lich-plugin": {ID: "lich@lich-plugin", Name: "lich", Marketplace: "lich-plugin", Version: "0.21.1", Installed: true, Enabled: true,
			Scope: "user", Fixed: true},
		"frontend-design@anthropic-plugin-directory": {ID: "frontend-design@anthropic-plugin-directory", Name: "frontend-design",
			Marketplace: "anthropic-plugin-directory", Description: "Product-grade interfaces", Version: "1.1.1", Installs: 48000},
	}
	for id, w := range want {
		if got := pluginNamed(t, list, id); !jsonEqual(got, w) {
			t.Errorf("%s = %+v, want %+v", id, got, w)
		}
	}
	if slices.ContainsFunc(list.Plugins, func(p api.ChatPlugin) bool { return p.ID == "elsewhere@local-mk" }) {
		t.Error("a plugin installed in another checkout is listed")
	}
	wantMarkets := []api.ChatMarketplace{
		{Name: "ponytail", Source: "DietrichGebert/ponytail", Plugins: 1},
		{Name: "lich-plugin", Source: "https://github.com/lichdotdev/lich-plugin.git", Plugins: 1, Fixed: true},
		{Name: "local-mk", Source: "/src/mk", Plugins: 2},
	}
	if !slices.Equal(list.Marketplaces, wantMarkets) {
		t.Errorf("marketplaces = %+v", list.Marketplaces)
	}
	r.plugins()
	if n := strings.Count(r.fakeLog(), "plugin-cli details ponytail@ponytail"); n != 1 {
		t.Errorf("details of one install ran %d times", n)
	}
}

func TestParseClaudeParts(t *testing.T) {
	if got := parseClaudeParts("hello 1\n  Skills (1)  greet\n  Agents (2)\nAlways-on:   ~8 tok"); !jsonEqual(got, &api.ChatPluginParts{Skills: 1, Agents: 2, Tokens: 8}) {
		t.Errorf("parts = %+v", got)
	}
	if got := parseClaudeParts("Always-on: < 20 tok"); got != nil {
		t.Errorf("no inventory = %+v, want nil", got)
	}
}

// Each change is its CLI line with its scope, never -y; a refusal is its
// message, a change already made is done, and the running chat reloads its
// plugins, whose answer is the slash list.
func TestClaudePluginAct(t *testing.T) {
	r := newChat(t, providers.Claude)
	r.waitCommands([]api.ChatCommand{
		{Text: "/review", Description: "Review a pull request", Hint: "[pr]"},
		{Text: "/deploy", Description: "Ship it"},
		{Text: "/doctor", Description: "Check the install"},
	})
	if _, err := r.pluginAct(api.PluginInstall, "frontend-design@anthropic-plugin-directory", api.PluginScopeProject); err != nil ||
		!r.logged("plugin-cli install frontend-design@anthropic-plugin-directory --json --scope project in ") || !r.logged(`"reload_plugins"`) {
		t.Fatalf("install: %v, log %s", err, r.fakeLog())
	}
	r.waitCommands([]api.ChatCommand{{Text: "/deploy", Description: "Ship it"}, {Text: "/doctor", Description: "Check the install"}})
	if strings.Contains(r.fakeLog(), " -y") {
		t.Error("a change ran with -y")
	}
	if _, err := r.pluginAct(api.PluginUninstall, "nope@x", api.PluginScopeUser); err == nil || !strings.Contains(err.Error(), `Plugin "nope" not found`) {
		t.Errorf("uninstall nope: %v", err)
	}
	if _, err := r.pluginAct(api.PluginDisable, "done@x", api.PluginScopeUser); err != nil {
		t.Errorf("a plugin already off: %v", err)
	}
	if got, err := r.pluginAct(api.PluginUpdate, "ponytail@ponytail", api.PluginScopeUser); err != nil || !got.Restart || got.Current {
		t.Errorf("update = %+v, %v", got, err)
	}
	if got, err := r.pluginAct(api.PluginUpdate, "hello@local-mk", api.PluginScopeProject); err != nil || !got.Current {
		t.Errorf("update up to date = %+v, %v", got, err)
	}
	if _, err := r.pluginAct(api.MarketplaceAdd, "owner/repo", api.PluginScopeUser); err != nil || !r.logged("plugin-cli marketplace add owner/repo --json in ") {
		t.Errorf("marketplace add: %v, log %s", err, r.fakeLog())
	}
}

// What no agent takes is refused before it reaches one: lich's own plugin and
// marketplace, a target that reads as a flag, an unknown action or scope.
func TestPluginActRefuses(t *testing.T) {
	r := newChat(t, providers.Claude)
	for _, c := range []struct{ action, target, scope string }{
		{api.PluginDisable, "lich@lich-plugin", ""},
		{api.PluginUninstall, "lich@lich-plugin", "user"},
		{api.PluginUpdate, "lich@lich-plugin", ""},
		{api.MarketplaceRemove, "lich-plugin", ""},
		{api.PluginInstall, "--plugin-dir=/x", ""},
		{api.PluginInstall, "", ""},
		{"explode", "x@y", ""},
		{api.PluginInstall, "x@y", "managed"},
	} {
		if _, err := r.pluginAct(c.action, c.target, c.scope); err == nil {
			t.Errorf("%s %q %q was taken", c.action, c.target, c.scope)
		}
	}
	if strings.Contains(r.fakeLog(), "plugin-cli") {
		t.Errorf("a refused action reached the CLI: %s", r.fakeLog())
	}
	if _, err := r.pluginAct(api.PluginInstall, "lich@lich-plugin", "user"); err != nil {
		t.Errorf("installing lich's own plugin: %v", err)
	}
}

func TestACPReportsNoPlugins(t *testing.T) {
	r := newChat(t, providers.OpenCode)
	if got := r.plugins(); got.Reported {
		t.Errorf("Plugins = %+v", got)
	}
	if _, err := r.pluginAct(api.PluginInstall, "x@y", ""); !errors.Is(err, errNoPlugins) {
		t.Errorf("PluginAct: %v", err)
	}
}

// Codex's list is plugin/installed with what plugin/read says each brings,
// then the local marketplaces' plugins not installed, never the remote
// catalog; its marketplaces are config.toml's.
func TestCodexPluginList(t *testing.T) {
	r := newChat(t, providers.Codex)
	list := r.plugins()
	if !list.Reported || !slices.Equal(list.Scopes, []string{api.PluginScopeUser}) || slices.Contains(list.Actions, api.PluginUpdate) {
		t.Fatalf("list = %+v", list)
	}
	want := map[string]api.ChatPlugin{
		"ponytail@ponytail": {ID: "ponytail@ponytail", Name: "ponytail", Marketplace: "ponytail", Description: "Lazy senior developer mode", Version: "5.3.3",
			Installed: true, Enabled: true, Scope: "user", Parts: &api.ChatPluginParts{Skills: 2, Hooks: 1}},
		"visualize@openai-curated-remote": {ID: "visualize@openai-curated-remote", Name: "visualize", Marketplace: "openai-curated-remote", Version: "1.0.89",
			Installed: true, Scope: "user"},
		"gmail@openai-curated": {ID: "gmail@openai-curated", Name: "gmail", Marketplace: "openai-curated", Description: "Read and manage Gmail", Version: "0.1.0"},
	}
	for id, w := range want {
		if got := pluginNamed(t, list, id); !jsonEqual(got, w) {
			t.Errorf("%s = %+v, want %+v", id, got, w)
		}
	}
	if len(list.Plugins) != len(want) {
		t.Errorf("plugins = %+v", list.Plugins)
	}
	if !r.logged(`"plugin/read"`, `"remoteMarketplaceName":"openai-curated-remote"`) {
		t.Errorf("a remote plugin was not read by its marketplace's name: %s", r.fakeLog())
	}
	wantMarkets := []api.ChatMarketplace{
		{Name: "lich-plugin", Source: "https://github.com/lichdotdev/lich-plugin.git", Fixed: true},
		{Name: "ponytail", Source: "https://github.com/DietrichGebert/ponytail.git", Plugins: 1},
	}
	if !slices.Equal(list.Marketplaces, wantMarkets) {
		t.Errorf("marketplaces = %+v", list.Marketplaces)
	}
}

// Codex installs from the marketplace's file, turns a plugin off through its
// enabled key, takes no update, and reads its skills again after a change.
func TestCodexPluginAct(t *testing.T) {
	r := newChat(t, providers.Codex)
	r.waitCommands([]api.ChatCommand{{Text: "$deploy", Description: "Ship it", Skill: true}})
	lists := strings.Count(r.fakeLog(), `"skills/list"`)
	if _, err := r.pluginAct(api.PluginInstall, "gmail@openai-curated", api.PluginScopeUser); err != nil ||
		!r.logged(`"plugin/install"`, `"pluginName":"gmail"`, `"marketplacePath":"/m/curated/marketplace.json"`) {
		t.Fatalf("install: %v, log %s", err, r.fakeLog())
	}
	if _, err := r.pluginAct(api.PluginDisable, "ponytail@ponytail", ""); err != nil ||
		!r.logged(`"config/value/write"`, `"keyPath":"plugins.\"ponytail@ponytail\".enabled"`, `"value":false`) {
		t.Errorf("disable: %v, log %s", err, r.fakeLog())
	}
	if n := strings.Count(r.fakeLog(), `"skills/list"`); n != lists+2 {
		t.Errorf("skills read %d times after two changes, want %d", n-lists, 2)
	}
	if _, err := r.pluginAct(api.PluginUninstall, "ponytail@ponytail", ""); err != nil || !r.logged(`"plugin/uninstall"`, `"pluginId":"ponytail@ponytail"`) {
		t.Errorf("uninstall: %v", err)
	}
	if _, err := r.pluginAct(api.PluginInstall, "nope@x", ""); err == nil {
		t.Error("a plugin no marketplace offers was installed")
	}
	if _, err := r.pluginAct(api.PluginUpdate, "ponytail@ponytail", ""); err == nil {
		t.Error("Codex took a plugin update")
	}
	if _, err := r.pluginAct(api.MarketplaceUpdate, "local-mk", ""); err == nil || !strings.Contains(err.Error(), "not configured as a Git marketplace") {
		t.Errorf("upgrade local-mk: %v", err)
	}
	if _, err := r.pluginAct(api.MarketplaceRemove, "ponytail", ""); err != nil || !r.logged(`"marketplace/remove"`, `"marketplaceName":"ponytail"`) {
		t.Errorf("remove: %v", err)
	}
}
