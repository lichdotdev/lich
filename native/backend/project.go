package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// BaseStatus mirrors project.BaseStatus: where a checkout stands against the
// branch it merges into.
type BaseStatus struct {
	Base      string   `json:"base"`
	Behind    int      `json:"behind"`
	Conflicts []string `json:"conflicts"`
}

// PullRequest mirrors project.PullRequest.
type PullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

// BaseStatus returns where path stands against its base, nil when it has no
// base to stand against (the base branch itself, no remote).
func (c *Client) BaseStatus(ctx context.Context, path string) (*BaseStatus, error) {
	var b *BaseStatus
	return b, c.Call(ctx, "project.BaseStatus", &b, path)
}

// PullRequest returns the open pull request of path's branch through gh, nil
// when there is none or gh cannot answer. Each call is a network round trip.
func (c *Client) PullRequest(ctx context.Context, path string) (*PullRequest, error) {
	var pr *PullRequest
	return pr, c.Call(ctx, "project.PullRequest", &pr, path)
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
