package project

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

type BaseFreshness = api.BaseFreshness

// BaseFreshness fetches opts.Branch's upstream and counts the commits each side
// has that the other lacks. A fetch that fails is part of the answer, never an
// error: the caller still has a local branch to start from. It is an error
// only for a branch the repository does not hold.
func (s *Service) BaseFreshness(_ context.Context, opts api.PathBranchOptions) (BaseFreshness, error) {
	if _, ok := gitQuiet(opts.Path, "rev-parse", "--verify", "--quiet", "refs/heads/"+opts.Branch); !ok {
		return BaseFreshness{}, fmt.Errorf("%q is not a local branch of this repository.", opts.Branch)
	}
	remote, _ := gitQuiet(opts.Path, "config", "branch."+opts.Branch+".remote")
	merge, _ := gitQuiet(opts.Path, "config", "branch."+opts.Branch+".merge")
	remote = strings.TrimSpace(remote)
	upstreamBranch, onRemote := strings.CutPrefix(strings.TrimSpace(merge), "refs/heads/")
	// "." is a branch tracking another local branch: nothing to fetch.
	if remote == "" || remote == "." || !onRemote {
		return BaseFreshness{}, nil
	}
	f := BaseFreshness{Upstream: remote + "/" + upstreamBranch}
	if err := s.fetchBaseRef(opts.Path, remote, upstreamBranch); err != nil {
		f.FetchError = err.Error()
	}
	counts, err := runGit(opts.Path, "rev-list", "--left-right", "--count", "refs/heads/"+opts.Branch+"...refs/remotes/"+f.Upstream)
	// A remote never fetched has no ref to count against, and the failed
	// fetch already says why.
	if err != nil && f.FetchError != "" {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	f.Ahead, f.Behind, err = parseCounts(counts)
	return f, err
}

// UpstreamCounts counts HEAD against its upstream from the refs on disk, with
// no fetch: the numbers `git status` prints. A detached HEAD or a branch with
// no upstream answers zero.
func (s *Service) UpstreamCounts(_ context.Context, opts api.PathOptions) (BaseFreshness, error) {
	upstream, ok := gitQuiet(opts.Path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if !ok {
		return BaseFreshness{}, nil
	}
	f := BaseFreshness{Upstream: strings.TrimSpace(upstream)}
	counts, err := runGit(opts.Path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return f, err
	}
	f.Ahead, f.Behind, err = parseCounts(counts)
	return f, err
}

// parseCounts reads `rev-list --left-right --count`: ahead, a tab, behind.
func parseCounts(counts string) (ahead, behind int, err error) {
	left, right, _ := strings.Cut(strings.TrimSpace(counts), "\t")
	if ahead, err = strconv.Atoi(left); err != nil {
		return 0, 0, fmt.Errorf("git counted %q commits ahead: %w", left, err)
	}
	if behind, err = strconv.Atoi(right); err != nil {
		return 0, 0, fmt.Errorf("git counted %q commits behind: %w", right, err)
	}
	return ahead, behind, nil
}
