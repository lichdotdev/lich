package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type BaseStatus = api.BaseStatus

type PullRequest = api.PullRequest

type BranchPullRequest = api.BranchPullRequest

// BaseStatus returns where path stands against its base, nil when it has no
// base to stand against (the base branch itself, no remote).
func (c *Client) BaseStatus(ctx context.Context, path string) (*BaseStatus, error) {
	return callMethod(ctx, c, api.ProjectBaseStatus, api.PathOptions{Path: path})
}

// RecheckPullRequest asks the backend to read the pull request of path's
// branch again; the answer arrives as api.BranchPullRequestEvent.
func (c *Client) RecheckPullRequest(ctx context.Context, path string) error {
	_, err := callMethod(ctx, c, api.ProjectRecheckPullRequest, api.PathOptions{Path: path})
	return err
}

// OpenExternal opens url in the user's browser.
func (c *Client) OpenExternal(ctx context.Context, url string) error {
	_, err := callMethod(ctx, c, api.SystemOpenExternal, api.OpenExternalOptions{URL: url})
	return err
}

// OpenFolderInEditor opens dir in the user's editor. A GUI editor is launched
// detached and the reply is ""; a terminal editor comes back as the command
// line to run in a shell at dir.
func (c *Client) OpenFolderInEditor(ctx context.Context, dir string) (string, error) {
	return callMethod(ctx, c, api.SystemOpenFolderInEditor, api.OpenFolderOptions{Dir: dir})
}

// OpenFolder opens dir in the file manager.
func (c *Client) OpenFolder(ctx context.Context, dir string) error {
	_, err := callMethod(ctx, c, api.SystemOpenFolder, api.OpenFolderOptions{Dir: dir})
	return err
}

type Diagnostics = api.Diagnostics

func (c *Client) Diagnostics(ctx context.Context) (Diagnostics, error) {
	return callMethod(ctx, c, api.SystemDiagnostics, api.None{})
}

// RevealLog opens the log's directory in the file manager; it fails when
// there is no log file.
func (c *Client) RevealLog(ctx context.Context) error {
	_, err := callMethod(ctx, c, api.SystemRevealLog, api.None{})
	return err
}

// TakeUncleanExit reports whether lich's run before this one ended without
// quitting cleanly. The backend clears the flag on the first ask, so only
// the first window to ask is told.
func (c *Client) TakeUncleanExit(ctx context.Context) (bool, error) {
	return callMethod(ctx, c, api.SystemTakeUncleanExit, api.None{})
}

// OpenFile opens rel, a file of the checkout at dir, in the application the
// desktop opens its type with, never in the user's editor.
func (c *Client) OpenFile(ctx context.Context, dir, rel string) error {
	_, err := callMethod(ctx, c, api.SystemOpenFile, api.OpenFileOptions{Dir: dir, Rel: rel})
	return err
}

// SandboxBackend names what confines a sandboxed session on this machine, ""
// when nothing can.
func (c *Client) SandboxBackend(ctx context.Context) (string, error) {
	return callMethod(ctx, c, api.SystemSandboxBackend, api.None{})
}

// SSHAgentKeys lists the keys the user's ssh-agent holds, for a sandboxed
// session that may use them.
func (c *Client) SSHAgentKeys(ctx context.Context) ([]string, error) {
	return callMethod(ctx, c, api.SystemSSHAgentKeys, api.None{})
}

// FileListing is a checkout's files as the code tab lists them.
type FileListing = api.FileListing

// Tree lists the files of the checkout at path.
func (c *Client) Tree(ctx context.Context, path string) (FileListing, error) {
	return callMethod(ctx, c, api.ProjectTree, api.PathOptions{Path: path})
}

// SearchResult is a search of a checkout's files.
type SearchResult = api.SearchResult

// SearchMatch is one line a search matched.
type SearchMatch = api.SearchMatch

// Search finds query in the files of the checkout at path.
func (c *Client) Search(ctx context.Context, path, query string) (SearchResult, error) {
	return callMethod(ctx, c, api.ProjectSearch, api.SearchOptions{Path: path, Query: query})
}

// ReadFile reads file rel of the checkout at path from the working tree.
func (c *Client) ReadFile(ctx context.Context, path, rel string) (string, error) {
	return callMethod(ctx, c, api.ProjectReadFile, api.CheckoutFileOptions{Path: path, Rel: rel})
}

// ReadFileAt reads file rel of the checkout at path as it was at ref.
func (c *Client) ReadFileAt(ctx context.Context, path, rel, ref string) (string, error) {
	return callMethod(ctx, c, api.ProjectReadFileAt, api.ReadFileAtOptions{Path: path, Rel: rel, Ref: ref})
}

// DiscardFile throws away every uncommitted change to file rel.
func (c *Client) DiscardFile(ctx context.Context, path, rel string) error {
	_, err := callMethod(ctx, c, api.ProjectDiscardFile, api.CheckoutFileOptions{Path: path, Rel: rel})
	return err
}

// RestoreLines undoes the revert undo describes, the result RevertLines gave.
func (c *Client) RestoreLines(ctx context.Context, path, rel string, undo RevertResult) error {
	_, err := callMethod(ctx, c, api.ProjectRestoreLines, api.RestoreLinesOptions{Path: path, Rel: rel, Undo: undo})
	return err
}
