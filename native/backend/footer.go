package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type QuotaWindow = api.QuotaWindow

type QuotaPlan = api.QuotaPlan

type Attachment = api.DropAttachment

// QuotaPlans reads every metered plan for the account sessionID spends; ""
// reads the machine's own login. The backend serves it from a five-minute
// cache.
func (c *Client) QuotaPlans(ctx context.Context, sessionID string) ([]QuotaPlan, error) {
	return callMethod(ctx, c, api.QuotaPlans, api.QuotaPlansOptions{SessionID: sessionID})
}

// HandsOn is how long session id has been worked on, in whole seconds.
func (c *Client) HandsOn(ctx context.Context, id string) (int64, error) {
	return callMethod(ctx, c, api.TerminalHandsOn, api.SessionIDOptions{ID: id})
}

// Attach opens the backend's file chooser for session sessionID. root is the
// session's checkout and confined whether it runs in the sandbox: a file
// outside what the session can read comes back as a copy's path.
func (c *Client) Attach(ctx context.Context, sessionID, root string, confined bool) (Attachment, error) {
	return callMethod(ctx, c, api.DropAttach, api.AttachDropOptions{SessionID: sessionID, Root: root, Confined: confined})
}

// Write types data into session id's PTY.
func (c *Client) Write(ctx context.Context, id, data string) error {
	_, err := callMethod(ctx, c, api.TerminalWrite, api.WriteOptions{ID: id, Data: data})
	return err
}

// SetSetting writes a setting; scope is a project id, "" for a global one.
func (c *Client) SetSetting(ctx context.Context, key, scope, value string) error {
	_, err := callMethod(ctx, c, api.StoreSetSetting, api.SetSettingOptions{Key: key, ProjectID: scope, Value: value})
	return err
}
