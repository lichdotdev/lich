package lichclient

import "context"

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
	return c.Call(ctx, "system.OpenExternal", nil, url)
}

// OpenFolderInEditor opens dir in the user's editor. A GUI editor is launched
// detached and the reply is ""; a terminal editor comes back as the command
// line to run in a shell at dir.
func (c *Client) OpenFolderInEditor(ctx context.Context, dir string) (string, error) {
	var command string
	return command, c.Call(ctx, "system.OpenFolderInEditor", &command, dir)
}

// OpenFolder opens dir in the file manager.
func (c *Client) OpenFolder(ctx context.Context, dir string) error {
	return c.Call(ctx, "system.OpenFolder", nil, dir)
}

// Diagnostics mirrors system.Diagnostics: what a bug report needs.
type Diagnostics struct {
	Version  string `json:"version"`
	Platform string `json:"platform"`
	LogPath  string `json:"logPath"` // "" when lich logs to stderr only
}

func (c *Client) Diagnostics(ctx context.Context) (Diagnostics, error) {
	var d Diagnostics
	return d, c.Call(ctx, "system.Diagnostics", &d)
}

// RevealLog opens the log's directory in the file manager; it fails when
// there is no log file.
func (c *Client) RevealLog(ctx context.Context) error {
	return c.Call(ctx, "system.RevealLog", nil)
}

// TakeUncleanExit reports whether lich's run before this one ended without
// quitting cleanly. The backend clears the flag on the first ask, so only
// the first window to ask is told.
func (c *Client) TakeUncleanExit(ctx context.Context) (bool, error) {
	var unclean bool
	return unclean, c.Call(ctx, "system.TakeUncleanExit", &unclean)
}
