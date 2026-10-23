package backend

import (
	"context"
	"fmt"
	"net/http"

	"github.com/lichdotdev/lich/api"
)

// The phases of AppUpdateProgress.
const (
	UpdatePhaseDownload  = "download"
	UpdatePhaseInstaller = "installer"
)

type AppUpdateProgress = api.AppUpdateProgress

// ApplyUpdate downloads, verifies and hands the latest release to the
// Windows installer, which closes and reopens lich. Only valid where
// AppUpdateStatus answered CanSelfApply; it returns once the installer runs.
func (c *Client) ApplyUpdate(ctx context.Context) error {
	_, err := callMethod(ctx, c, api.AppUpdateApply, api.None{})
	return err
}

// Restart relaunches lich in place: the backend answers 204, then spawns its
// successor and closes. It is POST /restart, not an RPC method.
func (c *Client) Restart(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("http", "/restart"), nil)
	if err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("restart: %s", resp.Status)
	}
	return nil
}
