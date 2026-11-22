//go:build !windows

package chat

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
)

// The fake's permission rules and hooks, in the shapes measured on Claude
// Code 2.1.330 and Codex 0.170.0.

// fakeRulesFile is where the fake's edit-permission-rules keeps the rules it
// added and removed, in the checkout, for its running chat to list: the real
// one writes a settings file the chat watches.
const fakeRulesFile = ".fake-permission-rules.json"

// fakeSlowRulesEnv has the fake chat list an edit only on its third read,
// the way the real one takes it a second later.
const fakeSlowRulesEnv = "LICH_CHAT_FAKE_SLOW_RULES"

var fakeClaudeRules = []map[string]any{
	{"behavior": "allow", "source": "projectSettings", "rule": "Bash(npm run *)", "editability": "persistent",
		"description": map[string]any{"prefix": "Any Bash command starting with ", "emphasis": "npm run"}},
	{"behavior": "ask", "source": "userSettings", "rule": "WebFetch", "editability": "persistent",
		"description": map[string]any{"prefix": "Any use of the ", "emphasis": "WebFetch", "suffix": " tool"}},
	{"behavior": "deny", "source": "policySettings", "rule": "Read(./.env)", "editability": "readonly"},
	{"behavior": "allow", "source": "cliArg", "rule": "Edit", "editability": "session"},
	{"behavior": "allow", "source": "localSettings", "rule": "Bash(rm *)", "editability": "persistent", "notInEffect": true},
}

// fakeRules is the rules the fake chat lists: its own, less those removed,
// with those added.
func fakeRules() []map[string]any {
	var edits []map[string]any
	raw, _ := os.ReadFile(fakeRulesFile)
	_ = json.Unmarshal(raw, &edits)
	rules := slices.Clone(fakeClaudeRules)
	for _, e := range edits {
		if e["op"] == "remove" {
			rules = slices.DeleteFunc(rules, func(r map[string]any) bool { return r["rule"] == e["rule"] && r["behavior"] == e["behavior"] })
			continue
		}
		rules = append(rules, map[string]any{"behavior": e["behavior"], "source": e["destination"], "rule": e["stored"], "editability": "persistent"})
	}
	return rules
}

// claudeSettings answers list_permission_rules and get_hooks_listing, false
// for any other request.
func (f *fake) claudeSettings(subtype string, ok func(any)) bool {
	switch subtype {
	case "list_permission_rules":
		if os.Getenv(fakeSlowRulesEnv) != "" && f.asks.Add(1) < 3 {
			ok(map[string]any{"state": fakeRuleState(fakeClaudeRules)})
			return true
		}
		ok(map[string]any{"state": fakeRuleState(fakeRules())})
	case "get_hooks_listing":
		ok(fakeClaudeHooks)
	default:
		return false
	}
	return true
}

func fakeRuleState(rules []map[string]any) map[string]any {
	return map[string]any{
		"rules": rules, "originalCwd": "/repo", "managedOnly": false,
		"workspaceDirectories": []any{map[string]any{"path": "/tmp", "source": "localSettings"}, map[string]any{"path": "/srv", "source": "cliArg"}},
		"errors":               []any{map[string]any{"path": "/repo/.claude/settings.json", "message": "Invalid JSON"}},
	}
}

// editPermissionRules is `claude edit-permission-rules --json`: it refuses
// to run inside another session and refuses a rule that holds a secret, its
// reason alone on stderr, the way the real one does, and saves a bare tool's
// "Bash(*)" as "Bash".
func (f *fake) editPermissionRules() {
	if os.Getenv(claudeChildEnv) != "" {
		fmt.Fprintln(os.Stderr, "claude edit-permission-rules is run by the VS Code extension, not from a session")
		os.Exit(1)
	}
	var edit map[string]any
	_ = json.NewDecoder(os.Stdin).Decode(&edit)
	if f.log != nil {
		cwd, _ := os.Getwd()
		line, _ := json.Marshal(edit)
		fmt.Fprintf(f.log, "edit-rules %s in %s\n", line, cwd)
	}
	reply := map[string]any{"ok": true, "warnings": []any{}, "stored": []any{}}
	if edit["op"] == "add" {
		rule := edit["rules"].([]any)[0].(string)
		if rule == "Bash(curl -H token:abc)" {
			fmt.Fprintln(os.Stderr, "the rule looks like it contains a secret, so it was not saved to settings")
			os.Exit(1)
		}
		stored := rule
		if rule == "Bash(*)" {
			stored = "Bash"
			reply["warnings"] = []any{`"Bash(*)" was saved as the tool-wide rule "Bash", which matches every use of the tool.`}
		}
		reply["stored"], edit["stored"] = []any{stored}, stored
	}
	var edits []map[string]any
	raw, _ := os.ReadFile(fakeRulesFile)
	_ = json.Unmarshal(raw, &edits)
	raw, _ = json.Marshal(append(edits, edit))
	_ = os.WriteFile(fakeRulesFile, raw, 0o600)
	f.send(reply)
}

