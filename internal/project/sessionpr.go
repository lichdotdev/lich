package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// A session's pull request is its branch's unless the person said otherwise:
// one linked by hand shows beside it (an agent opened it from another branch,
// a fork, another directory), one unlinked stays hidden even when it is the
// branch's. Each word is kept per session and URL, the latest one winning,
// and the window reads them from api.SessionPullRequestsEvent.

// sessionPRStore keeps the links across restarts; main wires the workspace
// store, and nil keeps nothing.
type sessionPRStore interface {
	SessionPullRequests() ([]api.SessionPullRequest, error)
	SaveSessionPullRequest(api.SessionPullRequest) error
}

// SetSessionPullRequests wires where the links are kept.
func (s *Service) SetSessionPullRequests(store sessionPRStore) { s.links = store }

var (
	errBadPRRef   = errors.New("not a pull request: give #N, or the URL of a GitHub pull request or a GitLab merge request")
	errNoLinks    = errors.New("pull request links are not kept by this backend")
	errNoGLab     = errors.New("glab is not available to read a GitLab merge request")
	errNoLinkedPR = errors.New("the forge has no such pull request")
)

// prBadgeFields is what a pull request's badge reads.
const prBadgeFields = "number,url,title,state,isDraft,headRefName,baseRefName"

// prRef is what a person typed to name a pull request: a number in the
// session's own repository (forge ""), or a URL that names its forge.
type prRef struct {
	forge  string
	number int
	// url is the pull request's own on GitHub, which gh reads; repo is the
	// project's on GitLab, which glab reads the merge request in.
	url, repo string
}

// parsePRRef reads #N (!N, GitLab's way, too), a GitHub pull request URL
// (https://host/owner/repo/pull/N, any tab after it) or a GitLab merge request
// URL (https://host/group/…/project/-/merge_requests/N).
func parsePRRef(ref string) (prRef, error) {
	ref = strings.TrimSpace(ref)
	if digits, ok := cutRefPrefix(ref); ok {
		if n, err := strconv.Atoi(digits); err == nil && n > 0 {
			return prRef{number: n}, nil
		}
		return prRef{}, errBadPRRef
	}
	u, err := url.Parse(ref)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return prRef{}, errBadPRRef
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	base := u.Scheme + "://" + u.Host + "/"
	if i := slices.Index(segs, "-"); i >= 2 && len(segs) > i+2 && segs[i+1] == "merge_requests" {
		if n, err := strconv.Atoi(segs[i+2]); err == nil && n > 0 {
			return prRef{forge: api.ForgeGitLab, number: n, repo: base + strings.Join(segs[:i], "/")}, nil
		}
	}
	if len(segs) >= 4 && segs[2] == "pull" {
		if n, err := strconv.Atoi(segs[3]); err == nil && n > 0 {
			return prRef{forge: api.ForgeGitHub, number: n, url: fmt.Sprintf("%s%s/%s/pull/%d", base, segs[0], segs[1], n)}, nil
		}
	}
	return prRef{}, errBadPRRef
}

func cutRefPrefix(ref string) (string, bool) {
	if digits, ok := strings.CutPrefix(ref, "#"); ok {
		return digits, true
	}
	return strings.CutPrefix(ref, "!")
}

// readLinkedPR reads the pull request ref names, from the checkout at path
// (whose account gh runs as), and answers the forge it is on.
func (s *Service) readLinkedPR(path string, ref prRef) (*PullRequest, string, error) {
	kind := ref.forge
	if kind == "" {
		kind = s.forgeOf(path).kind()
	}
	var pr *PullRequest
	var err error
	if kind == api.ForgeGitLab {
		pr, err = s.readLinkedMR(path, ref)
	} else {
		target := ref.url
		if target == "" {
			target = strconv.Itoa(ref.number)
		}
		pr, err = s.ghPRView(prReadTimeout, path, "pr", "view", target, "--json", prBadgeFields)
	}
	if errors.Is(err, errNoPullRequest) || (err == nil && pr == nil) {
		return nil, kind, errNoLinkedPR
	}
	return pr, kind, err
}

