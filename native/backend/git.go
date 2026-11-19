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

func (c *Client) GitAuthors(ctx context.Context, path, ref string) ([]string, error) {
	return callMethod(ctx, c, api.ProjectGitAuthors, api.GitAuthorsOptions{Path: path, Ref: ref})
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

func (c *Client) GitMergeAbort(ctx context.Context, path string) error {
	_, err := callMethod(ctx, c, api.ProjectGitMergeAbort, api.PathOptions{Path: path})
	return err
}

func (c *Client) GitMergeCommit(ctx context.Context, path string) error {
	_, err := callMethod(ctx, c, api.ProjectGitMergeCommit, api.PathOptions{Path: path})
	return err
}

func (c *Client) GitMarkResolved(ctx context.Context, path, rel string) error {
	_, err := callMethod(ctx, c, api.ProjectGitMarkResolved, api.CheckoutFileOptions{Path: path, Rel: rel})
	return err
}

func (c *Client) GitMergePreview(ctx context.Context, path, ref string) (api.GitMergePreview, error) {
	return callMethod(ctx, c, api.ProjectGitMergePreview, api.GitMergeOptions{Path: path, Ref: ref})
}

func (c *Client) GitMerge(ctx context.Context, path, ref string) (api.GitMergeResult, error) {
	return callMethod(ctx, c, api.ProjectGitMerge, api.GitMergeOptions{Path: path, Ref: ref})
}

func (c *Client) GitMergeUndo(ctx context.Context, path, before, after string) error {
	_, err := callMethod(ctx, c, api.ProjectGitMergeUndo, api.GitMergeUndoOptions{Path: path, Before: before, After: after})
	return err
}

func (c *Client) GitConflict(ctx context.Context, path, rel string) (api.GitConflict, error) {
	return callMethod(ctx, c, api.ProjectGitConflict, api.CheckoutFileOptions{Path: path, Rel: rel})
}

// GitBlame says who last changed each line of file rel of the checkout at path.
func (c *Client) GitBlame(ctx context.Context, path, rel string) (api.GitBlame, error) {
	return callMethod(ctx, c, api.ProjectGitBlame, api.CheckoutFileOptions{Path: path, Rel: rel})
}

func (c *Client) GitResolveFile(ctx context.Context, path, rel, content string) error {
	_, err := callMethod(ctx, c, api.ProjectGitResolveFile, api.GitResolveFileOptions{Path: path, Rel: rel, Content: content})
	return err
}
