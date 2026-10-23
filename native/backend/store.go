package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type DetectedProvider = api.DetectedProvider

// ReorderSessions stores a project's full card order. The backend writes a
// position per id, so ids must name every session of the project.
func (c *Client) ReorderSessions(ctx context.Context, projectID string, ids []string) error {
	_, err := callMethod(ctx, c, api.StoreReorderSessions, api.ReorderSessionsOptions{ProjectID: projectID, IDs: ids})
	return err
}

// SetSessionFolder files a session under folder, "" to take it out of one.
func (c *Client) SetSessionFolder(ctx context.Context, sessionID, folder string) error {
	_, err := callMethod(ctx, c, api.StoreSetSessionFolder, api.SessionFolderOptions{SessionID: sessionID, Folder: folder})
	return err
}

// RenameFolder renames a project's folder on every session filed under it,
// parked ones included, and ungroups it when to is "". It fails for an empty
// from, and returns the ids it rewrote.
func (c *Client) RenameFolder(ctx context.Context, projectID, from, to string) ([]string, error) {
	return callMethod(ctx, c, api.StoreRenameFolder, api.RenameProjectFolderOptions{ProjectID: projectID, From: from, To: to})
}

// AddSessionFrom adds a session opened from the session originID, which the
// row remembers along with originLabel, what it was called then.
func (c *Client) AddSessionFrom(ctx context.Context, projectID, sessionID, label, kind, path string, nextSeq int, originID, originLabel, folder string) error {
	_, err := callMethod(ctx, c, api.StoreAddSessionFrom, api.AddSessionFromOptions{ProjectID: projectID, SessionID: sessionID, Label: label, Kind: kind, Path: path, NextSeq: nextSeq, OriginID: originID, OriginLabel: originLabel, Folder: folder})
	return err
}

// MarkForked records that the session was born branching its origin's
// conversation.
func (c *Client) MarkForked(ctx context.Context, sessionID string) error {
	_, err := callMethod(ctx, c, api.StoreMarkForked, api.SessionIDOptions{ID: sessionID})
	return err
}

// AddSessionInFolder is AddSession for a session opened into folder, written
// with the row. The sandbox answer is always "": the native window has no
// confinement step, so the provider's rung decides.
func (c *Client) AddSessionInFolder(ctx context.Context, projectID, sessionID, label, kind, path string, nextSeq int, folder string) error {
	_, err := callMethod(ctx, c, api.StoreAddSessionInFolder, api.AddSessionInFolderOptions{AddSessionOptions: api.AddSessionOptions{ProjectID: projectID, SessionID: sessionID, Label: label, Kind: kind, Path: path, NextSeq: nextSeq}, Folder: folder})
	return err
}

// DetectProviders probes which provider CLIs this machine has, nil when the
// backend answers none.
func (c *Client) DetectProviders(ctx context.Context) ([]DetectedProvider, error) {
	return callMethod(ctx, c, api.ProvidersDetect, api.None{})
}
