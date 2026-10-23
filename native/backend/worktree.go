package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type Worktree = api.Worktree

type Branches = api.Branches

type BaseFreshness = api.BaseFreshness

type Issue = api.Issue

func (c *Client) ListBranches(ctx context.Context, path string) (Branches, error) {
	return callMethod(ctx, c, api.ProjectListBranches, api.PathOptions{Path: path})
}

// BaseFreshness fetches branch's upstream first, so it waits on the network
// for up to the backend's fetch budget.
func (c *Client) BaseFreshness(ctx context.Context, path, branch string) (BaseFreshness, error) {
	return callMethod(ctx, c, api.ProjectBaseFreshness, api.PathBranchOptions{Path: path, Branch: branch})
}

// CreateWorktree checks a new branch name out off base; a remote base is
// fetched first.
func (c *Client) CreateWorktree(ctx context.Context, projectPath, projectID, name, base string, baseIsRemote bool) (*Worktree, error) {
	return callMethod(ctx, c, api.ProjectCreateWorktree, api.CreateWorktreeOptions{ProjectPath: projectPath, ProjectID: projectID, Name: name, Base: base, BaseIsRemote: baseIsRemote})
}

// CarryUncommitted copies src's uncommitted work onto dst.
func (c *Client) CarryUncommitted(ctx context.Context, src, dst string) error {
	_, err := callMethod(ctx, c, api.ProjectCarryUncommitted, api.CarryUncommittedOptions{Src: src, Dst: dst})
	return err
}

// Issue resolves a GitHub issue through gh.
func (c *Client) Issue(ctx context.Context, path string, number int) (Issue, error) {
	return callMethod(ctx, c, api.ProjectIssue, api.NumberOptions{Path: path, Number: number})
}

// ReopenWorktreeSession brings back the session parked on path under
// newSessionID, nil when none is parked there.
func (c *Client) ReopenWorktreeSession(ctx context.Context, projectID, path, newSessionID string) (*Session, error) {
	return callMethod(ctx, c, api.StoreReopenWorktreeSession, api.ReopenWorktreeSessionOptions{ProjectID: projectID, Path: path, NewSessionID: newSessionID})
}

// AddSessionWithSandbox is AddSession with the sandbox answer the session was
// opened with: "on", "off", or "" to leave it to the provider's rung.
func (c *Client) AddSessionWithSandbox(ctx context.Context, projectID, sessionID, label, kind, path string, nextSeq int, sandbox string) error {
	_, err := callMethod(ctx, c, api.StoreAddSession, api.AddSessionOptions{ProjectID: projectID, SessionID: sessionID, Label: label, Kind: kind, Path: path, NextSeq: nextSeq, Sandbox: sandbox})
	return err
}

// Ready reports whether session id sits at a prompt that can take input.
func (c *Client) Ready(ctx context.Context, id string) (bool, error) {
	return callMethod(ctx, c, api.TerminalReady, api.SessionIDOptions{ID: id})
}
