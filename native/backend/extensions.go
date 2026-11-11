package backend

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

type (
	Extension        = api.Extension
	ExtensionSurface = api.ExtensionSurface
	ExtensionMessage = api.ExtensionMessage
)

// Extensions is every extension under lich's extensions directory.
func (c *Client) Extensions(ctx context.Context) ([]Extension, error) {
	return callMethod(ctx, c, api.ExtensionsList, api.None{})
}

// SendExtension writes msg to extension id's process.
func (c *Client) SendExtension(ctx context.Context, id string, msg extension.Message) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("%s: %w", api.ExtensionsSend.Name, err)
	}
	_, err = callMethod(ctx, c, api.ExtensionsSend, api.ExtensionSendOptions{ID: id, Message: raw})
	return err
}

// RestartExtension starts a failed extension again.
func (c *Client) RestartExtension(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ExtensionsRestart, api.ExtensionIDOptions{ID: id})
	return err
}

// InstallExtensionFromGit installs and starts the latest release of the
// GitHub repository at url.
func (c *Client) InstallExtensionFromGit(ctx context.Context, url string) (Extension, error) {
	return callMethod(ctx, c, api.ExtensionsInstallFromGit, api.ExtensionGitInstallOptions{URL: url})
}

// InstallExtensionFromDir copies the extension in dir and starts it.
func (c *Client) InstallExtensionFromDir(ctx context.Context, dir string) (Extension, error) {
	return callMethod(ctx, c, api.ExtensionsInstallFromDir, api.ExtensionDirInstallOptions{Dir: dir})
}

// RemoveExtension stops extension id and deletes its directory.
func (c *Client) RemoveExtension(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ExtensionsRemove, api.ExtensionIDOptions{ID: id})
	return err
}

// DeleteExtensionData deletes the data directory of extension id, which must
// be removed first.
func (c *Client) DeleteExtensionData(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ExtensionsDeleteData, api.ExtensionIDOptions{ID: id})
	return err
}

// ExtensionsDir is the directory the backend reads extensions from.
func (c *Client) ExtensionsDir(ctx context.Context) (string, error) {
	return callMethod(ctx, c, api.ExtensionsDir, api.None{})
}