var fakeClaudeHooks = map[string]any{
	"events": []any{
		map[string]any{"name": "PreToolUse", "summary": "Before tool execution", "supportsMatcher": true, "hookCount": 2},
		map[string]any{"name": "Stop", "summary": "Right before Claude concludes its response", "supportsMatcher": false, "hookCount": 1},
	},
	"hooks": []any{
		map[string]any{"event": "PreToolUse", "matcher": "Bash", "source": "projectSettings", "sourceLabel": "Project settings (.claude/settings.json)",
			"type": "command", "displayText": "echo pre", "commandText": "echo pre", "contentLabel": "Command", "timeout": 5,
			"editable": map[string]any{"matcher": "Bash", "config": map[string]any{"type": "command", "command": "echo pre"}}},
		map[string]any{"event": "PreToolUse", "matcher": "", "source": "pluginHook", "sourceLabel": "Plugin", "pluginName": "lich",
			"type": "command", "displayText": "Checking", "commandText": "node hook.js", "contentLabel": "Command", "disabled": true},
		map[string]any{"event": "Stop", "matcher": "", "source": "userSettings", "sourceLabel": "User settings (~/.claude/settings.json)",
			"type": "prompt", "displayText": "Is it done?", "commandText": "Is it done?", "contentLabel": "Prompt", "condition": "Bash(git *)"},
	},
	"eventCatalog": []any{},
	"policy":       map[string]any{"disabledByPolicy": false, "managedOnly": false, "pluginOnly": false, "allDisabled": true, "policyHookCount": 0},
}

// fakeCodexHookState is the hooks.state the fake Codex was written, by key.
var fakeCodexHookState = map[string]map[string]any{}

// serveHooks answers hooks/list and config/batchWrite.
func (c *fakeCodex) serveHooks(id any, method string, params map[string]any) {
	switch method {
	case "hooks/list":
		c.reply(id, map[string]any{"data": []any{map[string]any{"cwd": "/x", "warnings": []any{}, "hooks": fakeCodexHooks(),
			"errors": []any{map[string]any{"path": "/x/.codex/hooks.json", "message": "bad matcher"}}}}})
	case "config/batchWrite":
		for _, e := range params["edits"].([]any) {
			edit := e.(map[string]any)
			for key, v := range edit["value"].(map[string]any) {
				if fakeCodexHookState[key] == nil {
					fakeCodexHookState[key] = map[string]any{}
				}
				maps.Copy(fakeCodexHookState[key], v.(map[string]any))
			}
		}
		c.reply(id, map[string]any{"status": "ok", "version": "sha256:v", "filePath": "/x/config.toml"})
	}
}

func fakeCodexHooks() []any {
	hook := func(key, event, matcher, source string, managed bool, hash string) map[string]any {
		st := fakeCodexHookState[key]
		trust := "untrusted"
		if st["trusted_hash"] == hash {
			trust = "trusted"
		}
		if managed {
			trust = "managed"
		}
		enabled, set := st["enabled"].(bool)
		return map[string]any{"key": key, "eventName": event, "handlerType": "command", "matcher": matcher, "command": "echo " + event,
			"timeoutSec": 600, "statusMessage": nil, "source": source, "pluginId": nil, "displayOrder": 0, "enabled": !set || enabled,
			"isManaged": managed, "currentHash": hash, "trustStatus": trust, "sourcePath": "/x/hooks.json"}
	}
	return []any{
		hook("/h/hooks.json:pre_tool_use:0:0", "preToolUse", "Bash", "user", false, "sha256:pre"),
		hook("/h/hooks.json:stop:0:0", "stop", "", "project", false, "sha256:stop"),
		hook("mdm:session_start:0:0", "sessionStart", "", "mdm", true, "sha256:mdm"),
	}
}
