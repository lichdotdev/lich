package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type (
	GitRef          = api.GitRef
	GitCommit       = api.GitCommit
	GitLogOptions   = api.GitLogOptions
	GitLogPage      = api.GitLogPage
	GitRefs         = api.GitRefs
	GitCommitDetail = api.GitCommitDetail
	GitCommitFile   = api.GitCommitFile
)

const (
	GitRefLocal  = api.GitRefLocal
	GitRefRemote = api.GitRefRemote
	GitRefTag    = api.GitRefTag
)

func (c *Client) GitLog(ctx context.Context, opts GitLogOptions) (GitLogPage, error) {
	return callMethod(ctx, c, api.ProjectGitLog, opts)
}

func (c *Client) GitRefs(ctx context.Context, path string) (GitRefs, error) {
	return callMethod(ctx, c, api.ProjectGitRefs, api.PathOptions{Path: path})
}

func (c *Client) GitCommit(ctx context.Context, path, hash string) (GitCommitDetail, error) {
	return callMethod(ctx, c, api.ProjectGitCommit, api.GitCommitOptions{Path: path, Hash: hash})
}

func (c *Client) GitCommitDiff(ctx context.Context, path, hash, file, oldFile string) (string, error) {
	return callMethod(ctx, c, api.ProjectGitCommitDiff, api.GitCommitDiffOptions{Path: path, Hash: hash, File: file, OldFile: oldFile})
}

func (c *Client) GitCheckout(ctx context.Context, path, branch string, remote bool) error {
	_, err := callMethod(ctx, c, api.ProjectGitCheckout, api.GitCheckoutOptions{Path: path, Branch: branch, Remote: remote})
	return err
}

func (c *Client) GitCreateBranch(ctx context.Context, path, name, from string, upstream api.GitUpstream) error {
	_, err := callMethod(ctx, c, api.ProjectGitCreateBranch, api.GitCreateBranchOptions{Path: path, Name: name, From: from, Upstream: upstream})
	return err
}

func (c *Client) GitRenameBranch(ctx context.Context, path, name, newName string) error {
	_, err := callMethod(ctx, c, api.ProjectGitRenameBranch, api.GitRenameBranchOptions{Path: path, Name: name, NewName: newName})
	return err
}

// GitBranchOnly counts the commits only branch name reaches.
func (c *Client) GitBranchOnly(ctx context.Context, path, name string) (int, error) {
	return callMethod(ctx, c, api.ProjectGitBranchOnly, api.GitBranchOptions{Path: path, Name: name})
}

// GitDeleteBranch returns the commit and upstream the branch had, to make it again.
func (c *Client) GitDeleteBranch(ctx context.Context, path, name string) (api.GitDeletedBranch, error) {
	return callMethod(ctx, c, api.ProjectGitDeleteBranch, api.GitBranchOptions{Path: path, Name: name})
}
