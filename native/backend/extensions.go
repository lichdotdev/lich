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
	ExtensionText    = api.ExtensionText
	ExtensionMessage = api.ExtensionMessage
	// ExtensionInstallPreview is a staged install waiting for its confirmation.
	ExtensionInstallPreview = api.ExtensionInstallPreview
	ExtensionCatalog        = api.ExtensionCatalog
)

// ExtensionCatalog is the lich catalog the backend holds, read first when it
// holds none.
func (c *Client) ExtensionCatalog(ctx context.Context) (ExtensionCatalog, error) {
	return callMethod(ctx, c, api.ExtensionsCatalog, api.None{})
}

// ExtensionCatalogImage reads an image the catalog names, through the
// backend.
func (c *Client) ExtensionCatalogImage(ctx context.Context, url string) ([]byte, error) {
	return callMethod(ctx, c, api.ExtensionsCatalogImage, api.ExtensionCatalogImageOptions{URL: url})
}

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

// PrepareExtensionFromGit stages the latest release of the GitHub repository
// at url; nothing runs until CommitExtensionInstall.
func (c *Client) PrepareExtensionFromGit(ctx context.Context, url string) (ExtensionInstallPreview, error) {
	return callMethod(ctx, c, api.ExtensionsPrepareFromGit, api.ExtensionGitInstallOptions{URL: url})
}

// PrepareExtensionFromDir stages a copy of the extension in dir.
func (c *Client) PrepareExtensionFromDir(ctx context.Context, dir string) (ExtensionInstallPreview, error) {
	return callMethod(ctx, c, api.ExtensionsPrepareFromDir, api.ExtensionDirInstallOptions{Dir: dir})
}

// CommitExtensionInstall installs and starts the staged extension token names.
func (c *Client) CommitExtensionInstall(ctx context.Context, token string) (Extension, error) {
	return callMethod(ctx, c, api.ExtensionsCommitInstall, api.ExtensionInstallTokenOptions{Token: token})
}

// DiscardExtensionInstall deletes the staged extension token names.
func (c *Client) DiscardExtensionInstall(ctx context.Context, token string) error {
	_, err := callMethod(ctx, c, api.ExtensionsDiscardInstall, api.ExtensionInstallTokenOptions{Token: token})
	return err
}

// CheckExtensionUpdates looks for newer releases now and returns every
// extension with what it found.
func (c *Client) CheckExtensionUpdates(ctx context.Context) ([]Extension, error) {
	return callMethod(ctx, c, api.ExtensionsCheckUpdates, api.None{})
}

// PrepareExtensionUpdate stages the newer release of extension id; nothing
// changes until CommitExtensionInstall.
func (c *Client) PrepareExtensionUpdate(ctx context.Context, id string) (ExtensionInstallPreview, error) {
	return callMethod(ctx, c, api.ExtensionsPrepareUpdate, api.ExtensionIDOptions{ID: id})
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
