package project

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

type BaseFreshness = api.BaseFreshness

// BaseFreshness fetches branch's upstream and counts the commits each side
// has that the other lacks. A fetch that fails is part of the answer, never an
// error: the caller still has a local branch to start from. It is an error
// only for a branch the repository does not hold.
func (s *Service) BaseFreshness(path, branch string) (BaseFreshness, error) {
	if _, ok := gitQuiet(path, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); !ok {
		return BaseFreshness{}, fmt.Errorf("%q is not a local branch of this repository.", branch)
	}
	remote, _ := gitQuiet(path, "config", "branch."+branch+".remote")
	merge, _ := gitQuiet(path, "config", "branch."+branch+".merge")
	remote = strings.TrimSpace(remote)
	upstreamBranch, onRemote := strings.CutPrefix(strings.TrimSpace(merge), "refs/heads/")
	// "." is a branch tracking another local branch: nothing to fetch.
	if remote == "" || remote == "." || !onRemote {
		return BaseFreshness{}, nil
	}
	f := BaseFreshness{Upstream: remote + "/" + upstreamBranch}
	if err := s.fetchBaseRef(path, remote, upstreamBranch); err != nil {
		f.FetchError = err.Error()
	}
	counts, err := runGit(path, "rev-list", "--left-right", "--count", "refs/heads/"+branch+"...refs/remotes/"+f.Upstream)
	// A remote never fetched has no ref to count against, and the failed
	// fetch already says why.
	if err != nil && f.FetchError != "" {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	ahead, behind, _ := strings.Cut(strings.TrimSpace(counts), "\t")
	if f.Ahead, err = strconv.Atoi(ahead); err != nil {
		return f, fmt.Errorf("git counted %q commits ahead: %w", ahead, err)
	}
	if f.Behind, err = strconv.Atoi(behind); err != nil {
		return f, fmt.Errorf("git counted %q commits behind: %w", behind, err)
	}
	return f, nil
}
