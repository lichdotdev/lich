package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// QuotaWindow mirrors quota.Window (internal/quota/quota.go).
type QuotaWindow struct {
	Label        string `json:"label"`
	Seconds      int    `json:"seconds"`
	Percent      int    `json:"percent"`
	ResetsAt     string `json:"resetsAt,omitempty"`
	Active       bool   `json:"active,omitempty"`
	LockedReason string `json:"lockedReason,omitempty"`
	Ahead        bool   `json:"ahead,omitempty"`
}

// QuotaPlan mirrors quota.Plan. Status is "ok", "signed-out", "error" or
// "unknown"; NoAccount is "token-login" or "".
type QuotaPlan struct {
	Provider  string        `json:"provider"`
	Name      string        `json:"name"`
	Plan      string        `json:"plan,omitempty"`
	Account   string        `json:"account,omitempty"`
	NoAccount string        `json:"noAccount,omitempty"`
	Windows   []QuotaWindow `json:"windows,omitempty"`
	Status    string        `json:"status"`
}

type Attachment = api.DropAttachment

// QuotaPlans reads every metered plan for the account sessionID spends; ""
// reads the machine's own login. The backend serves it from a five-minute
// cache.
func (c *Client) QuotaPlans(ctx context.Context, sessionID string) ([]QuotaPlan, error) {
	var plans []QuotaPlan
	return plans, c.Call(ctx, "quota.Plans", &plans, sessionID)
}

// HandsOn is how long session id has been worked on, in whole seconds.
func (c *Client) HandsOn(ctx context.Context, id string) (int64, error) {
	var seconds int64
	return seconds, c.Call(ctx, "terminal.HandsOn", &seconds, id)
}

// Attach opens the backend's file chooser for session sessionID. root is the
// session's checkout and confined whether it runs in the sandbox: a file
// outside what the session can read comes back as a copy's path.
func (c *Client) Attach(ctx context.Context, sessionID, root string, confined bool) (Attachment, error) {
	return callMethod(ctx, c, api.DropAttach, api.AttachDropOptions{SessionID: sessionID, Root: root, Confined: confined})
}

// Write types data into session id's PTY.
func (c *Client) Write(ctx context.Context, id, data string) error {
	return c.Call(ctx, "terminal.Write", nil, id, data)
}

// SetSetting writes a setting; scope is a project id, "" for a global one.
func (c *Client) SetSetting(ctx context.Context, key, scope, value string) error {
	return c.Call(ctx, "store.SetSetting", nil, key, scope, value)
}
