package project

import (
	"context"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// glabAccountLine matches one signed-in host in `glab auth status` output:
// "✓ Logged in to gitlab.com as ana (keyring)".
var glabAccountLine = regexp.MustCompile(`Logged in to (\S+) as (\S+)`)

// GitLabAccounts lists the accounts glab is signed in to, "<host>/<login>",
// empty when it is signed in to none or is not installed.
func (s *Service) GitLabAccounts(_ context.Context, _ api.None) ([]string, error) {
	return parseGLabAccounts(s.glabStatus()), nil
}

// runGLabStatus is `glab auth status` with stdout and stderr together, where
// glab writes it. glab exits non-zero when any one host fails, so the exit
// status is not the answer: the lines are.
func runGLabStatus() string {
	ctx, cancel := context.WithTimeout(context.Background(), ghAccountTimeout)
	defer cancel()
	out, _ := commandContext(ctx, "glab", "auth", "status").CombinedOutput()
	return string(out)
}

func parseGLabAccounts(out string) []string {
	var accounts []string
	for _, m := range glabAccountLine.FindAllStringSubmatch(out, -1) {
		if a := m[1] + "/" + m[2]; !slices.Contains(accounts, a) {
			accounts = append(accounts, a)
		}
	}
	return accounts
}

// GlobalCommitIdentity is the name and email in the user's global git
// config, the identity a checkout without its own commits as.
func (s *Service) GlobalCommitIdentity(_ context.Context, _ api.None) (CommitIdentity, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return CommitIdentity{}, err
	}
	return CommitIdentity{Name: gitConfig(home, "--global", "user.name"), Email: gitConfig(home, "--global", "user.email")}, nil
}

// SetGlobalCommitIdentity writes the name and email to the global git
// config, both or neither: git refuses a commit missing either.
func (s *Service) SetGlobalCommitIdentity(_ context.Context, opts api.CommitIdentityOptions) (api.None, error) {
	name, email := strings.TrimSpace(opts.Name), strings.TrimSpace(opts.Email)
	if name == "" || email == "" {
		return api.None{}, errors.New("a commit identity needs both a name and an email")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return api.None{}, err
	}
	for _, kv := range [][2]string{{"user.name", name}, {"user.email", email}} {
		if _, err := runGit(home, "config", "--global", kv[0], kv[1]); err != nil {
			return api.None{}, err
		}
	}
	return api.None{}, nil
}
