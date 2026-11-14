package project

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// branchName refuses what git would read as something other than a branch name:
// a leading dash is an option, and check-ref-format says the rest.
func branchName(dir, name string) error {
	if name == "" || strings.HasPrefix(name, "-") {
		return fmt.Errorf("%q is not a valid branch name.", name)
	}
	_, err := runGit(dir, "check-ref-format", "--branch", name)
	return err
}

// GitCheckout switches the checkout to a branch. git refuses a switch that
// would overwrite uncommitted edits and a branch checked out in another
// worktree; its message is the answer.
func (s *Service) GitCheckout(_ context.Context, opts api.GitCheckoutOptions) (api.None, error) {
	if err := branchName(opts.Path, opts.Branch); err != nil {
		return api.None{}, err
	}
	args := []string{"switch", opts.Branch}
	if opts.Remote {
		args = []string{"switch", "--track", opts.Branch}
	}
	_, err := runGit(opts.Path, args...)
	return api.None{}, err
}

// GitCreateBranch makes a branch at From and leaves the checkout where it is.
func (s *Service) GitCreateBranch(_ context.Context, opts api.GitCreateBranchOptions) (api.None, error) {
	if err := branchName(opts.Path, opts.Name); err != nil {
		return api.None{}, err
	}
	if opts.From == "" || strings.HasPrefix(opts.From, "-") {
		return api.None{}, fmt.Errorf("%q is not a commit to branch from.", opts.From)
	}
	if _, err := runGit(opts.Path, "branch", opts.Name, opts.From); err != nil {
		return api.None{}, err
	}
	return api.None{}, setUpstream(opts.Path, opts.Name, opts.Upstream)
}

func upstreamKeys(name string, u *api.GitUpstream) map[string]*string {
	return map[string]*string{
		"branch." + name + ".remote":     &u.Remote,
		"branch." + name + ".merge":      &u.Merge,
		"branch." + name + ".pushRemote": &u.PushRemote,
	}
}

// readUpstream reads the settings git drops with the branch. --default makes
// an unset key an empty answer instead of an exit status.
func readUpstream(dir, name string) (api.GitUpstream, error) {
	var u api.GitUpstream
	for key, dst := range upstreamKeys(name, &u) {
		out, err := runGit(dir, "config", "--default", "", "--get", key)
		if err != nil {
			return api.GitUpstream{}, err
		}
		*dst = strings.TrimSpace(out)
	}
	return u, nil
}

func setUpstream(dir, name string, u api.GitUpstream) error {
	for key, v := range upstreamKeys(name, &u) {
		if *v == "" {
			continue
		}
		if _, err := runGit(dir, "config", key, *v); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) GitRenameBranch(_ context.Context, opts api.GitRenameBranchOptions) (api.None, error) {
	for _, name := range []string{opts.Name, opts.NewName} {
		if err := branchName(opts.Path, name); err != nil {
			return api.None{}, err
		}
	}
	_, err := runGit(opts.Path, "branch", "--move", opts.Name, opts.NewName)
	return api.None{}, err
}

// GitBranchOnly counts the commits no other branch, remote branch or tag
// reaches: what deleting the branch would leave without a name.
func (s *Service) GitBranchOnly(_ context.Context, opts api.GitBranchOptions) (int, error) {
	if err := branchName(opts.Path, opts.Name); err != nil {
		return 0, err
	}
	// --exclude before --branches takes the name without refs/heads/, as git
	// documents; a branch name cannot hold a glob character.
	out, err := runGit(opts.Path, "rev-list", "--count", "refs/heads/"+opts.Name, "--not", "--exclude="+opts.Name, "--branches", "--remotes", "--tags")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// GitDeleteBranch deletes a local branch whatever it holds, the caller having
// asked first, and returns the commit and upstream it had so it can be made again.
func (s *Service) GitDeleteBranch(_ context.Context, opts api.GitBranchOptions) (api.GitDeletedBranch, error) {
	if err := branchName(opts.Path, opts.Name); err != nil {
		return api.GitDeletedBranch{}, err
	}
	hash, err := runGit(opts.Path, "rev-parse", "--verify", "refs/heads/"+opts.Name)
	if err != nil {
		return api.GitDeletedBranch{}, err
	}
	upstream, err := readUpstream(opts.Path, opts.Name)
	if err != nil {
		return api.GitDeletedBranch{}, err
	}
	if _, err := runGit(opts.Path, "branch", "--delete", "--force", opts.Name); err != nil {
		return api.GitDeletedBranch{}, err
	}
	return api.GitDeletedBranch{Hash: strings.TrimSpace(hash), Upstream: upstream}, nil
}
