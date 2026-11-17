package api

var (
	ProjectAwaitThreadFix  = define[ThreadFixOptions, None]("project.AwaitThreadFix")
	ProjectMarkThreadFixed = define[MarkThreadFixedOptions, None]("project.MarkThreadFixed")
	ProjectThreadFixes     = define[None, []ThreadFix]("project.ThreadFixes")

	// ThreadFixesEvent carries every review thread handed to a session, emitted
	// once one is sent, marked fixed, or stops waiting: resolved by the push of
	// its fix, or dropped with its pull request.
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

// MarkThreadFixedOptions is the agent saying Commit fixes ThreadID, a thread
// handed to it from the Pulls screen.
type MarkThreadFixedOptions struct {
	ThreadID string `json:"threadId"`
	Commit   string `json:"commit"`
}

// ThreadFix is a review thread handed to a session. Commit is empty until the
// agent marks the commit that fixes it; the thread is resolved once the pull
// request's head contains that commit.
type ThreadFix struct {
	ThreadID  string `json:"threadId"`
	Path      string `json:"path"`
	Number    int    `json:"number"`
	CommentID int64  `json:"commentId"`
	Commit    string `json:"commit,omitempty"`
}
