package project

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
)

// forge is the part of the pull request flows that differs between GitHub (gh)
// and GitLab (glab). The Service methods keep what both share: events, the
// OPEN-only gate, worktree bookkeeping and the watch. A GitLab merge request
// answers in GitHub's vocabulary, so the screens never fork.
type forge interface {
	kind() string
	// branchPullRequest is the pull request of the branch at path in any
	// state; errNoPullRequest or nil when it has none.
	branchPullRequest(path string) (*PullRequest, error)
	// watchRead fills the head, checks and review of the open pull request a
	// watched checkout's branch has.
	watchRead(path string, read *api.PRRead) error
	pullRequestDetail(path string, number int, openOnly bool) (*PRDetail, error)
	listPullRequests(path, state string) ([]PRSummary, error)
	pullRequestDiff(path string, number int) (string, error)
	createPullRequest(path string) error
	mergePullRequest(path string, opts api.MergePullRequestOptions) error
	pullRequestHead(path string, number int) (prHead, error)
	// checkoutPullRequest checks number out inside the detached worktree at
	// wtPath; accountDir names whose account the call runs as.
	checkoutPullRequest(accountDir, wtPath string, number int) error
	checkLog(dir string, lines int) (api.CheckLog, error)
}

// Forge answers which forge opts.Path's remote points at.
func (s *Service) Forge(_ context.Context, opts api.PathOptions) (string, error) {
	return s.forgeOf(opts.Path).kind(), nil
}

// forgeOf picks the forge of the checkout at path from its remote: origin, else
// the first remote git lists. github.com and gitlab.com name themselves; any
// other host is a GitLab instance only when glab holds a token for it, so a
// GitHub Enterprise remote and a checkout with no remote stay GitHub, as they
// were before GitLab existed here.
func (s *Service) forgeOf(path string) forge {
	switch host := remoteHost(path); {
	case host == "github.com":
	case host == "gitlab.com", host != "" && s.glabHost(host):
		return gitlabForge{run: s.glab}
	}
	return githubForge{s: s}
}

// remoteHost is the host of the checkout's forge remote, "" for none or a
// remote on the local disk.
func remoteHost(path string) string {
	out, ok := gitQuiet(path, "remote")
	if !ok {
		return ""
	}
	remotes := strings.Fields(out)
	if len(remotes) == 0 {
		return ""
	}
	name := remotes[0]
	for _, r := range remotes {
		if r == "origin" {
			name = r
		}
	}
	// get-url rather than the config value: it applies url.<base>.insteadOf.
	remote, ok := gitQuiet(path, "remote", "get-url", "--", name)
	if !ok {
		return ""
	}
	return urlHost(strings.TrimSpace(remote))
}

// urlHost reads the host off the three shapes a git remote takes:
// scheme://[user@]host[:port]/path, scp-like [user@]host:path, and a local path,
// which has none. It keys a host the way glab does (glab 1.131, git.ParseURL):
// an http(s) port is part of the host, an ssh one is not.
func urlHost(remote string) string {
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		switch {
		case err != nil || u.Scheme == "file":
			return ""
		case u.Scheme == "ssh" || u.Scheme == "git+ssh":
			return strings.ToLower(u.Hostname())
		}
		return strings.ToLower(u.Host)
	}
	host, _, scp := strings.Cut(remote, ":")
	// A colon after a slash is a local path, as git itself reads it, and so is
	// a Windows one (glab 1.131, git.ParseURL).
	if !scp || strings.Contains(host, "/") || strings.Contains(remote, `\`) {
		return ""
	}
	if _, after, ok := strings.Cut(host, "@"); ok {
		host = after
	}
	return strings.ToLower(host)
}

// glabProbeTTL is how long one answer of glabHost holds. The probe spawns glab
// (about 300 ms measured), and every pull request call of a GitHub Enterprise
// checkout asks it; a host signed in to with glab later is seen within it.
const glabProbeTTL = time.Minute

type glabProbe struct {
	gitlab bool
	at     time.Time
}

// glabHosts caches glabHost per host. Zero value ready.
type glabHosts struct {
	mu   sync.Mutex
	seen map[string]glabProbe
}

// glabHost answers whether glab holds a token for host. Only token is a valid
// probe: glab answers a default for any other key on a host it has never seen
// (measured, glab 1.131, api_protocol). glab missing answers no.
func (s *Service) glabHost(host string) bool {
	if s.glab == nil {
		return false
	}
	s.glabProbes.mu.Lock()
	defer s.glabProbes.mu.Unlock()
	if p, ok := s.glabProbes.seen[host]; ok && time.Since(p.at) < glabProbeTTL {
		return p.gitlab
	}
	out, err := s.glab(glabProbeTimeout, "", "config", "get", "token", "--host", host)
	p := glabProbe{gitlab: err == nil && strings.TrimSpace(string(out)) != "", at: time.Now()}
	if s.glabProbes.seen == nil {
		s.glabProbes.seen = map[string]glabProbe{}
	}
	s.glabProbes.seen[host] = p
	return p.gitlab
}
