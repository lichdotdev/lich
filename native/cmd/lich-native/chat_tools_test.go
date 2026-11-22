package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

var toolsServers = api.ChatMCPList{
	Reported: true,
	Actions:  []string{api.MCPEnable, api.MCPDisable, api.MCPReconnect, api.MCPSignIn},
	Servers: []api.ChatMCPServer{
		{Name: "lich", Status: api.MCPConnected, Scope: "dynamic", Fixed: true, Tools: []api.ChatMCPTool{{Name: "open_session"}}},
		{Name: "ai-memory", Status: api.MCPConnected, Scope: "user", Config: "~/.claude.json", Target: "http://127.0.0.1:49374/mcp", Tools: []api.ChatMCPTool{{Name: "memory_query"}, {Name: "memory_recent"}}},
		{Name: "drive", Status: api.MCPNeedsAuth, Scope: "claudeai", Auth: api.MCPNotSignedIn},
	},
}

var toolsSkills = api.ChatSkillList{
	Reported: true,
	Skills: []api.ChatSkill{
		{Name: "lich-i18n", Source: "project", Tokens: 130, State: api.SkillOn},
		{Name: "ponytail-audit", Source: "plugin", Tokens: 120, State: api.SkillOn, Locked: "plugin"},
	},
}

// toolsFrames is session s1 drawn as a Claude Code chat with no turn running,
// its backend answering the servers and skills, command typed and sent.
func toolsFrames(t *testing.T, command string) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	f.answers = map[string]any{api.ChatMCP.Name: toolsServers, api.ChatSkills.Name: toolsSkills}
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}})
	r.settle(t)
	p := u.chats.pane("s1")
	typeIn(r, p, command)
	// the first Enter completes the command in the slash list, the second sends it
	r.press(key.NameReturn)
	r.press(key.NameReturn)
	toolsSettle(r)
	return u, r, f, p
}

// toolsSettle lets the read land and the frames draw it.
func toolsSettle(r *frameRig) {
	time.Sleep(50 * time.Millisecond)
	r.frames(3)
}

// /mcp opens the servers over the box instead of reaching the agent.
func TestMCPOpensTheServersInsteadOfSending(t *testing.T) {
	u, r, f, p := toolsFrames(t, "/mcp")
	if !r.showsAll("MCP servers", "ai-memory", "2 tools", "needs sign-in", "~/.claude.json") {
		t.Errorf("the servers are not drawn:\n%s", r.drawn())
	}
	if u.model.chat.isRunning("s1") || f.callsOf(api.ChatPrompt.Name) != 0 {
		t.Error("/mcp went to the agent")
	}
	if p.editor.Len() != 0 {
		t.Errorf("the box kept %q", p.editor.Text())
	}
}

// A server opens on a click with what can be done to it; Escape goes back to
// the list and then shuts the panel.
func TestAServerOpensAndEscapeGoesBack(t *testing.T) {
	_, r, f, p := toolsFrames(t, "/mcp")
	r.click(t, r.at(t, "ai-memory"))
	if !r.showsAll("Reconnect", "Turn off in this project", "View tools", "http://127.0.0.1:49374/mcp") {
		t.Fatalf("the server's detail is not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "Reconnect"))
	if body := f.waitCall(t, api.ChatMCPAct.Name); !strings.Contains(body, `"server":"ai-memory"`) || !strings.Contains(body, `"action":"reconnect"`) {
		t.Errorf("Reconnect asked %s", body)
	}
	r.press(key.NameEscape)
	if !r.shows("MCP servers") {
		t.Errorf("Escape did not go back to the list:\n%s", r.drawn())
	}
	r.press(key.NameEscape)
	if p.tools.opened() || r.shows("MCP servers") {
		t.Errorf("a second Escape left the panel open:\n%s", r.drawn())
	}
}

// lich's own server is never offered to turn off: every later session in the
// project would lose its tools.
func TestLichsOwnServerCannotBeTurnedOff(t *testing.T) {
	_, r, _, _ := toolsFrames(t, "/mcp")
	r.click(t, r.at(t, "lich"))
	if !r.shows("Reconnect") || r.shows("Turn off in this project") {
		t.Errorf("lich's server offers to turn off:\n%s", r.drawn())
	}
}

// Signing in opens the page the provider answers.
func TestSigningInOpensThePage(t *testing.T) {
	_, r, f, _ := toolsFrames(t, "/mcp")
	f.mu.Lock()
	f.answers[api.ChatMCPAct.Name] = api.ChatMCPActResult{AuthURL: "https://claude.ai/connect"}
	f.mu.Unlock()
	r.click(t, r.at(t, "drive"))
	r.click(t, r.at(t, "Sign in"))
	if body := f.waitCall(t, api.SystemOpenExternal.Name); !strings.Contains(body, "https://claude.ai/connect") {
		t.Errorf("opened %s", body)
	}
}

// /skills lists the skills; a click turns one off, a locked one stays.
func TestSkillsTurnOnAndOff(t *testing.T) {
	_, r, f, _ := toolsFrames(t, "/skills")
	if !r.showsAll("Skills", "lich-i18n", "project · ~130 tokens", "locked by its plugin · plugin · ~120 tokens", "Plugin skills are managed with /plugin.") {
		t.Fatalf("the skills are not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "ponytail-audit"))
	time.Sleep(20 * time.Millisecond)
	if n := f.callsOf(api.ChatSetSkill.Name); n != 0 {
		t.Errorf("a locked skill was turned (%d calls)", n)
	}
	r.click(t, r.at(t, "lich-i18n"))
	if body := f.waitCall(t, api.ChatSetSkill.Name); !strings.Contains(body, `"name":"lich-i18n"`) || !strings.Contains(body, `"enabled":false`) {
		t.Errorf("turned %s", body)
	}
}

// /mcp typed while the agent works opens the panel too, not the queue.
func TestMCPDuringATurnOpensThePanel(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	f.answers = map[string]any{api.ChatMCP.Name: toolsServers}
	u.model.client = u.client
	u.model.chat.history("s1", nil)
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frame()
	p := u.chats.pane("s1")
	typeIn(r, p, "/mcp")
	r.press(key.NameReturn)
	r.press(key.NameReturn)
	toolsSettle(r)
	if q := u.model.chat.queued("s1"); len(q) != 0 {
		t.Errorf("/mcp was queued: %q", q)
	}
	if !p.tools.opened() {
		t.Error("/mcp during a turn did not open the panel")
	}
}

// The slash list offers /mcp, /skills, /plugin, /permissions and /hooks only
// where the provider reports what they open.
func TestTheSlashListOffersTheToolsWhereTheyOpen(t *testing.T) {
	texts := func(kind string) []string {
		var out []string
		for _, c := range slashCommands([]backend.ChatCommand{{Text: "/mcp"}, {Text: "/review"}}, kind) {
			out = append(out, c.Text)
		}
		return out
	}
	if got := texts("claude"); !slices.Equal(got, []string{"/new", "/mcp", "/skills", "/plugin", "/permissions", "/hooks", "/review"}) {
		t.Errorf("claude lists %q", got)
	}
	if got := texts("opencode"); !slices.Equal(got, []string{"/new", "/mcp", "/review"}) {
		t.Errorf("opencode lists %q, want its own /mcp", got)
	}
}
