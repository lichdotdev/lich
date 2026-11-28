package terminal

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/library"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/subagent"
)

// leanCard leaves out everything a card can.
var leanCard = subagent.Card{Lean: true, Brought: []string{}, Left: []api.SubagentCardItem{
	{ID: "mcp:ai-memory", Kind: api.SubagentBringMCP, Name: "ai-memory", Origin: "opencode.json"},
	{ID: "mcp:github", Kind: api.SubagentBringMCP, Name: "github", Origin: api.SubagentOriginLibrary},
	{ID: "plugin:ponytail@ponytail", Kind: api.SubagentBringPlugin, Name: "ponytail", Origin: "ponytail"},
}}

// However little a card brings, lich's own plugin and MCP server ride every
// provider's spawn that takes them, and nothing a card leaves out names them.
func TestLichAlwaysRidesALeanCard(t *testing.T) {
	t.Parallel()
	// The plugin's path in the OS's own spelling, as the spawn joins it.
	bin, plugin := "/usr/bin/lich", filepath.FromSlash("/p/0.23.0")
	lib := library.Delivery{MCP: map[string]library.Server{"github": {Command: "gh-mcp"}}}
	for _, kind := range []string{providers.Claude, providers.Codex, providers.OMP, providers.Cursor} {
		t.Run(kind, func(t *testing.T) {
			args := providerArgs(kind, "", "", "", "", bin, "", plugin, lib, leanCard, false, false, false, relay.RouteCards, prompt.English)
			joined := strings.Join(args, " ")
			if pluginArgs(kind, plugin) != nil && !strings.Contains(joined, plugin) {
				t.Errorf("lich's plugin is missing: %v", args)
			}
			if providers.AcceptsMCPServer(kind) && !strings.Contains(joined, relay.MCPServerName) {
				t.Errorf("lich's MCP server is missing: %v", args)
			}
			if strings.Contains(joined, "gh-mcp") {
				t.Errorf("the library server the card leaves out still rides: %v", args)
			}
			if doc := claudeSettingsArgs(kind, false, leanCard); doc != nil && strings.Contains(doc[1], `"lich`) {
				t.Errorf("the settings name lich's own: %s", doc[1])
			}
		})
	}
	env := mcpEnv(nil, providers.OpenCode, bin)
	env = pluginEnv(env, providers.OpenCode, plugin)
	env = cardEnv(env, providers.OpenCode, leanCard)
	var config struct {
		MCP    map[string]map[string]any `json:"mcp"`
		Plugin []string                  `json:"plugin"`
	}
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, opencodeConfigContentVar+"="); ok {
			if err := json.Unmarshal([]byte(v), &config); err != nil {
				t.Fatal(err)
			}
		}
	}
	if config.MCP[relay.MCPServerName]["command"] == nil || len(config.Plugin) != 1 {
		t.Errorf("opencode lost lich's server or plugin: %+v", config)
	}
	if config.MCP["ai-memory"]["enabled"] != false {
		t.Errorf("opencode's own server stays on: %+v", config.MCP)
	}
	if _, ok := config.MCP["github"]; ok {
		t.Errorf("a library server got an entry opencode cannot start: %+v", config.MCP)
	}
}

// One --settings document carries ultracode and what a card leaves out.
func TestUltracodeAndALeanCardShareOneSettings(t *testing.T) {
	t.Parallel()
	args := claudeSettingsArgs(providers.Claude, true, leanCard)
	if len(args) != 2 {
		t.Fatalf("args = %v", args)
	}
	doc := map[string]any{}
	if err := json.Unmarshal([]byte(args[1]), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["ultracode"] != true || doc["enabledPlugins"] == nil || doc["deniedMcpServers"] == nil {
		t.Errorf("settings = %s", args[1])
	}
}

func TestCardLibraryLeavesTheDeliveryAlone(t *testing.T) {
	t.Parallel()
	lib := library.Delivery{MCP: map[string]library.Server{"github": {}, "kept": {}}}
	got := cardLibrary(lib, leanCard)
	if _, ok := got.MCP["github"]; ok || len(got.MCP) != 1 {
		t.Errorf("card delivery = %v", got.MCP)
	}
	if len(lib.MCP) != 2 {
		t.Error("the library's own delivery was changed")
	}
	if !slices.Equal(cardArgs(providers.OMP, subagent.Card{}), nil) {
		t.Error("a session that is no card got oh-my-pi's lean flags")
	}
}
