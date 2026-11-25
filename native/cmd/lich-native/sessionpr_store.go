package main

import (
	"context"
	"slices"

	"github.com/lichdotdev/lich/native/backend"
)

// setSessionPRs keeps the backend's word on each session's pull requests
// (api.SessionPullRequestsEvent), by session, newest first as it comes; every
// badge reads it through viewOf, and only the event and the first load write it.
func (m *model) setSessionPRs(list []backend.SessionPullRequest) {
	by := map[string][]backend.SessionPullRequest{}
	for _, l := range list {
		by[l.Session] = append(by[l.Session], l)
	}
	m.mu.Lock()
	m.prLinks = by
	m.mu.Unlock()
	m.invalidate()
}

func (m *model) loadSessionPRs(ctx context.Context, client *backend.Client) error {
	list, err := client.SessionPullRequests(ctx)
	if err != nil {
		return err
	}
	m.setSessionPRs(list)
	return nil
}

// pickSessionPR is the pull request a session's badges show and what else it
// has. The newest hand link wins over the branch's, which is never shown once
// unlinked; others are the rest of the links and the branch's, for a tooltip.
func pickSessionPR(branch *backend.PullRequest, links []backend.SessionPullRequest) (shown *backend.PullRequest, link *backend.SessionPullRequest, others []*backend.PullRequest) {
	hidden := false
	for i := range links {
		l := &links[i]
		switch {
		case !l.Linked:
			hidden = hidden || (branch != nil && l.URL == branch.URL)
		case l.PR == nil:
		case link == nil:
			shown, link = l.PR, l
		default:
			others = append(others, l.PR)
		}
	}
	linked := func(l backend.SessionPullRequest) bool { return l.Linked && l.URL == branch.URL }
	switch {
	case branch == nil || hidden:
	case link == nil:
		shown = branch
	case !slices.ContainsFunc(links, linked):
		others = append(others, branch)
	}
	return shown, link, others
}
