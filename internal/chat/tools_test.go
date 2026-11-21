//go:build !windows

package chat

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func (r *rig) mcp() api.ChatMCPList {
	r.t.Helper()
	list, err := r.svc.MCP(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("MCP: %v", err)
	}
	return list
}

func (r *rig) act(server, action string) (api.ChatMCPActResult, error) {
	return r.svc.MCPAct(r.t.Context(), api.ChatMCPActOptions{ID: "s1", Server: server, Action: action})
}

func (r *rig) skills() api.ChatSkillList {
	r.t.Helper()
	list, err := r.svc.Skills(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("Skills: %v", err)
	}
	return list
}

func (r *rig) setSkill(name string, enabled bool) error {
	_, err := r.svc.SetSkill(r.t.Context(), api.ChatSetSkillOptions{ID: "s1", Name: name, Enabled: enabled})
	return err
}

// logged reports whether the fake read a line holding every one of parts.
func (r *rig) logged(parts ...string) bool {
	for line := range strings.Lines(r.fakeLog()) {
		if !slices.ContainsFunc(parts, func(p string) bool { return !strings.Contains(line, p) }) {
			return true
		}
	}
	return false
}

func serverNamed(t *testing.T, list api.ChatMCPList, name string) api.ChatMCPServer {
	t.Helper()
	i := slices.IndexFunc(list.Servers, func(s api.ChatMCPServer) bool { return s.Name == name })
	if i < 0 {
		t.Fatalf("no server %q in %+v", name, list.Servers)
	}
	return list.Servers[i]
}

// noSecrets fails when v carries what a server's env or headers hold.
func noSecrets(t *testing.T, v any) {
	t.Helper()
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), fakeSecret) {
		t.Errorf("the list carries a server's env or headers: %s", raw)
	}
}

