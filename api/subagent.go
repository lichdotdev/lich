package api

var (
	// SubagentVetoed is every call /subagent answered deny that a chat may still
	// draw, for a window that attaches after they were made.
	SubagentVetoed = define[None, []SubagentVeto]("subagent.Vetoed")
)

// The calls a /subagent request is (docs/hooks/subagent-cards.md).
const (
	SubagentSpawn = "spawn"
	SubagentWait  = "wait"
)

// The decisions a /subagent answer carries.
const (
	SubagentAllow = "allow"
	SubagentDeny  = "deny"
)

// SubagentRequest is one /subagent call: the provider's own subagent tool
// asking lich first (docs/hooks/subagent-cards.md, Request).
type SubagentRequest struct {
	SessionID         string         `json:"session_id"`
	ProviderSessionID string         `json:"provider_session_id"`
	Nested            bool           `json:"nested"`
	Call              string         `json:"call"`
	Tool              string         `json:"tool"`
	ToolUseID         string         `json:"tool_use_id"`
	Tasks             []SubagentTask `json:"tasks"`
	Model             string         `json:"model"`
	Sealed            bool           `json:"sealed"`
	Isolation         string         `json:"isolation"`
	Background        bool           `json:"background"`
}

// SubagentTask is one subagent a spawn starts.
type SubagentTask struct {
	Title  string `json:"title"`
	Prompt string `json:"prompt"`
	Type   string `json:"type"`
}

// SubagentAnswer is lich's decision: deny with the reason the model reads and
// what it opened, or allow.
type SubagentAnswer struct {
	Decision string           `json:"decision"`
	Reason   string           `json:"reason,omitempty"`
	Sessions []SubagentOpened `json:"sessions,omitempty"`
}

// SubagentOpened is one session a deny opened.
type SubagentOpened struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Ticket string `json:"ticket"`
}

// SubagentVeto is a tool call /subagent answered deny once its answer was
// written: the chat of SessionID draws that call as opened in Sessions rather
// than as a failed call.
type SubagentVeto struct {
	SessionID string           `json:"sessionId"`
	ToolUseID string           `json:"toolUseId"`
	Sessions  []SubagentOpened `json:"sessions"`
}

// CodexHookUntrusted names the Codex session whose chat found lich-plugin's
// PreToolUse hook not trusted (modified or untrusted), so Codex runs without
// it and its subagents stay native.
type CodexHookUntrusted struct {
	SessionID string `json:"sessionId"`
	Trust     string `json:"trust"`
}
