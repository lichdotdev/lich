package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

var memoryList = api.ChatMemoryList{
	Reported: true,
	Files: []api.ChatMemoryFile{
		{Kind: api.MemoryUser, Path: nativeTestPath("/Users/me/.claude/CLAUDE.md")},
		{Kind: api.MemoryProject, Path: nativeTestPath("/repo/CLAUDE.md"), Exists: true},
		{Kind: api.MemoryOther, Path: nativeTestPath("/repo/docs/rules.md"), Imported: true, Exists: true},
	},
	Memories:   []api.ChatMemoryNote{{Name: "lich-uses-pnpm.md", Path: nativeTestPath("/Users/me/.claude/projects/repo/memory/lich-uses-pnpm.md"), Description: "frontend is pnpm", Type: "project"}},
	AutoMemory: &api.ChatMemoryToggle{Enabled: true},
}

var agentList = api.ChatAgentList{
	Reported: true,
	Create:   []string{api.AgentProject, api.AgentUser},
	Agents: []api.ChatAgent{
		{Name: "Explore", Description: "Search the code", Model: "inherit", Source: api.AgentBuiltIn},
		{Name: "reviewer", Description: "Reviews code", Model: "sonnet", Source: api.AgentUser, Path: nativeTestPath("/Users/me/.claude/agents/reviewer.md"), Tools: []string{"Read", "Grep"}},
		{Name: "helper", Description: "Helps", Source: api.AgentProject, Path: nativeTestPath("/repo/.claude/agents/helper.md")},
		{Name: "plug-agent", Description: "A plugin's", Source: api.AgentPlugin, Path: nativeTestPath("/Users/me/.claude/plugins/p/agents/plug-agent.md"), ReadOnly: true},
	},
}

// memoryFrames is toolsFrames with the memory and subagents answered and the
// session in checkout /repo, whose editor a row opens.
func memoryFrames(t *testing.T, command string) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	f.answers = map[string]any{api.ChatMemory.Name: memoryList, api.ChatAgents.Name: agentList, api.ChatNewAgent.Name: api.ChatNewAgentResult{Path: nativeTestPath("/repo/.claude/agents/scout.md")}}
	u.model.projects[0].Path = nativeTestPath("/repo")
	u.settings, u.pulls, u.extension = new(settingsScreen), new(pullsScreen), new(extensionScreen)
	u.editors = stageEditors{client: u.client, invalidate: func() {}, dirty: u.model.markEditorDirty, saved: func(string) {}}
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}})
	r.settle(t)
	p := u.chats.pane("s1")
	typeIn(r, p, command)
	r.press(key.NameReturn)
	r.press(key.NameReturn)
	toolsSettle(r)
	return u, r, f, p
}

// /memory lists the instruction files, the setting and the saved memories
// instead of reaching the agent.
func TestMemoryOpensTheFilesInsteadOfSending(t *testing.T) {
	u, r, f, _ := memoryFrames(t, "/memory")
	// Group labels are uppercase since the composer panels share the palette's
	// group heading (contract changed).
	if !r.showsAll("Memory", "User instructions", "not created yet", "Project instructions", filepath.FromSlash("docs/rules.md"), "Auto-memory", "SAVED MEMORIES", "lich-uses-pnpm", "frontend is pnpm") {
		t.Errorf("the memory is not drawn:\n%s", r.drawn())
	}
	if u.model.chat.isRunning("s1") || f.callsOf(api.ChatPrompt.Name) != 0 {
		t.Error("/memory went to the agent")
	}
}

// A file of the checkout opens in its editor; a missing one is created first;
// the setting turns over.
func TestMemoryRowsOpenCreateAndTurn(t *testing.T) {
	u, r, f, p := memoryFrames(t, "/memory")
	r.click(t, r.at(t, "Auto-memory"))
	if body := f.waitCall(t, api.ChatSetMemory.Name); !strings.Contains(body, `"setting":"auto-memory"`) || strings.Contains(body, `"enabled":true`) {
		t.Errorf("the setting asked %s", body)
	}
	r.click(t, r.at(t, "Project instructions"))
	if ed := u.editors.byCheckout[nativeTestPath("/repo")]; ed == nil || ed.rel != "CLAUDE.md" || ed.dir != "" || p.tools.opened() {
		t.Fatalf("the project's file did not open in the checkout's editor: %+v", ed)
	}
	p.tools.open(chatMemoryCommand)
	u.fetchTools(p, "s1")
	toolsSettle(r)
	r.click(t, r.at(t, "User instructions"))
	wantPath, _ := json.Marshal(nativeTestPath("/Users/me/.claude/CLAUDE.md"))
	if body := f.waitCall(t, api.ChatCreateMemory.Name); !strings.Contains(body, string(wantPath)) {
		t.Errorf("create asked %s", body)
	}
	toolsSettle(r)
	u.takeCodeAsk()
	if ed := u.editors.byCheckout[nativeTestPath("/repo")]; ed.dir != nativeTestPath("/Users/me/.claude") || ed.rel != "CLAUDE.md" || !ed.writable {
		t.Errorf("the user's file opened as %q in %q writable %v", ed.rel, ed.dir, ed.writable)
	}
}

