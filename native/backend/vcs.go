package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

const (
	CheckOK    = api.CheckOK
	CheckEmpty = api.CheckEmpty
)

type BinaryCheck = api.BinaryCheck

// CommitIdentity mirrors project.CommitIdentity: who the next commit in a
// checkout would be authored as. Every field empty means none is configured.
type CommitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Local bool   `json:"local"`
}

// Verify resolves bin as a session's spawn would, through the pinned $PATH.
func (c *Client) Verify(ctx context.Context, bin string) (BinaryCheck, error) {
	return callMethod(ctx, c, api.ProvidersVerify, api.BinaryOptions{Bin: bin})
}

type ProviderBinary = api.ProviderBinary

// ProviderVersion runs bin, resolved as Verify resolves it, with --version.
// It boots the whole CLI, so it is asked off the frame.
func (c *Client) ProviderVersion(ctx context.Context, bin string) (string, error) {
	return callMethod(ctx, c, api.ProvidersVersion, api.BinaryOptions{Bin: bin})
}

// ProviderBinaries lists every distinct binary of provider id on the pinned
// $PATH, in $PATH order, each with its version.
func (c *Client) ProviderBinaries(ctx context.Context, id string) ([]ProviderBinary, error) {
	return callMethod(ctx, c, api.ProvidersOnPath, api.ProviderIDOptions{ID: id})
}

// RefreshPath re-reads the login shell's $PATH and re-pins it, so the next
// Verify answers from what is installed now.
func (c *Client) RefreshPath(ctx context.Context) error {
	_, err := callMethod(ctx, c, api.ProvidersRefreshPath, api.None{})
	return err
}

// GitHubAccounts lists gh's signed-in accounts as "<host>/<login>"; empty
// when gh is signed in to none. A round trip to every host gh knows.
func (c *Client) GitHubAccounts(ctx context.Context) ([]string, error) {
	var accounts []string
	err := c.Call(ctx, "project.GitHubAccounts", &accounts)
	if accounts == nil {
		accounts = []string{}
	}
	return accounts, err
}

// CommitIdentity returns who commits in the checkout at path land as.
func (c *Client) CommitIdentity(ctx context.Context, path string) (CommitIdentity, error) {
	var id CommitIdentity
	return id, c.Call(ctx, "project.CommitIdentity", &id, path)
}
