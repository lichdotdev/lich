package backend

import "context"

// DetectedProvider mirrors providers.Detected (internal/providers), trimmed to
// what the launch menu and Settings › Providers read. Binary is the executable
// a spawn asks $PATH for, Source where it was found ("path", "setting", or ""
// for none) and Docs the page that documents installing it.
type DetectedProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Binary    string `json:"binary"`
	Installed bool   `json:"installed"`
	Source    string `json:"source"`
	Docs      string `json:"docs"`
}

// ReorderSessions stores a project's full card order. The backend writes a
// position per id, so ids must name every session of the project.
func (c *Client) ReorderSessions(ctx context.Context, projectID string, ids []string) error {
	return c.Call(ctx, "store.ReorderSessions", nil, projectID, ids)
}

// SetSessionFolder files a session under folder, "" to take it out of one.
func (c *Client) SetSessionFolder(ctx context.Context, sessionID, folder string) error {
	return c.Call(ctx, "store.SetSessionFolder", nil, sessionID, folder)
}

// RenameFolder renames a project's folder on every session filed under it,
// parked ones included, and ungroups it when to is "". It fails for an empty
// from, and returns the ids it rewrote.
func (c *Client) RenameFolder(ctx context.Context, projectID, from, to string) ([]string, error) {
	var moved []string
	return moved, c.Call(ctx, "store.RenameFolder", &moved, projectID, from, to)
}

// AddSessionFrom adds a session opened from the session originID, which the
// row remembers along with originLabel, what it was called then.
func (c *Client) AddSessionFrom(ctx context.Context, projectID, sessionID, label, kind, path string, nextSeq int, originID, originLabel, folder string) error {
	return c.Call(ctx, "store.AddSessionFrom", nil, projectID, sessionID, label, kind, path, nextSeq, originID, originLabel, folder)
}

// MarkForked records that the session was born branching its origin's
// conversation.
func (c *Client) MarkForked(ctx context.Context, sessionID string) error {
	return c.Call(ctx, "store.MarkForked", nil, sessionID)
}

// AddSessionInFolder is AddSession for a session opened into folder, written
// with the row. The sandbox answer is always "": the native window has no
// confinement step, so the provider's rung decides.
func (c *Client) AddSessionInFolder(ctx context.Context, projectID, sessionID, label, kind, path string, nextSeq int, folder string) error {
	return c.Call(ctx, "store.AddSessionInFolder", nil, projectID, sessionID, label, kind, path, nextSeq, "", folder)
}

// DetectProviders probes which provider CLIs this machine has, nil when the
// backend answers none.
func (c *Client) DetectProviders(ctx context.Context) ([]DetectedProvider, error) {
	var found []DetectedProvider
	return found, c.Call(ctx, "providers.Detect", &found)
}