// /agents files the subagents by where they come from; one of a file opens
// it, a plugin's read only, and the fields add a new one.
func TestAgentsListOpenAndAdd(t *testing.T) {
	u, r, f, p := memoryFrames(t, "/agents")
	// Group labels are uppercase since the composer panels share the palette's
	// group heading (contract changed).
	if !r.showsAll("Subagents", "PROJECT", "USER", filepath.FromSlash("~/.claude/agents"), "BUILT-IN", "reviewer", "sonnet · 2 tools", "PLUGIN", "Add to this project", "Add to all projects") {
		t.Fatalf("the subagents are not drawn:\n%s", r.drawn())
	}
	writeShot(t, r, "agents-list.png")
	r.click(t, r.at(t, "plug-agent"))
	if ed := u.editors.byCheckout[nativeTestPath("/repo")]; ed == nil || ed.dir != nativeTestPath("/Users/me/.claude/plugins/p/agents") || ed.writable {
		t.Fatalf("the plugin's agent did not open read only: %+v", ed)
	}
	p.tools.open(chatAgentsCommand)
	u.fetchTools(p, "s1")
	toolsSettle(r)
	p.tools.mem.name.SetText("scout")
	r.frames(2)
	r.press(key.NameReturn)
	r.frames(2)
	p.tools.mem.desc.SetText("Finds things")
	r.frames(2)
	r.press(key.NameReturn)
	body := f.waitCall(t, api.ChatNewAgent.Name)
	if !strings.Contains(body, `"scope":"project"`) || !strings.Contains(body, `"name":"scout"`) || !strings.Contains(body, `"description":"Finds things"`) {
		t.Errorf("new agent asked %s", body)
	}
	toolsSettle(r)
	u.takeCodeAsk()
	if ed := u.editors.byCheckout[nativeTestPath("/repo")]; ed.rel != ".claude/agents/scout.md" || ed.dir != "" {
		t.Errorf("the new agent opened as %q in %q", ed.rel, ed.dir)
	}
}

// Only Claude Code and Codex get the commands.
func TestMemoryCommandsOnlyWhereReported(t *testing.T) {
	has := func(kind, text string) bool {
		for _, c := range slashCommands(nil, kind) {
			if c.Text == text {
				return true
			}
		}
		return false
	}
	if !has(claudeKind, "/memory") || !has(codexKind, "/agents") || has("opencode", "/memory") {
		t.Error("the commands are offered where they should not be, or missing")
	}
}

// Codex's /agents has no fields to add one: the keys stay on the box, and
// the arrows, Enter and Escape walk and shut the list.
func TestAgentsWithoutFieldsKeepTheKeysOnTheBox(t *testing.T) {
	u, r, f, p := memoryFrames(t, "/agents")
	f.mu.Lock()
	f.answers[api.ChatAgents.Name] = api.ChatAgentList{Reported: true, Agents: []api.ChatAgent{
		{Name: "reviewer", Source: api.AgentOther, Path: nativeTestPath("/Users/me/.codex/reviewer.toml")},
		{Name: "writer", Source: api.AgentOther, Path: nativeTestPath("/Users/me/.codex/writer.toml")},
	}}
	f.mu.Unlock()
	p.tools.open(chatAgentsCommand)
	u.fetchTools(p, "s1")
	toolsSettle(r)
	if p.tools.keyTag(p) != &p.editor || r.shows("Add to this project") {
		t.Fatal("a panel with no fields took the keys or drew them")
	}
	r.press(key.NameDownArrow)
	r.press(key.NameReturn)
	if ed := u.editors.byCheckout[nativeTestPath("/repo")]; ed == nil || ed.rel != "writer.toml" || !ed.writable {
		t.Fatalf("Down and Enter opened %+v, want writer.toml", ed)
	}
	p.tools.open(chatAgentsCommand)
	u.fetchTools(p, "s1")
	toolsSettle(r)
	r.press(key.NameEscape)
	if p.tools.opened() {
		t.Error("Escape left the panel open")
	}
}
