package lichclient

import "context"

// CloseSession parks a session: its row leaves the open list but stays for a
// later reopen, and activeID becomes the project's active session ("" for
// none). It does not stop the session's process; CloseTerminal does.
func (c *Client) CloseSession(ctx context.Context, projectID, sessionID, activeID string) error {
	return c.Call(ctx, "store.CloseSession", nil, projectID, sessionID, activeID)
}

// CloseTerminal stops a session's process and drops its PTY.
func (c *Client) CloseTerminal(ctx context.Context, id string) error {
	return c.Call(ctx, "terminal.Close", nil, id)
}

// SetSessionPinned pins a session to the head of the sidebar, or unpins it.
func (c *Client) SetSessionPinned(ctx context.Context, sessionID string, pinned bool) error {
	return c.Call(ctx, "store.SetSessionPinned", nil, sessionID, pinned)
}

// SetSessionUnread records whether a session's finished turn is still unread,
// the one mark a window restart brings back (Session.Unread).
func (c *Client) SetSessionUnread(ctx context.Context, sessionID string, unread bool) error {
	return c.Call(ctx, "store.SetSessionUnread", nil, sessionID, unread)
}

// RenameSession gives a session the label the user typed.
func (c *Client) RenameSession(ctx context.Context, sessionID, label string) error {
	return c.Call(ctx, "store.RenameSession", nil, sessionID, label)
}

// SetSessionColor tints a session's card, "" for the theme's.
func (c *Client) SetSessionColor(ctx context.Context, sessionID, color string) error {
	return c.Call(ctx, "store.SetSessionColor", nil, sessionID, color)
}

// Live reports whether a session has a process running right now.
func (c *Client) Live(ctx context.Context, id string) (bool, error) {
	var live bool
	return live, c.Call(ctx, "terminal.Live", &live, id)
}

// WorkdirMissing reports whether a session cannot start in cwd because the
// directory is provably gone; false for every uncertainty.
func (c *Client) WorkdirMissing(ctx context.Context, cwd string) (bool, error) {
	var missing bool
	return missing, c.Call(ctx, "terminal.WorkdirMissing", &missing, cwd)
}

// ResumeAvailable reports whether the provider conversation providerSessionID
// can still be reopened from cwd.
func (c *Client) ResumeAvailable(ctx context.Context, kind, providerSessionID, cwd string) (bool, error) {
	var available bool
	return available, c.Call(ctx, "terminal.ResumeAvailable", &available, kind, providerSessionID, cwd)
}
