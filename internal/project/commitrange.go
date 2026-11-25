package project

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// errRangeNotLocalGitLab is a GitLab range whose commits this clone does not
// hold: GitLab's compare answers JSON hunks, not a diff, and is not read here.
var errRangeNotLocalGitLab = errors.New("these commits are not in this clone. Fetch the merge request's branch to see them")

// CommitRangeDiff is the diff of a pull request's commits opts.First to
// opts.Last alone, from First's parent, for the Pulls screen's commit picker.
// git answers when the clone holds both commits; a pull request's head need
// not be here (see FileLines), so the forge answers otherwise.
func (s *Service) CommitRangeDiff(_ context.Context, opts api.CommitRangeOptions) (api.CommitRangeDiff, error) {
	if !isOID(opts.First) || !isOID(opts.Last) {
		return api.CommitRangeDiff{}, fmt.Errorf("invalid commit range %q..%q", opts.First, opts.Last)
	}
	got, err := localRangeDiff(opts.Path, opts.First, opts.Last)
	if errors.Is(err, errNotLocal) {
		got, err = s.forgeOf(opts.Path).commitRangeDiff(opts.Path, opts.First, opts.Last)
	}
	if err != nil {
		return api.CommitRangeDiff{}, err
	}
	got.Diff = displayDiff(got.Diff)
	return got, nil
}

var errNotLocal = errors.New("commits not in this clone")

func localRangeDiff(path, first, last string) (api.CommitRangeDiff, error) {
	base, ok := gitQuiet(path, "rev-parse", "--verify", "--quiet", first+"~1^{commit}")
	if !ok {
		return api.CommitRangeDiff{}, errNotLocal
	}
	if _, ok := gitQuiet(path, "cat-file", "-e", last+"^{commit}"); !ok {
		return api.CommitRangeDiff{}, errNotLocal
	}
	base = strings.TrimSpace(base)
	diff, err := readGit(path, "diff", "--no-color", "--no-ext-diff", "--no-textconv", base, last)
	return api.CommitRangeDiff{Base: base, Diff: diff}, err
}

// commitRangeDiff reads First's parent, then the compare between it and Last
// as a diff. {owner}/{repo} is the base repository, which keeps a fork's
// commits too (see contentsPath).
func (g githubForge) commitRangeDiff(path, first, last string) (api.CommitRangeDiff, error) {
	out, err := g.s.gh(prReadTimeout, path, "api", "repos/{owner}/{repo}/commits/"+first, "--jq", ".parents[0].sha")
	if err != nil {
		return api.CommitRangeDiff{}, err
	}
	base := strings.TrimSpace(string(out))
	if !isOID(base) {
		return api.CommitRangeDiff{}, errUnreadableAnswer(fmt.Errorf("parent of %s %q", first, out))
	}
	diff, err := g.s.gh(prReadTimeout, path, "api", "-H", "Accept: application/vnd.github.diff",
		"repos/{owner}/{repo}/compare/"+base+"..."+last)
	return api.CommitRangeDiff{Base: base, Diff: string(diff)}, err
}

func (gitlabForge) commitRangeDiff(string, string, string) (api.CommitRangeDiff, error) {
	return api.CommitRangeDiff{}, errRangeNotLocalGitLab
}