func (s *Service) readLinkedMR(path string, ref prRef) (*PullRequest, error) {
	if s.glab == nil {
		return nil, errNoGLab
	}
	args := []string{"mr", "view", strconv.Itoa(ref.number), "-F", "json"}
	if ref.repo != "" {
		args = append(args, "-R", ref.repo)
	}
	out, err := s.glab(prReadTimeout, path, args...)
	if err != nil {
		return nil, err
	}
	var mr glabMR
	if err := json.Unmarshal(out, &mr); err != nil {
		return nil, errUnreadableGitLab(err)
	}
	if mr.IID == 0 {
		return nil, errNoPullRequest
	}
	return mr.pullRequest(), nil
}

// LinkPullRequest links the pull request opts.Ref names to opts.Session once
// the forge has answered for it, undoing an unlink of the same one. The
// session's branch and checkout stay as they are.
func (s *Service) LinkPullRequest(_ context.Context, opts api.LinkPullRequestOptions) (api.None, error) {
	if s.links == nil {
		return api.None{}, errNoLinks
	}
	ref, err := parsePRRef(opts.Ref)
	if err != nil {
		return api.None{}, err
	}
	pr, kind, err := s.readLinkedPR(opts.Path, ref)
	if err != nil {
		return api.None{}, err
	}
	l := api.SessionPullRequest{Session: opts.Session, URL: pr.URL, Linked: true, At: time.Now().UnixMilli(),
		Forge: kind, Here: ref.forge == "", PR: pr}
	return api.None{}, s.saveLink(l)
}

// UnlinkPullRequest hides opts.URL from opts.Session, for good: the branch's
// reads never bring it back, only a link does.
func (s *Service) UnlinkPullRequest(_ context.Context, opts api.UnlinkPullRequestOptions) (api.None, error) {
	if s.links == nil {
		return api.None{}, errNoLinks
	}
	return api.None{}, s.saveLink(api.SessionPullRequest{Session: opts.Session, URL: opts.URL, At: time.Now().UnixMilli()})
}

func (s *Service) saveLink(l api.SessionPullRequest) error {
	if err := s.links.SaveSessionPullRequest(l); err != nil {
		return err
	}
	s.emitSessionPRs()
	return nil
}

// SessionPullRequests is every word kept on a session's pull requests, the
// newest first.
func (s *Service) SessionPullRequests(_ context.Context, _ api.None) ([]api.SessionPullRequest, error) {
	if s.links == nil {
		return []api.SessionPullRequest{}, nil
	}
	return s.links.SessionPullRequests()
}

func (s *Service) emitSessionPRs() {
	if s.emit == nil {
		return
	}
	list, err := s.SessionPullRequests(context.Background(), api.None{})
	if err != nil {
		slog.Warn("session pull requests: list", "err", err)
		return
	}
	events.Send(s.emit, api.SessionPullRequestsEvent, list)
}

// refreshLinkedPRs reads again every linked pull request of a live session
// that is still open or whose last read failed, and announces the list once
// one of them reads differently. A failed read keeps the last good one beside
// its reason.
func (s *Service) refreshLinkedPRs() {
	if s.links == nil || s.sessionCheckout == nil {
		return
	}
	list, err := s.links.SessionPullRequests()
	if err != nil {
		slog.Warn("session pull requests: list", "err", err)
		return
	}
	changed := false
	for _, l := range list {
		dir := s.sessionCheckout(l.Session)
		if !l.Linked || dir == "" || (l.PR != nil && l.PR.State != prOpen && l.Error == "") {
			continue
		}
		next, ok := s.rereadLink(dir, l)
		if !ok {
			continue
		}
		if err := s.links.SaveSessionPullRequest(next); err != nil {
			slog.Warn("session pull requests: save a read", "url", l.URL, "err", err)
			continue
		}
		changed = true
	}
	if changed {
		s.emitSessionPRs()
	}
}

// rereadLink is l after a new read, and whether it changed.
func (s *Service) rereadLink(dir string, l api.SessionPullRequest) (api.SessionPullRequest, bool) {
	ref, err := parsePRRef(l.URL)
	if err != nil {
		return l, false
	}
	pr, _, err := s.readLinkedPR(dir, ref)
	next := l
	if err != nil {
		next.PR, next.Error = nil, err.Error()
		return next, next.Error != l.Error
	}
	next.PR, next.Error = pr, ""
	return next, l.Error != "" || l.PR == nil || *pr != *l.PR
}
