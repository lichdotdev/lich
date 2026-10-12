package lichclient

import "context"

// The Status of a BinaryCheck that passed, and of one with nothing to check;
// every other value is a failure (providers.Check).
const (
	CheckOK    = "ok"
	CheckEmpty = "empty"
)

// BinaryCheck mirrors providers.Check: Path is the executable a session would
// spawn, empty for every status but CheckOK.
type BinaryCheck struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// Failed reports a check to show as a problem.
func (b BinaryCheck) Failed() bool { return b.Status != CheckOK && b.Status != CheckEmpty }

// CommitIdentity mirrors project.CommitIdentity: who the next commit in a
// checkout would be authored as. Every field empty means none is configured.
type CommitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Local bool   `json:"local"`
}

// Verify resolves bin as a session's spawn would, through the pinned $PATH.
func (c *Client) Verify(ctx context.Context, bin string) (BinaryCheck, error) {
	var b BinaryCheck
	return b, c.Call(ctx, "providers.Verify", &b, bin)
}

// RefreshPath re-reads the login shell's $PATH and re-pins it, so the next
// Verify answers from what is installed now.
func (c *Client) RefreshPath(ctx context.Context) error {
	return c.Call(ctx, "providers.RefreshPath", nil)
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
