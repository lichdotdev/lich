package api

var (
	ProjectLinkPullRequest     = define[LinkPullRequestOptions, None]("project.LinkPullRequest")
	ProjectUnlinkPullRequest   = define[UnlinkPullRequestOptions, None]("project.UnlinkPullRequest")
	ProjectSessionPullRequests = define[None, []SessionPullRequest]("project.SessionPullRequests")

	// SessionPullRequestsEvent carries every pull request linked to or
	// unlinked from a session by hand, emitted once a link or an unlink is
	// stored and whenever a read of a linked one changes what it holds.
	SessionPullRequestsEvent = defineEvent[[]SessionPullRequest]("session-pull-requests")
)

// LinkPullRequestOptions links Ref to Session: #N names a pull request of the
// repository checked out at Path, else Ref is a GitHub pull request or GitLab
// merge request URL.
type LinkPullRequestOptions struct {
	Session string `json:"session"`
	Path    string `json:"path"`
	Ref     string `json:"ref"`
}

// UnlinkPullRequestOptions hides the pull request at URL from Session, a
// hand-linked one or the one its branch has.
type UnlinkPullRequestOptions struct {
	Session string `json:"session"`
	URL     string `json:"url"`
}

// SessionPullRequest is the person's latest word on one pull request of a
// session, keyed by the URL the forge answers for it. Linked false is an
// unlink, which hides the branch's own pull request too. Here is a link made
// by number, in the session's own repository. PR is the last read, nil for an
// unlink nothing read; Error is why the last read of a linked one failed.
type SessionPullRequest struct {
	Session string       `json:"session"`
	URL     string       `json:"url"`
	Linked  bool         `json:"linked"`
	At      int64        `json:"at"`
	Forge   string       `json:"forge"`
	Here    bool         `json:"here"`
	PR      *PullRequest `json:"pr"`
	Error   string       `json:"error,omitempty"`
}