// Claude Code's mcp_status is the list: the URL or command as the target,
// never the env or headers; the file named by scope; sign-in from the status
// and transport; lich's own server fixed.
func TestClaudeMCPList(t *testing.T) {
	r := newChat(t, providers.Claude)
	list := r.mcp()
	noSecrets(t, list)
	if !list.Reported || !slices.Equal(list.Actions, mcpActions) || len(list.Servers) != len(fakeClaudeMCP) {
		t.Fatalf("list = %+v", list)
	}
	global := filepath.Join(r.home, "claude", ".claude.json")
	want := map[string]api.ChatMCPServer{
		"lich": {Name: "lich", Status: api.MCPConnected, Scope: "dynamic", Transport: "stdio", Target: "/bin/lich mcp", Fixed: true},
		"memory": {Name: "memory", Status: api.MCPConnected, Scope: "user", Config: global, Transport: "http", Target: "https://mem.example/mcp", Version: "1.2",
			Tools: []api.ChatMCPTool{{Name: "recall", Description: "Recall it"}}},
		"local-tool":      {Name: "local-tool", Status: api.MCPFailed, Scope: "local", Config: global, Transport: "stdio", Target: "node srv.js --port 1", Error: "spawn failed"},
		"repo":            {Name: "repo", Status: api.MCPPending, Scope: "project", Config: filepath.Join(r.home, "proj", ".mcp.json"), Transport: "stdio", Target: "repo-mcp"},
		"claude.ai Drive": {Name: "claude.ai Drive", Status: api.MCPNeedsAuth, Scope: "claudeai", Transport: claudeAITransport, Auth: api.MCPNotSignedIn},
		"claude.ai Docs":  {Name: "claude.ai Docs", Status: api.MCPConnected, Scope: "claudeai", Transport: claudeAITransport, Auth: api.MCPSignedIn},
	}
	for name, w := range want {
		if got := serverNamed(t, list, name); !jsonEqual(got, w) {
			t.Errorf("%s = %+v, want %+v", name, got, w)
		}
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// Each action is its control request; lich's own server is never turned off,
// and a sign-in whose callback would need relaying is refused.
func TestClaudeMCPAct(t *testing.T) {
	r := newChat(t, providers.Claude)
	if _, err := r.act("lich", api.MCPDisable); !errors.Is(err, errFixedMCP) {
		t.Errorf("disable lich: %v, want errFixedMCP", err)
	}
	if r.logged(`"mcp_toggle"`) {
		t.Error("disabling lich reached Claude Code")
	}
	if _, err := r.act("memory", api.MCPDisable); err != nil || !r.logged(`"mcp_toggle"`, `"serverName":"memory"`, `"enabled":false`) {
		t.Errorf("disable memory: %v, log %s", err, r.fakeLog())
	}
	if _, err := r.act("memory", api.MCPEnable); err != nil || !r.logged(`"mcp_toggle"`, `"enabled":true`) {
		t.Errorf("enable memory: %v", err)
	}
	if _, err := r.act("nope", api.MCPReconnect); err == nil || !strings.Contains(err.Error(), "Server not found") {
		t.Errorf("reconnect nope: %v", err)
	}
	if _, err := r.act("memory", "explode"); err == nil {
		t.Error("an action the agent does not offer was taken")
	}
	for server, want := range map[string]string{"claude.ai Drive": "https://claude.ai/connect/drive", "memory": "", "repo": "https://idp.example/auth?port"} {
		if got, err := r.act(server, api.MCPSignIn); err != nil || got.AuthURL != want {
			t.Errorf("sign in %s = %q, %v, want %q", server, got.AuthURL, err, want)
		}
	}
	if _, err := r.act("relay-only", api.MCPSignIn); !errors.Is(err, errNoMCPPage) {
		t.Errorf("sign in relay-only: %v, want errNoMCPPage", err)
	}
}

// get_skills_dialog is the list; refused, as an @internal request may come
// to be, the list is not reported rather than an error.
func TestClaudeSkills(t *testing.T) {
	r := newChat(t, providers.Claude)
	want := []api.ChatSkill{
		{Name: "deploy", Description: "Ship it", Source: "user", Tokens: 163, State: api.SkillOn},
		{Name: "plug", Description: "A plugin's", Source: "plugin", Tokens: 40, State: api.SkillUserInvocableOnly, Locked: "plugin"},
	}
	if got := r.skills(); !got.Reported || !slices.Equal(got.Skills, want) {
		t.Errorf("skills = %+v", got)
	}
	t.Setenv(fakeNoSkillsEnv, "1")
	r = newChat(t, providers.Claude)
	if got := r.skills(); got.Reported || got.Skills != nil {
		t.Errorf("refused skills = %+v, want not reported", got)
	}
}

// A skill is turned off by the CLI in the session's checkout, run without the
// variable that makes it refuse, and the slash list loses it on the reload.
func TestClaudeSetSkill(t *testing.T) {
	t.Setenv(claudeChildEnv, "1")
	r := newChat(t, providers.Claude)
	r.waitCommands([]api.ChatCommand{
		{Text: "/review", Description: "Review a pull request", Hint: "[pr]"},
		{Text: "/deploy", Description: "Ship it"},
		{Text: "/doctor", Description: "Check the install"},
	})
	if err := r.setSkill("deploy", false); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	if !r.logged("edit-skill deploy off in ", filepath.Join("proj")) || !r.logged(`"reload_skills"`) {
		t.Errorf("log = %s", r.fakeLog())
	}
	r.waitCommands([]api.ChatCommand{
		{Text: "/review", Description: "Review a pull request", Hint: "[pr]"},
		{Text: "/doctor", Description: "Check the install"},
	})
	if err := r.setSkill("deploy", true); err != nil || !r.logged("edit-skill deploy on in ") {
		t.Errorf("SetSkill on: %v", err)
	}
	if err := r.setSkill("plug", false); err == nil || !strings.Contains(err.Error(), "managed with the plugin") {
		t.Errorf("SetSkill plug: %v", err)
	}
}

// The ACP providers and Kiro report neither, and take no action.
func TestACPReportsNoTools(t *testing.T) {
	for _, kind := range []string{providers.OpenCode, providers.Kiro} {
		t.Run(kind, func(t *testing.T) {
			r := newChat(t, kind)
			if got := r.mcp(); got.Reported {
				t.Errorf("MCP = %+v", got)
			}
			if got := r.skills(); got.Reported {
				t.Errorf("Skills = %+v", got)
			}
			if _, err := r.act("x", api.MCPReconnect); !errors.Is(err, errNoTools) {
				t.Errorf("MCPAct: %v", err)
			}
			if err := r.setSkill("x", false); !errors.Is(err, errNoTools) {
				t.Errorf("SetSkill: %v", err)
			}
		})
	}
}

// Codex's list joins mcpServerStatus/list, every page, with config/read:
// whether a server is off, its target and its file come from the config, the
// sign-in from authStatus, and a server with no runtimeStatus is up when it
// has tools.
func TestCodexMCPList(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	r := newChat(t, providers.Codex)
	list := r.mcp()
	noSecrets(t, list)
	if !list.Reported || !slices.Equal(list.Actions, mcpActions) || len(list.Servers) != len(fakeCodexMCP) {
		t.Fatalf("list = %+v", list)
	}
	file := filepath.Join(r.home, ".codex", "config.toml")
	want := map[string]api.ChatMCPServer{
		"codex_apps": {Name: "codex_apps", Status: api.MCPConnected, Auth: api.MCPSignedIn, Tools: []api.ChatMCPTool{{Name: "a_tool"}, {Name: "b_tool", Description: "B"}}},
		"stub":       {Name: "stub", Status: api.MCPDisabled, Scope: "user", Config: file, Transport: "stdio", Target: "python3 stub.py"},
		"linear": {Name: "linear", Status: api.MCPNeedsAuth, Scope: "user", Config: file, Transport: "http", Target: "https://mcp.linear.app/mcp",
			Error: "login needed", Auth: api.MCPNotSignedIn},
		"lich": {Name: "lich", Status: api.MCPConnected, Transport: "stdio", Target: "/bin/lich mcp", Version: "9", Fixed: true,
			Tools: []api.ChatMCPTool{{Name: "list_sessions"}}},
	}
	for name, w := range want {
		if got := serverNamed(t, list, name); !jsonEqual(got, w) {
			t.Errorf("%s = %+v, want %+v", name, got, w)
		}
	}
}

// Turning a Codex server off writes its enabled key and reloads; signing in
// answers the authorization page.
func TestCodexMCPAct(t *testing.T) {
	r := newChat(t, providers.Codex)
	if _, err := r.act("lich", api.MCPDisable); !errors.Is(err, errFixedMCP) {
		t.Errorf("disable lich: %v", err)
	}
	if _, err := r.act("stub", api.MCPEnable); err != nil ||
		!r.logged(`"config/value/write"`, `"keyPath":"mcp_servers.stub.enabled"`, `"value":true`, `"mergeStrategy":"upsert"`) || !r.logged(`"config/mcpServer/reload"`) {
		t.Errorf("enable stub: %v, log %s", err, r.fakeLog())
	}
	if got, err := r.act("linear", api.MCPSignIn); err != nil || got.AuthURL != "https://mcp.linear.app/authorize?x=1" || !r.logged(`"mcpServer/oauth/login"`, `"name":"linear"`) {
		t.Errorf("sign in linear = %+v, %v", got, err)
	}
	if _, err := r.act("linear", api.MCPReconnect); err != nil {
		t.Errorf("reconnect: %v", err)
	}
}

// Codex's skills are skills/list's, one per name; turning one off writes its
// path and lists them again, which the prompt's $ commands follow.
func TestCodexSkills(t *testing.T) {
	r := newChat(t, providers.Codex)
	want := []api.ChatSkill{
		{Name: "deploy", Description: "Ship it", Source: "user", State: api.SkillOn},
		{Name: "off", Description: "Turned off", Source: "user", State: api.SkillOff},
	}
	if got := r.skills(); !got.Reported || !slices.Equal(got.Skills, want) {
		t.Errorf("skills = %+v", got)
	}
	r.waitCommands([]api.ChatCommand{{Text: "$deploy", Description: "Ship it", Skill: true}})
	if err := r.setSkill("deploy", false); err != nil || !r.logged(`"skills/config/write"`, `"path":"/skills/deploy/SKILL.md"`, `"enabled":false`) {
		t.Fatalf("SetSkill: %v, log %s", err, r.fakeLog())
	}
	r.waitCommands([]api.ChatCommand{{Text: "$deploy", Description: "A repo's deploy", Skill: true}})
	if err := r.setSkill("nope", true); err == nil {
		t.Error("a skill the chat does not have was written")
	}
}
