package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type ClosedSession = api.ParkedSession

type ClosedHistory = api.ClosedHistory

type ExternalSession = api.ExternalSession

type TranscriptMatch = api.TranscriptMatch

// RecentProjectsMatching is RecentProjects narrowed to the closed projects
// whose name or path hold every word of term.
func (c *Client) RecentProjectsMatching(ctx context.Context, term string) ([]RecentProject, error) {
	return callMethod(ctx, c, api.StoreRecentProjects, api.SearchTermOptions{Term: term})
}

// ClosedProjectCountMatching counts every closed project matching term, past
// the page RecentProjectsMatching answers with.
func (c *Client) ClosedProjectCountMatching(ctx context.Context, term string) (int, error) {
	return callMethod(ctx, c, api.StoreClosedProjectCount, api.SearchTermOptions{Term: term})
}

// ClosedSessions searches the parked sessions by name, project, path, branch
// and conversation; an empty term is the plain history, newest close first.
// A non-empty term also starts indexing any conversation not indexed yet.
func (c *Client) ClosedSessions(ctx context.Context, term string) (ClosedHistory, error) {
	return callMethod(ctx, c, api.StoreClosedSessions, api.SearchTermOptions{Term: term})
}

// ExternalSessions lists the conversations started outside lich in a known
// project's checkouts that no session row holds, newest first.
func (c *Client) ExternalSessions(ctx context.Context) ([]ExternalSession, error) {
	return callMethod(ctx, c, api.StoreExternalSessions, api.None{})
}

// AdoptExternalSession files an external conversation as a parked session of
// projectID under sessionID, which ReopenSession then brings back. It fails
// when a session already holds the conversation.
func (c *Client) AdoptExternalSession(ctx context.Context, projectID, sessionID, kind, path, providerSessionID, label string) error {
	_, err := callMethod(ctx, c, api.StoreAdoptExternalSession, api.AdoptExternalSessionOptions{ProjectID: projectID, SessionID: sessionID, Kind: kind, Path: path, ProviderSessionID: providerSessionID, Label: label})
	return err
}

// ForgetSession deletes one parked session for good. It fails on a session
// that is open.
func (c *Client) ForgetSession(ctx context.Context, sessionID string) error {
	_, err := callMethod(ctx, c, api.StoreForgetSession, api.SessionIDOptions{ID: sessionID})
	return err
}

// SearchTranscripts finds which of the sessions ids have talked about query
// and what they said, in the order the sessions were given. A query under
// three characters is refused and answers none.
func (c *Client) SearchTranscripts(ctx context.Context, ids []string, query string) ([]TranscriptMatch, error) {
	return callMethod(ctx, c, api.TerminalSearchTranscripts, api.SearchTranscriptsOptions{IDs: ids, Query: query})
}

// BranchesOf names the branch each checkout path is on now; a path that is no
// repository is absent from the answer.
func (c *Client) BranchesOf(ctx context.Context, paths []string) (map[string]string, error) {
	var named map[string]string
	return named, c.Call(ctx, "project.BranchesOf", &named, paths)
}
