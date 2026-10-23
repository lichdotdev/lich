package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// ClosedSession mirrors store.ClosedSession: a parked session as the command
// palette's History tab lists it. ParkedBranch is the branch recorded at the
// close, the searchable snapshot; the branch a row shows is read live from the
// checkout (BranchesOf). ClosedAt is Unix seconds, 0 for a row parked before
// lich recorded the close.
type ClosedSession struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	ProjectName  string `json:"projectName"`
	ProjectPath  string `json:"projectPath"`
	Label        string `json:"label"`
	Kind         string `json:"kind"`
	Path         string `json:"path"`
	ParkedBranch string `json:"parkedBranch"`
	ClosedAt     int64  `json:"closedAt"`
	// MatchedConversation is whether the search reached the row through its
	// conversation, which keeps it listed even when Snippet is empty.
	MatchedConversation bool   `json:"matchedConversation"`
	Snippet             string `json:"snippet"`
	// Truncated is whether the index kept only the newest part of a
	// conversation too long to hold whole.
	Truncated bool `json:"truncated"`
}

// ClosedHistory mirrors store.ClosedHistory: one page of parked sessions, the
// size of the match it was cut from, and how many parked conversations the
// store has not indexed yet.
type ClosedHistory struct {
	Sessions []ClosedSession `json:"sessions"`
	Total    int             `json:"total"`
	Indexing int             `json:"indexing"`
}

// ExternalSession mirrors store.ExternalSession: a conversation started
// outside lich in a known project's checkout. Path is "" for the project's own
// directory; Profile is the Claude Code profile whose directory holds it.
type ExternalSession struct {
	Kind              string `json:"kind"`
	ProviderSessionID string `json:"providerSessionId"`
	Title             string `json:"title"`
	Path              string `json:"path"`
	ProjectID         string `json:"projectId"`
	ProjectName       string `json:"projectName"`
	ProjectPath       string `json:"projectPath"`
	UpdatedAt         int64  `json:"updatedAt"`
	Profile           string `json:"profile,omitempty"`
}

type TranscriptMatch = api.TranscriptMatch

// RecentProjectsMatching is RecentProjects narrowed to the closed projects
// whose name or path hold every word of term.
func (c *Client) RecentProjectsMatching(ctx context.Context, term string) ([]RecentProject, error) {
	var rows []RecentProject
	return rows, c.Call(ctx, "store.RecentProjects", &rows, term)
}

// ClosedProjectCountMatching counts every closed project matching term, past
// the page RecentProjectsMatching answers with.
func (c *Client) ClosedProjectCountMatching(ctx context.Context, term string) (int, error) {
	var n int
	return n, c.Call(ctx, "store.ClosedProjectCount", &n, term)
}

// ClosedSessions searches the parked sessions by name, project, path, branch
// and conversation; an empty term is the plain history, newest close first.
// A non-empty term also starts indexing any conversation not indexed yet.
func (c *Client) ClosedSessions(ctx context.Context, term string) (ClosedHistory, error) {
	var h ClosedHistory
	return h, c.Call(ctx, "store.ClosedSessions", &h, term)
}

// ExternalSessions lists the conversations started outside lich in a known
// project's checkouts that no session row holds, newest first.
func (c *Client) ExternalSessions(ctx context.Context) ([]ExternalSession, error) {
	var rows []ExternalSession
	return rows, c.Call(ctx, "store.ExternalSessions", &rows)
}

// AdoptExternalSession files an external conversation as a parked session of
// projectID under sessionID, which ReopenSession then brings back. It fails
// when a session already holds the conversation.
func (c *Client) AdoptExternalSession(ctx context.Context, projectID, sessionID, kind, path, providerSessionID, label string) error {
	return c.Call(ctx, "store.AdoptExternalSession", nil, projectID, sessionID, kind, path, providerSessionID, label)
}

// ForgetSession deletes one parked session for good. It fails on a session
// that is open.
func (c *Client) ForgetSession(ctx context.Context, sessionID string) error {
	return c.Call(ctx, "store.ForgetSession", nil, sessionID)
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
