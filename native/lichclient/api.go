package lichclient

import (
	"context"
	"encoding/base64"
	"fmt"
)

// Project mirrors store.Project (internal/store/store.go), trimmed to what the
// native client reads.
type Project struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Path            string    `json:"path"`
	NextSeq         int       `json:"nextSeq"`
	ActiveSessionID string    `json:"activeSessionId"`
	DefaultProvider string    `json:"defaultProvider"`
	Sessions        []Session `json:"sessions"`
}

// Session mirrors store.Session, trimmed. Path is the checkout the session
// runs in; Kind is the provider (claude, codex, ...) or "shell".
type Session struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Folder string `json:"folder"`
	Pinned bool   `json:"pinned"`
	Unread bool   `json:"unread"`
	Color  string `json:"color"`
	// ProviderSessionID is the provider conversation the session last ran,
	// "" for none: what a restored card offers to resume.
	ProviderSessionID string `json:"providerSessionId"`
	// MCPServers are the servers the session's spawn found registered with
	// its provider, which divide an MCP tool's name into server and tool.
	MCPServers []string `json:"mcpServers"`
	// Profile is the Claude Code profile the session runs in (a slug), "" for
	// Personal and for every other provider.
	Profile string `json:"profile"`
	// Model is the model the session was born on, "" for one opened on its
	// provider's default.
	Model string `json:"model"`
	// OriginSessionID is the session this one came from, in any project, ""
	// for one nobody opened from another; OriginLabel is what that session
	// was called when it did.
	OriginSessionID string `json:"originSessionId"`
	OriginLabel     string `json:"originLabel"`
	// Forked is whether the session was born branching that session's
	// conversation, rather than delegated or opened as a subagent.
	Forked bool `json:"forked"`
}

// DiffStats mirrors project.DiffStats.
type DiffStats struct {
	Files   int    `json:"files"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Head    string `json:"head"`
	Branch  string `json:"branch"`
}

// LoadState returns every open project with its sessions.
func (c *Client) LoadState(ctx context.Context) ([]Project, error) {
	var ps []Project
	return ps, c.Call(ctx, "store.LoadState", &ps)
}

// AddProject opens path as a project under id.
func (c *Client) AddProject(ctx context.Context, id, name, path string) error {
	return c.Call(ctx, "store.AddProject", nil, id, name, path)
}

// AddSession records a new session row; Start then spawns its process.
func (c *Client) AddSession(ctx context.Context, projectID, sessionID, label, kind, path string, nextSeq int) error {
	return c.Call(ctx, "store.AddSession", nil, projectID, sessionID, label, kind, path, nextSeq, "")
}

// SetActiveSession records which session a project shows.
func (c *Client) SetActiveSession(ctx context.Context, projectID, sessionID string) error {
	return c.Call(ctx, "store.SetActiveSession", nil, projectID, sessionID)
}

// Replay returns the tail of a session's output, to write into a fresh
// terminal before live output is applied.
func (c *Client) Replay(ctx context.Context, id string) ([]byte, error) {
	var b64 string
	if err := c.Call(ctx, "terminal.Replay", &b64, id); err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("terminal.Replay: %w", err)
	}
	return data, nil
}

// SessionConversationEvent is the event a session's row naming another
// provider conversation emits (terminal conversationEventName).
const SessionConversationEvent = "session-conversation"

// ConversationReported is the session-conversation payload: ProviderSessionID
// is the conversation the session now names, "" for none.
type ConversationReported struct {
	ID                string `json:"id"`
	ProviderSessionID string `json:"providerSessionId"`
}

// Start spawns the session's process at cols x rows; a no-op when it already
// runs. resume is a provider conversation id to reopen, "" to start fresh.
func (c *Client) Start(ctx context.Context, id, projectID, cwd, kind, resume string, cols, rows int) error {
	return c.Call(ctx, "terminal.Start", nil, id, projectID, cwd, kind, resume, false, cols, rows)
}

// Fork spawns the session's process branching the provider conversation
// parent: the conversation is copied into one of its own, and the card that
// runs parent goes on untouched. A no-op when the session already runs.
func (c *Client) Fork(ctx context.Context, id, projectID, cwd, kind, parent string, cols, rows int) error {
	return c.Call(ctx, "terminal.Start", nil, id, projectID, cwd, kind, parent, true, cols, rows)
}

// Resize sets the session's PTY size.
func (c *Client) Resize(ctx context.Context, id string, cols, rows int) error {
	return c.Call(ctx, "terminal.Resize", nil, id, cols, rows)
}

// SetVisible tells the backend whether the session is on screen, which sets
// how long it batches output before sending.
func (c *Client) SetVisible(ctx context.Context, id string, visible bool) error {
	return c.Call(ctx, "terminal.SetVisible", nil, id, visible)
}

// Diff returns a checkout's branch and change counts.
func (c *Client) Diff(ctx context.Context, path string) (DiffStats, error) {
	var d DiffStats
	return d, c.Call(ctx, "project.Diff", &d, path)
}

// DiffText returns one unified diff of a checkout's staged, unstaged and
// untracked changes against HEAD.
func (c *Client) DiffText(ctx context.Context, path string) (string, error) {
	var s string
	return s, c.Call(ctx, "project.DiffText", &s, path)
}

// FileLines returns lines from..to (1-based, inclusive) of rel at ref, "" for
// the working tree. The backend caps one answer at 500 lines, so a caller
// wanting more asks again from where the answer ended.
func (c *Client) FileLines(ctx context.Context, path, rel, ref string, from, to int) ([]string, error) {
	var lines []string
	return lines, c.Call(ctx, "project.FileLines", &lines, path, rel, ref, from, to)
}

// RevertLine mirrors project.RevertLine: one changed line of a drawn diff,
// "old" for a deletion numbered in HEAD and "new" for an addition numbered in
// the working tree. Text proves the number still means the same line.
type RevertLine struct {
	Side string `json:"side"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// RevertResult mirrors project.RevertResult, what undoing a revert needs. Both
// patches are base64, opaque here: handed back to RestoreLines as they came.
type RevertResult struct {
	Patch      string `json:"patch"`
	IndexPatch string `json:"indexPatch"`
}

// RevertLines puts these changed lines of rel back to HEAD, in the index too
// when it holds them. It fails when the file moved on since the diff was
// drawn.
func (c *Client) RevertLines(ctx context.Context, path, rel string, lines []RevertLine) (RevertResult, error) {
	var r RevertResult
	return r, c.Call(ctx, "project.RevertLines", &r, path, rel, lines)
}

// GetSetting returns a setting's value, "" when unset. scope is a project id,
// "" for a global setting.
func (c *Client) GetSetting(ctx context.Context, key, scope string) (string, error) {
	var v string
	return v, c.Call(ctx, "store.GetSetting", &v, key, scope)
}

// HomeProject is the project rooted at the user's home directory, the one the
// windows pin as Home. It is not opened by asking.
func (c *Client) HomeProject(ctx context.Context) (Project, error) {
	var p Project
	return p, c.Call(ctx, "project.Home", &p)
}
