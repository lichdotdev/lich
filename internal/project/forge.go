package project

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
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
	// commitRangeDiff asks the forge for a range of commits this clone does
	// not hold.
	commitRangeDiff(path, first, last string) (api.CommitRangeDiff, error)
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
	f := s.pickForge(path)
	s.noteForge(path, f.kind())
	return f
}

func (s *Service) pickForge(path string) forge {
	switch host := remoteHost(path); {
	case host == "github.com":
	case host == "gitlab.com", host != "" && s.glabHost(host):
		return gitlabForge{run: s.glab}
	}
	return githubForge{s: s}
}

// forgeKinds is the last forge each checkout was answered with. Zero value
// ready.
type forgeKinds struct {
	mu   sync.Mutex
	last map[string]string
}

// noteForge announces path's forge when it differs from the last answer for
// path. The first answer is not news: whoever asked has it.
func (s *Service) noteForge(path, kind string) {
	s.forges.mu.Lock()
	prev, seen := s.forges.last[path]
	if s.forges.last == nil {
		s.forges.last = map[string]string{}
	}
	s.forges.last[path] = kind
	s.forges.mu.Unlock()
	if seen && prev != kind && s.emit != nil {
		events.Send(s.emit, api.ForgeEvent, api.ForgeChange{Path: path, Forge: kind})
	}
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
	p, ok := s.glabProbes.seen[host]
	s.glabProbes.mu.Unlock()
	if ok && time.Since(p.at) < glabProbeTTL {
		return p.gitlab
	}
	// Unlocked while glab runs, so one slow host does not hold up another's
	// probe; two probes of one host at once both answer and the last one stays.
	out, err := s.glab(glabProbeTimeout, "", "config", "get", "token", "--host", host)
	p = glabProbe{gitlab: err == nil && strings.TrimSpace(string(out)) != "", at: time.Now()}
	s.glabProbes.mu.Lock()
	if s.glabProbes.seen == nil {
		s.glabProbes.seen = map[string]glabProbe{}
	}
	s.glabProbes.seen[host] = p
	s.glabProbes.mu.Unlock()
	return p.gitlab
}
