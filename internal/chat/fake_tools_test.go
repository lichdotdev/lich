//go:build !windows

package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

// The fake's MCP servers and skills, in the shapes measured on Claude Code
// 2.1.330 and Codex 0.170.0.

// fakeNoSkillsEnv has the fake Claude Code refuse get_skills_dialog, the way
// one that dropped the @internal request would.
const fakeNoSkillsEnv = "LICH_CHAT_FAKE_NOSKILLS"

// fakeSecret is in every server's env and headers, which lich must not pass on.
const fakeSecret = "s3cr3t-token"

var fakeClaudeMCP = []any{
	map[string]any{"name": "lich", "status": "connected", "scope": "dynamic", "transport": "stdio",
		"config": map[string]any{"type": "stdio", "command": "/bin/lich", "args": []any{"mcp"}}},
	map[string]any{"name": "memory", "status": "connected", "scope": "user", "transport": "http", "serverInfo": map[string]any{"name": "mem", "version": "1.2"},
		"config": map[string]any{"type": "http", "url": "https://mem.example/mcp", "headers": map[string]any{"Authorization": fakeSecret}},
		"tools":  []any{map[string]any{"name": "recall", "description": "Recall it", "annotations": map[string]any{"readOnly": true}}}},
	map[string]any{"name": "local-tool", "status": "failed", "error": "spawn failed", "scope": "local", "transport": "stdio",
		"config": map[string]any{"type": "stdio", "command": "node", "args": []any{"srv.js", "--port", "1"}, "env": map[string]any{"TOKEN": fakeSecret}}},
	map[string]any{"name": "repo", "status": "pending", "scope": "project", "transport": "stdio", "config": map[string]any{"command": "repo-mcp"}},
	map[string]any{"name": "claude.ai Drive", "status": "needs-auth", "scope": "claudeai", "transport": "claudeai-proxy"},
	map[string]any{"name": "claude.ai Docs", "status": "connected", "scope": "claudeai", "transport": "claudeai-proxy"},
}

var fakeClaudeSkills = []any{
	map[string]any{"name": "deploy", "display_name": "Deploy", "description": "Ship it", "source": "user", "tokens": 163, "state": "on", "advertised": true},
	map[string]any{"name": "plug", "description": "A plugin's", "source": "plugin", "tokens": 40, "state": "user-invocable-only", "locked_by": "plugin"},
}

// claudeTools answers the MCP and skill requests, false for any other.
func (f *fake) claudeTools(subtype string, m map[string]any, ok func(any), fail func(string)) bool {
	switch subtype {
	case "mcp_status":
		ok(map[string]any{"mcpServers": fakeClaudeMCP})
	case "mcp_toggle", "mcp_reconnect":
		if str(m, "request", "serverName") == "nope" {
			fail("Server not found: nope")
			return true
		}
		ok(nil)
	case "mcp_authenticate":
		f.claudeAuthenticate(str(m, "request", "serverName"), ok)
	case "get_skills_dialog":
		if os.Getenv(fakeNoSkillsEnv) != "" {
			fail("Unknown subtype: get_skills_dialog")
			return true
		}
		ok(map[string]any{"skills": fakeClaudeSkills})
	case "reload_plugins":
		ok(map[string]any{"commands": fakeReloadedCommands, "agents": []any{}, "plugins": []any{}, "mcpServers": []any{}, "error_count": 0})
	case "reload_skills":
		ok(map[string]any{"skills": slices.DeleteFunc(slices.Clone(fakeClaudeCommands), func(c any) bool {
			return c.(map[string]any)["name"] == "deploy"
		})})
	default:
		return f.claudeSettings(subtype, ok)
	}
	return true
}

func (f *fake) claudeAuthenticate(server string, ok func(any)) {
	switch server {
	case "claude.ai Drive":
		ok(map[string]any{"authUrl": "https://claude.ai/connect/drive", "requiresUserAction": true, "callbackExpected": false})
	case "relay-only":
		ok(map[string]any{"authUrl": "https://idp.example/auth", "requiresUserAction": true, "callbackExpected": true, "redirectScheme": "http"})
	case "memory":
		ok(map[string]any{"requiresUserAction": false, "callbackExpected": false})
	default:
		ok(map[string]any{"authUrl": "https://idp.example/auth?port", "requiresUserAction": true, "callbackExpected": true, "callbackPort": 52123})
	}
}

