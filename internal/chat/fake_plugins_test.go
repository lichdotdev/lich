//go:build !windows

package chat

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// The fake's plugins, in the shapes measured on Claude Code 2.1.330 and
// Codex 0.170.0.

// claudePluginCLI is `claude plugin ...`: it logs what it was run with, where
// and whether under a child session, then answers as the real one does.
func (f *fake) claudePluginCLI(args []string) {
	cwd, _ := os.Getwd()
	if f.log != nil {
		fmt.Fprintf(f.log, "plugin-cli %s in %s child=%t\n", strings.Join(args[1:], " "), cwd, os.Getenv(claudeChildEnv) != "")
	}
	switch {
	case slices.Equal(args, []string{"plugin", "list", "--json", "--available"}):
		f.send(fakeClaudePluginList(cwd))
	case slices.Equal(args, []string{"plugin", "marketplace", "list", "--json"}):
		fmt.Println(`[{"name":"ponytail","source":"github","repo":"DietrichGebert/ponytail","installLocation":"/m/ponytail"},` +
			`{"name":"lich-plugin","source":"git","url":"https://github.com/lichdotdev/lich-plugin.git","ref":"v0.21.1"},` +
			`{"name":"local-mk","source":"directory","path":"/src/mk"}]`)
	case args[1] == "details":
		f.claudePluginDetails(args[2])
	default:
		f.claudePluginChange(args)
	}
}

func fakeClaudePluginList(cwd string) map[string]any {
	return map[string]any{
		"installed": []any{
			map[string]any{"id": "ponytail@ponytail", "version": "5.3.2", "scope": "user", "enabled": true, "installPath": "/c/ponytail/5.3.2", "projectEnabled": false},
			map[string]any{"id": "hello@local-mk", "version": "1.0.0", "scope": "project", "enabled": false, "installPath": "/c/hello/1.0.0",
				"readFromFolder": "/src/mk/hello", "folderVersion": "1.1.0", "projectPath": cwd, "projectEnabled": true},
			map[string]any{"id": "elsewhere@local-mk", "version": "1.0.0", "scope": "local", "enabled": true, "installPath": "/c/elsewhere/1.0.0", "projectPath": "/another/checkout"},
			map[string]any{"id": "lich@lich-plugin", "version": "0.21.1", "scope": "user", "enabled": true, "installPath": "/c/lich/0.21.1"},
		},
		"available": []any{
			map[string]any{"pluginId": "frontend-design@anthropic-plugin-directory", "name": "frontend-design", "description": "Product-grade interfaces",
				"marketplaceName": "anthropic-plugin-directory", "version": "1.1.1", "source": map[string]any{"source": "url"}, "installCount": 48000},
			map[string]any{"pluginId": "other@local-mk", "name": "other", "description": "Another one", "marketplaceName": "local-mk", "source": "./plugins/other"},
		},
	}
}

func (f *fake) claudePluginDetails(id string) {
	if id != "ponytail@ponytail" {
		fmt.Println(id + " 1.0.0\n  a wording lich cannot read")
		return
	}
	fmt.Print(`ponytail 5.3.2
  Description: Forces the laziest solution that works.
  Source: ponytail@ponytail

Component inventory
  Skills (6)  ponytail, ponytail-audit, ponytail-debt, ponytail-gain, ponytail-help, ponytail-review
  Agents (0)
  Hooks (3)  SessionStart, SubagentStart, UserPromptSubmit  (harness-only — no model context cost)
  MCP servers (1)
  LSP servers (0)

Projected token cost
  Always-on:   ~1.2k tok   added to every session
`)
}

// claudePluginChange answers a change with its --json line, failing on
// nope@x, already done on done@x, and an update of ponytail as a new version.
func (f *fake) claudePluginChange(args []string) {
	target := args[len(args)-2]
	if i := slices.Index(args, "--json"); i > 0 {
		target = args[i-1]
	}
	switch {
	case target == "nope@x":
		f.send(map[string]any{"command": args[1], "outcome": "failed", "message": `Plugin "nope" not found in marketplace "x".`, "failureCode": "not_found"})
		fmt.Fprintln(os.Stderr, "✘ Failed")
		os.Exit(1)
	case target == "done@x":
		f.send(map[string]any{"command": args[1], "outcome": "failed", "message": "already disabled", "failureCode": claudeInGoalState, "alreadyInGoalState": true})
		os.Exit(1)
	case args[1] == "update" && target == "ponytail@ponytail":
		f.send(map[string]any{"command": "update", "outcome": "ok", "updateOutcome": "updated", "oldVersion": "5.3.2", "newVersion": "5.4.0"})
	case args[1] == "update":
		f.send(map[string]any{"command": "update", "outcome": "ok", "updateOutcome": "up_to_date"})
	default:
		f.send(map[string]any{"command": args[1], "outcome": "ok", "message": "Successfully"})
	}
}

