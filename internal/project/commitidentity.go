package project

import "github.com/lichdotdev/lich/api"

import "strings"

type CommitIdentity = api.CommitIdentity

// CommitIdentity reads the identity git itself resolves in the checkout at
// path, so a repository-local override wins over the global config exactly as
// it would for a commit. A non-repository, an unreadable config or no identity
// at all answer the zero value — hence the quiet probes (see runGit for why).
func (s *Service) CommitIdentity(path string) CommitIdentity {
	identity := CommitIdentity{
		Name:  gitConfig(path, "user.name"),
		Email: gitConfig(path, "user.email"),
	}
	// The email alone decides "set in this repository": it is the value git
	// refuses a commit without, and the one the readout shows.
	identity.Local = identity.Email != "" && gitConfig(path, "--local", "user.email") != ""
	return identity
}

// gitConfig reads one config key in the checkout at path — with git's own
// resolution order, or narrowed to a scope ("--local") when one leads the
// arguments. "" for every key git declines to answer.
func gitConfig(path string, args ...string) string {
	out, _ := gitQuiet(path, append([]string{"config", "--get"}, args...)...)
	return strings.TrimSpace(out)
}