// editSkillOverrides is `claude edit-skill-overrides --json`: it refuses to
// run inside another session, refuses a plugin's skill, and logs the rest.
func (f *fake) editSkillOverrides() {
	if os.Getenv(claudeChildEnv) != "" {
		fmt.Fprintln(os.Stderr, "Cannot edit skill overrides from inside a child session")
		os.Exit(1)
	}
	var in struct{ Name, State string }
	_ = json.NewDecoder(os.Stdin).Decode(&in)
	if in.Name == "plug" {
		f.send(map[string]any{"ok": false, "error": "A plugin skill is managed with the plugin"})
		os.Exit(1)
	}
	if f.log != nil {
		cwd, _ := os.Getwd()
		fmt.Fprintf(f.log, "edit-skill %s %s in %s\n", in.Name, in.State, cwd)
	}
	f.send(map[string]any{"ok": true, "written": map[string]any{"skillOverrides": map[string]any{in.Name: in.State}}, "changed": true})
}

var fakeCodexMCP = []any{
	map[string]any{"name": "codex_apps", "authStatus": "bearerToken", "tools": map[string]any{
		"b_tool": map[string]any{"name": "b_tool", "description": "B"}, "a_tool": map[string]any{"name": "a_tool"},
	}, "resources": []any{}, "resourceTemplates": []any{}},
	map[string]any{"name": "stub", "authStatus": "unsupported", "tools": map[string]any{}, "resources": []any{}, "resourceTemplates": []any{}},
	map[string]any{"name": "linear", "authStatus": "notLoggedIn", "runtimeStatus": "authenticationRequired", "toolsError": "login needed",
		"tools": map[string]any{}, "resources": []any{}, "resourceTemplates": []any{}},
	map[string]any{"name": "lich", "authStatus": "unsupported", "runtimeStatus": "connected", "serverInfo": map[string]any{"name": "lich", "version": "9"},
		"tools": map[string]any{"list_sessions": map[string]any{"name": "list_sessions"}}, "resources": []any{}, "resourceTemplates": []any{}},
}

var fakeCodexConfig = map[string]any{
	"stub":   map[string]any{"command": "python3", "args": []any{"stub.py"}, "env": map[string]any{"TOKEN": fakeSecret}, "enabled": false},
	"linear": map[string]any{"url": "https://mcp.linear.app/mcp", "bearer_token_env_var": fakeSecret},
	"lich":   map[string]any{"command": "/bin/lich", "args": []any{"mcp"}},
}

// serveTools answers the MCP and skill requests, and no other.
func (c *fakeCodex) serveTools(id any, method string, params map[string]any) {
	switch method {
	case "mcpServerStatus/list":
		// Two pages, the second behind a cursor.
		if str(params, "cursor") == "" {
			c.reply(id, map[string]any{"data": fakeCodexMCP[:2], "nextCursor": "m2"})
			return
		}
		c.reply(id, map[string]any{"data": fakeCodexMCP[2:], "nextCursor": nil})
	case "config/read":
		c.reply(id, map[string]any{"config": map[string]any{"model": "gpt-a", "mcp_servers": fakeCodexConfig, "marketplaces": fakeCodexMarketplaces}, "origins": map[string]any{}})
	case "config/value/write":
		c.reply(id, map[string]any{"status": "ok", "version": "sha256:x", "filePath": "/x/config.toml"})
	case "config/mcpServer/reload":
		c.reply(id, map[string]any{})
	case "mcpServer/oauth/login":
		c.reply(id, map[string]any{"authorizationUrl": "https://mcp.linear.app/authorize?x=1", "loginId": "l1"})
	case "skills/list":
		c.skills(id)
	case "skills/config/write":
		path, _ := params["path"].(string)
		on, _ := params["enabled"].(bool)
		if c.off == nil {
			c.off = map[string]bool{}
		}
		c.off[path] = !on
		c.reply(id, map[string]any{"effectiveEnabled": on})
	default:
		if !c.servePlugins(id, method, params) {
			c.serveHooks(id, method, params)
		}
	}
}

func (c *fakeCodex) skills(id any) {
	skill := func(dir, name, desc, short, scope string, on bool) map[string]any {
		path := dir + "/skills/" + name + "/SKILL.md"
		if v, ok := c.off[path]; ok {
			on = !v
		}
		return map[string]any{"name": name, "description": desc, "shortDescription": short, "path": path, "scope": scope, "enabled": on}
	}
	c.reply(id, map[string]any{"data": []any{map[string]any{"cwd": "/x", "errors": []any{}, "skills": []any{
		skill("", "deploy", "Ship the build to staging, long form", "Ship it", "user", true),
		skill("", "off", "Turned off", "", "user", false),
		skill("/repo", "deploy", "A repo's deploy", "", "repo", true),
	}}}})
}