// fakeReloadedCommands is the slash list reload_plugins answers: the
// commands with a plugin's skill gone.
var fakeReloadedCommands = slices.DeleteFunc(slices.Clone(fakeClaudeCommands), func(c any) bool {
	return c.(map[string]any)["name"] == "review"
})

var fakeCodexInstalled = []any{
	map[string]any{"name": "ponytail", "path": "/m/ponytail/.agents/plugins/marketplace.json", "plugins": []any{
		map[string]any{"id": "ponytail@ponytail", "name": "ponytail", "installed": true, "enabled": true, "version": nil, "localVersion": "5.3.3",
			"interface": map[string]any{"shortDescription": "Lazy senior developer mode"}},
	}},
	map[string]any{"name": "openai-curated-remote", "path": nil, "plugins": []any{
		map[string]any{"id": "visualize@openai-curated-remote", "name": "visualize", "installed": true, "enabled": false, "version": "1.0.89", "localVersion": "1.0.89", "interface": nil},
	}},
}

var fakeCodexOffered = []any{
	map[string]any{"name": "ponytail", "path": "/m/ponytail/.agents/plugins/marketplace.json", "plugins": []any{
		map[string]any{"id": "ponytail@ponytail", "name": "ponytail", "installed": true, "enabled": true},
	}},
	map[string]any{"name": "openai-curated", "path": "/m/curated/marketplace.json", "plugins": []any{
		map[string]any{"id": "gmail@openai-curated", "name": "gmail", "installed": false, "enabled": false, "localVersion": "0.1.0",
			"interface": map[string]any{"shortDescription": nil, "longDescription": "Read and manage Gmail"}},
	}},
}

// servePlugins answers the plugin and marketplace requests, false for any
// other.
func (c *fakeCodex) servePlugins(id any, method string, params map[string]any) bool {
	switch method {
	case "plugin/installed":
		c.reply(id, map[string]any{"marketplaces": fakeCodexInstalled, "marketplaceLoadErrors": []any{}})
	case "plugin/list":
		if kinds, _ := params["marketplaceKinds"].([]any); len(kinds) != 1 || kinds[0] != "local" {
			c.send(map[string]any{"id": id, "error": map[string]any{"code": -32600, "message": "the remote catalog is 43 MB"}})
			return true
		}
		c.reply(id, map[string]any{"marketplaces": fakeCodexOffered, "marketplaceLoadErrors": []any{}, "featuredPluginIds": []any{}})
	case "plugin/read":
		c.readPlugin(id, params)
	case "plugin/install", "plugin/uninstall", "marketplace/add", "marketplace/remove":
		c.reply(id, map[string]any{})
	case "marketplace/upgrade":
		errs := []any{}
		if str(params, "marketplaceName") == "local-mk" {
			errs = append(errs, map[string]any{"marketplaceName": "local-mk", "message": "marketplace `local-mk` is not configured as a Git marketplace"})
		}
		c.reply(id, map[string]any{"errors": errs, "selectedMarketplaces": []any{}, "upgradedRoots": []any{}})
	default:
		return false
	}
	return true
}

func (c *fakeCodex) readPlugin(id any, params map[string]any) {
	if str(params, "pluginName") != "ponytail" {
		c.send(map[string]any{"id": id, "error": map[string]any{"code": -32600, "message": "not found"}})
		return
	}
	c.reply(id, map[string]any{"plugin": map[string]any{
		"marketplaceName": "ponytail", "description": "Lazy senior dev mode, long form",
		"skills": []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}}, "hooks": []any{map[string]any{"key": "h"}},
		"mcpServers": []any{}, "apps": []any{}, "appTemplates": []any{},
	}})
}

// fakeCodexMarketplaces is config.toml's [marketplaces], as config/read
// gives it.
var fakeCodexMarketplaces = map[string]any{
	"ponytail":    map[string]any{"source_type": "git", "source": "https://github.com/DietrichGebert/ponytail.git"},
	"lich-plugin": map[string]any{"source_type": "git", "source": "https://github.com/lichdotdev/lich-plugin.git"},
}
