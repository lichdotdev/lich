package lichclient

import "context"

// DetectedProvider mirrors providers.Detected (internal/providers), trimmed to
// what the launch menu reads.
type DetectedProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
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
