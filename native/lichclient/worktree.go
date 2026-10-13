package lichclient

import "context"

// Worktree mirrors project.Worktree.
type Worktree struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Reused bool   `json:"reused,omitempty"`
}

// Branches mirrors project.Branches: what a new worktree can start from.
type Branches struct {
	Local     []string   `json:"local"`
	Remote    []string   `json:"remote"` // "origin/main" form
	Worktrees []Worktree `json:"worktrees"`
}

// BaseFreshness mirrors project.BaseFreshness.
type BaseFreshness struct {
	Upstream   string `json:"upstream"`
	Ahead      int    `json:"ahead"`
	Behind     int    `json:"behind"`
	FetchError string `json:"fetchError,omitempty"`
}

// Issue mirrors project.Issue.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}

func (c *Client) ListBranches(ctx context.Context, path string) (Branches, error) {
	var b Branches
	return b, c.Call(ctx, "project.ListBranches", &b, path)
}

// BaseFreshness fetches branch's upstream first, so it waits on the network
// for up to the backend's fetch budget.
func (c *Client) BaseFreshness(ctx context.Context, path, branch string) (BaseFreshness, error) {
	var f BaseFreshness
	return f, c.Call(ctx, "project.BaseFreshness", &f, path, branch)
}

// CreateWorktree checks a new branch name out off base; a remote base is
// fetched first.
func (c *Client) CreateWorktree(ctx context.Context, projectPath, projectID, name, base string, baseIsRemote bool) (*Worktree, error) {
	var wt *Worktree
	return wt, c.Call(ctx, "project.CreateWorktree", &wt, projectPath, projectID, name, base, baseIsRemote)
}

// CarryUncommitted copies src's uncommitted work onto dst.
func (c *Client) CarryUncommitted(ctx context.Context, src, dst string) error {
	return c.Call(ctx, "project.CarryUncommitted", nil, src, dst)
}

// Issue resolves a GitHub issue through gh.
func (c *Client) Issue(ctx context.Context, path string, number int) (Issue, error) {
	var i Issue
	return i, c.Call(ctx, "project.Issue", &i, path, number)
}

// ReopenWorktreeSession brings back the session parked on path under
// newSessionID, nil when none is parked there.
func (c *Client) ReopenWorktreeSession(ctx context.Context, projectID, path, newSessionID string) (*Session, error) {
	var s *Session
	return s, c.Call(ctx, "store.ReopenWorktreeSession", &s, projectID, path, newSessionID)
}

// AddSessionWithSandbox is AddSession with the sandbox answer the session was
// opened with: "on", "off", or "" to leave it to the provider's rung.
func (c *Client) AddSessionWithSandbox(ctx context.Context, projectID, sessionID, label, kind, path string, nextSeq int, sandbox string) error {
	return c.Call(ctx, "store.AddSession", nil, projectID, sessionID, label, kind, path, nextSeq, sandbox)
}

// Ready reports whether session id sits at a prompt that can take input.
func (c *Client) Ready(ctx context.Context, id string) (bool, error) {
	var ok bool
	return ok, c.Call(ctx, "terminal.Ready", &ok, id)
}
