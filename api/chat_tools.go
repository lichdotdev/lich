package api

// A chat session's MCP servers and skills, as its provider reports them, for
// the window's /mcp and /skills panels: the screens Claude Code's and Codex's
// TUIs open for those commands, which their structured modes do not draw.

var (
	// ChatMCP is a chat session's MCP servers as its provider reports them
	// now. A provider that reports none leaves Reported false.
	ChatMCP = define[SessionIDOptions, ChatMCPList]("chat.MCP")
	// ChatMCPAct acts on one of them: turns it on or off in the session's
	// project, reconnects it, or signs in to it. Signing in answers the page
	// the person opens to finish it.
	ChatMCPAct = define[ChatMCPActOptions, ChatMCPActResult]("chat.MCPAct")
	// ChatSkills is a chat session's skills as its provider reports them now.
	// A provider that reports none leaves Reported false.
	ChatSkills = define[SessionIDOptions, ChatSkillList]("chat.Skills")
	// ChatSetSkill turns one of them on or off.
	ChatSetSkill = define[ChatSetSkillOptions, None]("chat.SetSkill")
)

// MCP server statuses, as ChatMCPServer.Status names them.
const (
	MCPConnected = "connected"
	MCPPending   = "pending"
	MCPFailed    = "failed"
	MCPNeedsAuth = "needs-auth"
	MCPDisabled  = "disabled"
)

// MCP sign-in states, as ChatMCPServer.Auth names them; "" when the provider
// does not say.
const (
	MCPSignedIn    = "signed-in"
	MCPNotSignedIn = "not-signed-in"
)

// ChatMCPAct actions.
const (
	MCPEnable    = "enable"
	MCPDisable   = "disable"
	MCPReconnect = "reconnect"
	MCPSignIn    = "sign-in"
)

// ChatMCPList is a chat session's MCP servers. Actions are the ChatMCPAct
// actions its provider takes.
type ChatMCPList struct {
	Reported bool            `json:"reported"`
	Servers  []ChatMCPServer `json:"servers,omitempty"`
	Actions  []string        `json:"actions,omitempty"`
}

// ChatMCPServer is one MCP server of a chat session. Scope is where it is
// configured as the provider names it (Claude Code: user, project, local,
// claudeai, managed, plugin; Codex: user), Config the file that holds it,
// "" when lich cannot tell. Target is its URL or command, never its
// environment or headers. Fixed is set on a server the panel must not turn
// off: lich's own, which it adds to every session.
type ChatMCPServer struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Scope     string        `json:"scope,omitempty"`
	Config    string        `json:"config,omitempty"`
	Transport string        `json:"transport,omitempty"`
	Target    string        `json:"target,omitempty"`
	Version   string        `json:"version,omitempty"`
	Error     string        `json:"error,omitempty"`
	Auth      string        `json:"auth,omitempty"`
	Fixed     bool          `json:"fixed,omitempty"`
	Tools     []ChatMCPTool `json:"tools,omitempty"`
}

// ChatMCPTool is one tool an MCP server offers.
type ChatMCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ChatMCPActOptions names a ChatMCPAct action on server Server of session ID.
type ChatMCPActOptions struct {
	ID     string `json:"id"`
	Server string `json:"server"`
	Action string `json:"action"`
}

// ChatMCPActResult is what an action leaves the person to do: AuthURL is the
// page that finishes a sign-in, "" when none is needed.
type ChatMCPActResult struct {
	AuthURL string `json:"authUrl,omitempty"`
}

// Skill states, as ChatSkill.State names them. Claude Code reports all four;
// Codex only on and off.
const (
	SkillOn                = "on"
	SkillOff               = "off"
	SkillNameOnly          = "name-only"
	SkillUserInvocableOnly = "user-invocable-only"
)

// ChatSkillList is a chat session's skills.
type ChatSkillList struct {
	Reported bool        `json:"reported"`
	Skills   []ChatSkill `json:"skills,omitempty"`
}

// ChatSkill is one skill of a chat session. Source is where it comes from as
// the provider names it (project, user, plugin, system...), Tokens what it
// costs the context, 0 when not reported, and Locked why it cannot be turned
// on or off (Claude Code: author, plugin, policy, flag, reserved-name), ""
// when it can.
type ChatSkill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Tokens      int    `json:"tokens,omitempty"`
	State       string `json:"state"`
	Locked      string `json:"locked,omitempty"`
}

// ChatSetSkillOptions turns skill Name of session ID on or off.
type ChatSetSkillOptions struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}
