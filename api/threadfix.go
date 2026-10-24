package api

var (
	ProjectAwaitThreadFix = define[ThreadFixOptions, None]("project.AwaitThreadFix")
	ProjectThreadFixes    = define[None, []ThreadFix]("project.ThreadFixes")

	// ThreadFixesEvent carries every review thread waiting for its fix, emitted
	// once one starts waiting or stops: resolved by a push, or dropped with
	// its pull request.
	ThreadFixesEvent = defineEvent[[]ThreadFix]("thread-fixes")
)

// ThreadFixOptions names a review thread just handed to a session: ThreadID is
// its GraphQL node id (what resolving takes), CommentID the database id of a
// comment in it (what a reply takes).
type ThreadFixOptions struct {
	Path      string `json:"path"`
	Number    int    `json:"number"`
	ThreadID  string `json:"threadId"`
	CommentID int64  `json:"commentId"`
}

// ThreadFix is a review thread waiting for the pull request's next push, which
// resolves it. Head is the commit the pull request stood on when the thread
// was sent.
type ThreadFix struct {
	ThreadID  string `json:"threadId"`
	Path      string `json:"path"`
	Number    int    `json:"number"`
	CommentID int64  `json:"commentId"`
	Head      string `json:"head"`
}
