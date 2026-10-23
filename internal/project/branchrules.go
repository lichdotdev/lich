package project

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/lichdotdev/lich/api"
)

type BranchRules = api.BranchRules

type CommitPattern = api.CommitPattern

// commitPatternTargets maps the rule types that test commit text onto the word
// the screen puts in the sentence. GitHub has pattern rules for branch and tag
// names too; they gate a push, never a merge, so they are left out.
var commitPatternTargets = map[string]string{
	"commit_message_pattern":      "message",
	"commit_author_email_pattern": "author email",
	"committer_email_pattern":     "committer email",
}

// ghBranchRule is one entry of the rules list; the pull_request rule and the
// commit pattern rules are all this reads.
type ghBranchRule struct {
	Type       string `json:"type"`
	Parameters struct {
		AllowedMergeMethods []string `json:"allowed_merge_methods"`
		Operator            string   `json:"operator"`
		Pattern             string   `json:"pattern"`
		Negate              bool     `json:"negate"`
	} `json:"parameters"`
}

// BranchRules reads the rules a repository's rulesets apply to one branch, and
// whether this account may merge past them.
//
// A branch nothing governs answers with an empty list, and so does a token that
// may not read the rules — both reach the screen as "no rule known", which is
// the same shape as a repository that has none. That is deliberate: this read
// exists to *narrow* what the merge menu offers, so failing to answer must
// leave the menu as wide as it was, never empty. The bypass is the exception
// and runs the other way; see ViewerCanBypass and viewerAdministers.
func (s *Service) BranchRules(_ context.Context, opts api.PathBranchOptions) (*BranchRules, error) {
	if opts.Branch == "" {
		return nil, fmt.Errorf("a branch is required")
	}
	out, err := s.gh(prReadTimeout, opts.Path, "api",
		"repos/{owner}/{repo}/rules/branches/"+url.PathEscape(opts.Branch))
	if err != nil {
		return nil, err
	}
	rules := parseBranchRules(out)
	rules.ViewerCanBypass = s.viewerAdministers(opts.Path)
	return rules, nil
}

// viewerAdministers reports whether the account gh uses for this checkout
// administers the repository — GitHub's own bar for the merge override.
//
// A second call, because nothing on a pull request or in the rules answers it.
// It costs one small read per base branch, which the screen caches, and buys
// the thing a click cannot: an action that is *absent* rather than offered and
// refused. Every failure — the call, the decode, a repository gh cannot see —
// is that same absence, never an error: the rules this rides along with are
// worth drawing on their own.
//
// Only ADMIN counts. A ruleset can name any role as a bypass actor, so an
// account that may bypass without administering the repository does not get
// the option here; reading each ruleset's bypass list to find out is the
// upgrade path.
func (s *Service) viewerAdministers(path string) bool {
	out, err := s.gh(prReadTimeout, path, "repo", "view", "--json", "viewerPermission")
	if err != nil {
		return false
	}
	var repo struct {
		ViewerPermission string `json:"viewerPermission"`
	}
	if err := json.Unmarshal(out, &repo); err != nil {
		return false
	}
	return repo.ViewerPermission == "ADMIN"
}

// parseBranchRules keeps what the merge menu asks for. An answer this cannot
// read is not an error: GitHub returns a plain object rather than a list for a
// branch it will not report on, and a screen that refuses to draw its Merge
// menu over that is worse than one that draws the whole menu.
func parseBranchRules(out []byte) *BranchRules {
	var rules []ghBranchRule
	if err := json.Unmarshal(out, &rules); err != nil {
		return &BranchRules{}
	}
	parsed := &BranchRules{}
	for _, rule := range rules {
		if rule.Type == "pull_request" && len(rule.Parameters.AllowedMergeMethods) > 0 {
			parsed.AllowedMergeMethods = rule.Parameters.AllowedMergeMethods
		}
		// A pattern with nothing to test against explains nothing, so it is
		// dropped rather than shown as an empty requirement.
		if target, ok := commitPatternTargets[rule.Type]; ok && rule.Parameters.Pattern != "" {
			parsed.CommitPatterns = append(parsed.CommitPatterns, CommitPattern{
				Target:   target,
				Operator: rule.Parameters.Operator,
				Pattern:  rule.Parameters.Pattern,
				Negate:   rule.Parameters.Negate,
			})
		}
	}
	return parsed
}
