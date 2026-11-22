//go:build !windows

package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The fake's memory and subagents, in the shapes measured on Claude Code
// 2.1.330 and Codex 0.170.0. Its config dir is $HOME/claude, as the rig's
// CLAUDE_CONFIG_DIR is, and its project the folder it runs in.

// fakeNoMemoryEnv has the fake Claude Code refuse get_memory_dialog, the way
// one that dropped the @internal request would.
const fakeNoMemoryEnv = "LICH_CHAT_FAKE_NOMEMORY"

// fakeClaudeAgents is what initialize lists: two built in and the user's
// reviewer, whose file the test writes.
var fakeClaudeAgents = []any{
	map[string]any{"name": "Explore", "description": "Search the code", "model": "inherit"},
	map[string]any{"name": "general-purpose", "description": "Anything else"},
	map[string]any{"name": "reviewer", "description": "Reviews code", "model": "sonnet"},
}

func fakeClaudeDir() string { return filepath.Join(os.Getenv("HOME"), "claude") }

// claudeMemory answers the memory and subagent requests, false for any
// other.
func (f *fake) claudeMemory(subtype string, ok func(any), fail func(string)) bool {
	switch subtype {
	case "get_memory_dialog":
		if os.Getenv(fakeNoMemoryEnv) != "" {
			fail("Unknown subtype: get_memory_dialog")
			return true
		}
		ok(fakeMemoryDialog())
	case "get_context_usage":
		ok(map[string]any{"agents": fakeFiledAgents()})
	default:
		return false
	}
	return true
}

func fakeMemoryDialog() map[string]any {
	cwd, _ := os.Getwd()
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
	user, project := filepath.Join(fakeClaudeDir(), "CLAUDE.md"), filepath.Join(cwd, "CLAUDE.md")
	auto := !strings.Contains(fakeSettings(), `"autoMemoryEnabled":false`)
	return map[string]any{
		"files": []any{
			map[string]any{"kind": "user", "path": user, "label": "User instructions", "description": "Saved in ~/.claude/CLAUDE.md", "exists": exists(user)},
			map[string]any{"kind": "project", "path": project, "label": "Project instructions", "description": "Checked in at ./CLAUDE.md", "exists": exists(project)},
			map[string]any{"kind": "other", "path": filepath.Join(cwd, "imp.md"), "label": "  L imp.md", "description": "@-imported", "exists": true},
		},
		"folders": []any{map[string]any{"kind": "auto", "path": filepath.Join(fakeClaudeDir(), "memory") + "/", "label": "Open auto-memory folder", "description": ""}},
		"memories": []any{
			map[string]any{"name": "MEMORY.md", "path": filepath.Join(fakeClaudeDir(), "memory", "MEMORY.md"), "description": "Index of saved memories", "type": nil, "modified_ms": 1795354671889.8262},
			map[string]any{"name": "a.md", "path": filepath.Join(fakeClaudeDir(), "memory", "a.md"), "description": "A fact", "type": "user", "modified_ms": 1795354671889.8262},
		},
		"auto_memory": map[string]any{"enabled": auto, "toggleable": true, "status": "on"},
		"auto_dream":  map[string]any{"shown": false, "enabled": false, "toggleable": true, "status": "off", "detail": ""},
	}
}

// fakeFiledAgents lists the agents files of the user's and the project's
// folders, as get_context_usage does, built-ins left out.
func fakeFiledAgents() []any {
	cwd, _ := os.Getwd()
	var out []any
	for source, dir := range map[string]string{"userSettings": filepath.Join(fakeClaudeDir(), "agents"), "projectSettings": filepath.Join(cwd, ".claude", "agents")} {
		names, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		for _, p := range names {
			out = append(out, map[string]any{"agentType": strings.TrimSuffix(filepath.Base(p), ".md"), "source": source, "path": p, "tokens": 9})
		}
	}
	return out
}

func fakeSettings() string {
	data, _ := os.ReadFile(filepath.Join(fakeClaudeDir(), "settings.json"))
	return string(data)
}

// editMemorySettings is `claude edit-memory-settings --json`: it refuses to
// run inside another session, and writes the rest to settings.json.
func (f *fake) editMemorySettings() {
	if os.Getenv(claudeChildEnv) != "" {
		fmt.Println("This setting cannot be changed from an editor started inside a Claude Code session. Start the editor outside the session and try again.")
		os.Exit(1)
	}
	var in map[string]bool
	_ = json.NewDecoder(os.Stdin).Decode(&in)
	data, _ := json.Marshal(in)
	_ = os.MkdirAll(fakeClaudeDir(), 0o755)
	_ = os.WriteFile(filepath.Join(fakeClaudeDir(), "settings.json"), data, 0o644)
	f.send(map[string]any{"ok": true, "written": in})
}

// fakeCodexAgents is config/read's agents table: its own settings and one
// role.
var fakeCodexAgents = map[string]any{
	"max_threads": nil, "max_depth": nil, "job_max_runtime_seconds": nil, "interrupt_message": nil,
	"reviewer": map[string]any{"description": "Reviews code", "config_file": "/x/reviewer.toml", "nickname_candidates": nil},
}

// fakeCodexSources is instructionSources: the AGENTS.md of the personal
// CODEX_HOME, then the project's.
func fakeCodexSources() []any {
	return []any{filepath.Join(os.Getenv("HOME"), ".codex", "AGENTS.md"), "/repo/AGENTS.md", "/repo/sub/AGENTS.override.md"}
}
